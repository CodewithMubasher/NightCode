from typing import AsyncIterator
from core.llm_router import stream_llm, LLMRequest
from core.stream_normalizer import sse_chunk, sse_thinking, sse_reasoning, sse_synthesis, sse_done, sse_error

REASONING_BATCH = 200


async def _call_internal_llm(
    messages: list,
    model_id: str,
    provider_name: str,
    system: str = "",
    max_tokens: int = 512,
) -> str:
    request = LLMRequest(
        messages=messages,
        model_id=model_id,
        provider_name=provider_name,
        system_prompt=system,
        max_tokens=max_tokens,
        stream=True,
    )
    result = ""
    async for chunk in stream_llm(request):
        if not chunk.is_done:
            result += chunk.text or ""
    return result.strip()


async def _stream_with_batched_reasoning(
    request: LLMRequest,
) -> AsyncIterator[str]:
    buffer = ""
    async for chunk in stream_llm(request):
        if chunk.is_done:
            if buffer.strip():
                yield sse_reasoning(buffer.strip())
            break
        if chunk.text:
            buffer += chunk.text
            if len(buffer) >= REASONING_BATCH or buffer.endswith(('.', '!', '?', '\n')):
                yield sse_reasoning(buffer.strip())
                buffer = ""
    if buffer.strip():
        yield sse_reasoning(buffer.strip())


async def run_plan_graph(
    messages: list[dict],
    system_prompt: str,
    model_id: str,
    provider_name: str,
    user_query: str,
) -> AsyncIterator[str]:
    try:
        yield sse_thinking("Analyzing request scope...")
        analysis = await _call_internal_llm(
            messages=[{
                "role": "user",
                "content": (
                    f"Analyze this request and identify: "
                    f"1) The core goal 2) Key constraints 3) Success criteria\n\n"
                    f"Request: {user_query}"
                )
            }],
            model_id=model_id,
            provider_name=provider_name,
            system="You are an analytical assistant. Be concise and structured.",
            max_tokens=512,
        )
        yield sse_reasoning(f"Analysis: {analysis[:200]}...")

        research = await _call_internal_llm(
            messages=[{
                "role": "user",
                "content": (
                    f"Based on this analysis: {analysis}\n\n"
                    f"What are the most important considerations, best practices, "
                    f"and potential pitfalls for: {user_query}?"
                )
            }],
            model_id=model_id,
            provider_name=provider_name,
            system="You are a knowledgeable research assistant. Focus on actionable insights.",
            max_tokens=512,
        )
        yield sse_reasoning(f"Research: {research[:200]}...")

        structure = await _call_internal_llm(
            messages=[{
                "role": "user",
                "content": (
                    f"Analysis: {analysis}\n"
                    f"Research: {research}\n\n"
                    f"Create a structured outline for a plan to: {user_query}"
                )
            }],
            model_id=model_id,
            provider_name=provider_name,
            system="You are a planning expert. Create clear, actionable outlines.",
            max_tokens=512,
        )
        yield sse_reasoning(f"Structure: {structure[:200]}...")

        yield sse_synthesis("Plan structured. Writing final response...")
        yield sse_reasoning("Writing the plan...")

        async for chunk in stream_llm(
            LLMRequest(
                messages=[{
                    "role": "user",
                    "content": (
                        f"Using this analysis and structure, write a comprehensive plan:\n\n"
                        f"Original request: {user_query}\n"
                        f"Analysis: {analysis}\n"
                        f"Research insights: {research}\n"
                        f"Structure: {structure}\n\n"
                        f"Write the final actionable plan now."
                    )
                }],
                model_id=model_id,
                provider_name=provider_name,
                system_prompt=system_prompt,
                stream=True,
            )
        ):
            if chunk.is_done:
                break
            if chunk.text:
                yield sse_chunk(chunk.text)

        yield sse_done()
    except Exception as e:
        yield sse_error(str(e))
        yield sse_done()
