from typing import AsyncIterator
from core.llm_router import stream_llm, LLMRequest
from core.stream_normalizer import sse_chunk, sse_thinking, sse_reasoning, sse_done, sse_error


async def run_chat_graph(
    messages: list[dict],
    system_prompt: str,
    model_id: str,
    provider_name: str,
) -> AsyncIterator[str]:
    yield sse_thinking("Thinking...")
    request = LLMRequest(
        messages=messages,
        model_id=model_id,
        provider_name=provider_name,
        system_prompt=system_prompt,
        stream=True,
    )
    try:
        reasoning_buffer = ""
        async for chunk in stream_llm(request):
            if chunk.is_done:
                if reasoning_buffer.strip():
                    yield sse_reasoning(reasoning_buffer.strip())
                yield sse_done()
                return
            if chunk.text:
                yield sse_chunk(chunk.text)
            if chunk.reasoning:
                reasoning_buffer += chunk.reasoning
                if len(reasoning_buffer) >= 200 or reasoning_buffer.endswith(('.', '!', '?', '\n')):
                    yield sse_reasoning(reasoning_buffer.strip())
                    reasoning_buffer = ""
    except Exception as e:
        yield sse_error(str(e))
        yield sse_done()
