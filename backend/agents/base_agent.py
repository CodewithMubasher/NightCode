from typing import AsyncIterator, Optional
from core.llm_router import call_llm, stream_llm, LLMRequest
from core.stream_normalizer import sse_chunk, sse_thinking, sse_reasoning, sse_tool_call, sse_tool_result, sse_final, sse_done, sse_error
from tools.registry import ToolRegistry
import json
import time

MAX_REACT_STEPS = 12
MAX_WALL_SECONDS = 120

PLAN_PROMPT = """You are in plan mode. Analyze the user's request and create a structured, actionable plan.

Use this format:
1. **Goal**: Restate the core objective
2. **Steps**: Numbered list of steps to accomplish the goal
3. **Considerations**: Any risks, dependencies, or prerequisites

Be thorough and practical."""


class Agent:
    def __init__(
        self,
        mode: str = "chat",
        system_prompt: str = "",
        tools_enabled: bool = False,
        tool_registry: Optional[ToolRegistry] = None,
    ):
        self.mode = mode
        self.system_prompt = system_prompt
        self.tools_enabled = tools_enabled
        self.tool_registry = tool_registry

    async def run(
        self,
        messages: list[dict],
        model_id: str,
        provider_name: str,
    ) -> AsyncIterator[str]:
        try:
            if self.mode == "chat":
                async for event in self._stream(messages, model_id, provider_name):
                    yield event
            else:
                async for event in self._plan_and_stream(messages, model_id, provider_name):
                    yield event
        except Exception as e:
            yield sse_error(str(e))
            yield sse_done()

    async def _stream(
        self,
        messages: list[dict],
        model_id: str,
        provider_name: str,
    ) -> AsyncIterator[str]:
        yield sse_thinking("processing your request")
        full_content = ""
        total_input = 0
        total_output = 0

        async for chunk in stream_llm(LLMRequest(
            messages=messages,
            model_id=model_id,
            provider_name=provider_name,
            system_prompt=self.system_prompt,
            stream=True,
        )):
            if chunk.is_done:
                break
            if chunk.reasoning:
                yield sse_reasoning(chunk.reasoning)
            if chunk.text:
                full_content += chunk.text
                yield sse_chunk(chunk.text)
            total_input = chunk.input_tokens or total_input
            total_output = chunk.output_tokens or total_output

        yield sse_final(full_content)
        yield sse_done()

    async def _plan_and_stream(
        self,
        messages: list[dict],
        model_id: str,
        provider_name: str,
    ) -> AsyncIterator[str]:
        start_time = time.monotonic()

        user_query = next(
            (m["content"] for m in reversed(messages) if m["role"] == "user"),
            ""
        )

        if self.mode == "plan":
            yield sse_thinking("analyzing request")
            plan_messages = [{"role": "user", "content": f"{PLAN_PROMPT}\n\nRequest: {user_query}"}]
            response = await call_llm(LLMRequest(
                messages=plan_messages,
                model_id=model_id,
                provider_name=provider_name,
                system_prompt=self.system_prompt,
                max_tokens=2048,
                stream=False,
            ))

            if response.reasoning:
                yield sse_reasoning(response.reasoning)

            yield sse_chunk(response.text)

            async for chunk in stream_llm(LLMRequest(
                messages=messages + [{"role": "assistant", "content": response.text}],
                model_id=model_id,
                provider_name=provider_name,
                system_prompt=self.system_prompt + " Write the detailed plan.",
                stream=True,
            )):
                if chunk.is_done:
                    break
                if chunk.reasoning:
                    yield sse_reasoning(chunk.reasoning)
                if chunk.text:
                    yield sse_chunk(chunk.text)

            yield sse_done()
            return

        if self.mode == "build":
            yield sse_thinking("analyzing task scope")
            tools_schema = self.tool_registry.get_llm_tools() if self.tool_registry and self.tools_enabled else None

            for step in range(MAX_REACT_STEPS):
                if time.monotonic() - start_time > MAX_WALL_SECONDS:
                    yield sse_error("Task timed out")
                    yield sse_done()
                    return

                response = await call_llm(LLMRequest(
                    messages=messages,
                    model_id=model_id,
                    provider_name=provider_name,
                    system_prompt=self.system_prompt,
                    max_tokens=4096,
                    stream=False,
                    tools=tools_schema,
                    tool_choice="auto" if tools_schema else None,
                ))

                if response.reasoning:
                    yield sse_reasoning(response.reasoning)

                if response.tool_calls and tools_schema:
                    assistant_msg = {
                        "role": "assistant",
                        "content": response.text or "",
                        "tool_calls": response.tool_calls,
                    }
                    messages.append(assistant_msg)

                    for tc in response.tool_calls:
                        tc_id = tc["id"]
                        tc_name = tc["function"]["name"]
                        try:
                            tc_args = json.loads(tc["function"]["arguments"])
                        except json.JSONDecodeError:
                            tc_args = {}

                        yield sse_tool_call(tc_id, tc_name, tc_args)

                        result = await self.tool_registry.execute(tc_id, tc_name, tc_args)

                        yield sse_tool_result(tc_id, tc_name, result.output, result.status)

                        messages.append({
                            "role": "tool",
                            "tool_call_id": tc_id,
                            "content": json.dumps({"output": result.output, "status": result.status}),
                        })
                    continue

                if response.text:
                    yield sse_chunk(response.text)
                    yield sse_final(response.text)
                    yield sse_done()
                    return

            yield sse_error("Max iterations reached")
            yield sse_done()
