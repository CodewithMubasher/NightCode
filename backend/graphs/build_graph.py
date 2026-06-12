from typing import AsyncIterator
from core.llm_router import stream_llm, call_llm, LLMRequest, LLMResponse
from core.stream_normalizer import sse_chunk, sse_thinking, sse_reasoning, sse_tool_call, sse_tool_result, sse_done, sse_error
from tools.web_search import web_search
from tools.code_executor import execute_code
from tools.file_ops import read_file, write_file, list_directory, grep_files
import json
import re
import asyncio
import time

TOOL_DESCRIPTIONS = {
    "web_search":      "web_search(query: str, num_results: int = 5) — Search the web for current information.",
    "run_code":        "run_code(code: str) — Execute Python code in a sandbox and return the output.",
    "read_file":       "read_file(path: str) — Read a file from the filesystem. Path must be absolute.",
    "write_file":      "write_file(path: str, content: str) — Write content to a file. Creates parent directories.",
    "list_directory":  "list_directory(path: str, recursive: bool = False) — List files and directories.",
    "grep_files":      "grep_files(pattern: str, path: str, file_pattern: str = '*') — Search for a regex pattern in files.",
}

AVAILABLE_TOOLS = {
    "web_search": web_search,
    "run_code": execute_code,
    "read_file": read_file,
    "write_file": write_file,
    "list_directory": list_directory,
    "grep_files": grep_files,
}

TOOL_CALL_RE = re.compile(r'<tool_call>(.*?)</tool_call>', re.DOTALL)
FINAL_ANSWER_RE = re.compile(r'<final_answer>(.*?)</final_answer>', re.DOTALL)
MAX_REACT_ITERATIONS = 6
MAX_WALL_SECONDS = 120
REASONING_BATCH_SIZE = 200


def _tool_descriptions_text() -> str:
    return "\n".join(f"  - {d}" for d in TOOL_DESCRIPTIONS.values())


def _build_react_prompt(
    user_query: str,
    conversation_context: str,
    history_context: str,
) -> str:
    parts = []
    if history_context:
        parts.append(f"Prior conversation:\n{history_context}")
    parts.append(f"Task: {user_query}")
    if conversation_context:
        parts.append(f"What you know so far:\n{conversation_context}")
    parts.append(
        f"Available tools:\n{_tool_descriptions_text()}\n\n"
        f"To use a tool, respond with:\n"
        f"<tool_call>{{\"tool\": \"name\", \"args\": {{...}}}}</tool_call>\n\n"
        f"To give your final answer, respond with:\n"
        f"<final_answer>your complete answer</final_answer>"
    )
    return "\n\n".join(parts)


def _emit_reasoning_batched(reasoning_text: str, batch_size: int = REASONING_BATCH_SIZE):
    words = reasoning_text.split()
    for i in range(0, len(words), batch_size):
        chunk = ' '.join(words[i:i+batch_size])
        if chunk:
            yield sse_reasoning(chunk)


async def _execute_tool(tool_name: str, tool_args: dict) -> tuple[str, str]:
    if tool_name not in AVAILABLE_TOOLS:
        return f"Unknown tool '{tool_name}'. Available: {list(AVAILABLE_TOOLS.keys())}", "failed"
    try:
        tool_fn = AVAILABLE_TOOLS[tool_name]
        result = await asyncio.wait_for(tool_fn(**tool_args), timeout=30.0)
        return str(result)[:500], "success"
    except asyncio.TimeoutError:
        return f"Tool '{tool_name}' timed out after 30 seconds", "failed"
    except Exception as e:
        return f"Tool error: {type(e).__name__}: {e}", "failed"


async def react_loop(
    user_query: str,
    system_prompt: str,
    model_id: str,
    provider_name: str,
    conversation_history: list[dict] | None = None,
) -> AsyncIterator[str]:
    start_time = time.time()
    conversation_context = ""

    history_context = ""
    if conversation_history:
        recent = conversation_history[-6:]
        for msg in recent:
            role_label = "User" if msg["role"] == "user" else "Assistant"
            history_context += f"{role_label}: {msg['content'][:300]}\n"

    iteration = 0
    while iteration < MAX_REACT_ITERATIONS:
        if time.time() - start_time > MAX_WALL_SECONDS:
            break

        iteration += 1

        prompt = _build_react_prompt(user_query, conversation_context, history_context)
        messages = [{"role": "user", "content": prompt}]

        request = LLMRequest(
            messages=messages,
            model_id=model_id,
            provider_name=provider_name,
            system_prompt=system_prompt,
            max_tokens=2048,
            stream=False,
        )

        response: LLMResponse | None = None
        try:
            response = await call_llm(request)
        except Exception as e:
            yield sse_error(f"LLM call failed: {e}")
            break

        text = response.text
        reasoning = response.reasoning

        if reasoning:
            for event in _emit_reasoning_batched(reasoning):
                yield event

        final_match = FINAL_ANSWER_RE.search(text)
        if final_match:
            final_text = final_match.group(1).strip()
            words = final_text.split(' ')
            for i in range(0, len(words), 5):
                chunk = ' '.join(words[i:i+5])
                if chunk:
                    yield sse_chunk(chunk + ' ')
                    await asyncio.sleep(0.01)
            yield sse_done()
            return

        tool_match = TOOL_CALL_RE.search(text)
        if tool_match:
            try:
                tool_data = json.loads(tool_match.group(1).strip())
                tool_name = tool_data.get("tool", "")
                tool_args = tool_data.get("args", {})
            except json.JSONDecodeError:
                conversation_context += "\n[PARSE ERROR] Malformed tool call JSON."
                continue

            tool_id = f"tool_{iteration}"
            yield sse_tool_call(tool_id, tool_name, tool_args)

            result_str, status = await _execute_tool(tool_name, tool_args)
            yield sse_tool_result(tool_id, tool_name, result_str, status)

            conversation_context += (
                f"\n---\nTool: {tool_name} | Status: {status.upper()}\n"
                f"Args: {json.dumps(tool_args)}\n"
                f"Result: {result_str}\n"
            )
            if len(conversation_context) > 3000:
                conversation_context = conversation_context[-3000:]
            continue

        yield sse_chunk(text.strip() + ' ')
        yield sse_done()
        return

    yield sse_reasoning("Generating final answer...")

    final_messages = [
        {
            "role": "user",
            "content": (
                f"Task: {user_query}\n\n"
                f"Information gathered:\n{conversation_context}\n\n"
                f"Write a complete, direct answer to the task. Do not use XML tags."
            )
        }
    ]

    final_request = LLMRequest(
        messages=final_messages,
        model_id=model_id,
        provider_name=provider_name,
        system_prompt=system_prompt,
        max_tokens=2048,
        stream=True,
    )

    try:
        async for llm_chunk in stream_llm(final_request):
            if llm_chunk.is_done:
                break
            if llm_chunk.reasoning:
                for event in _emit_reasoning_batched(llm_chunk.reasoning):
                    yield event
            if llm_chunk.text:
                yield sse_chunk(llm_chunk.text)
    except Exception as e:
        yield sse_error(f"Final answer generation failed: {e}")

    yield sse_done()


async def run_build_graph(
    messages, system_prompt, model_id, provider_name, user_query
) -> AsyncIterator[str]:
    try:
        yield sse_thinking("Thinking...")
        async for event in react_loop(
            user_query=user_query,
            system_prompt=system_prompt,
            model_id=model_id,
            provider_name=provider_name,
            conversation_history=messages,
        ):
            yield event
    except Exception as e:
        yield sse_error(str(e))
        yield sse_done()
