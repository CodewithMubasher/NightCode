import os
os.environ["LITELLM_LOG"] = "WARNING"

import logging
logging.getLogger("LiteLLM").setLevel(logging.WARNING)
logging.getLogger("litellm").setLevel(logging.WARNING)

import litellm
from litellm import acompletion
from typing import AsyncIterator, Optional
from dataclasses import dataclass
import traceback

litellm.success_callback = []
litellm.set_verbose = False
litellm.suppress_debug_info = True
litellm.telemetry = False

@dataclass
class LLMRequest:
    messages: list[dict]
    model_id: str
    provider_name: str
    system_prompt: Optional[str] = None
    temperature: float = 0.7
    max_tokens: int = 4096
    stream: bool = True

@dataclass
class LLMChunk:
    text: str = ""
    reasoning: str = ""
    is_done: bool = False
    input_tokens: int = 0
    output_tokens: int = 0

@dataclass
class LLMResponse:
    text: str = ""
    reasoning: str = ""
    input_tokens: int = 0
    output_tokens: int = 0

def _build_litellm_model_string(provider_name: str, model_id: str) -> str:
    if provider_name == "opencode":
        return f"openai/{model_id.split('/', 1)[-1]}"
    if model_id.startswith(f"{provider_name}/"):
        return model_id
    if provider_name in ("openai", "anthropic"):
        return model_id
    return f"{provider_name}/{model_id}"

def _get_api_key(provider_name: str) -> Optional[str]:
    key_map = {
        "openrouter": "OPENROUTER_API_KEY",
        "groq": "GROQ_API_KEY",
        "openai": "OPENAI_API_KEY",
        "anthropic": "ANTHROPIC_API_KEY",
        "gemini": "GEMINI_API_KEY",
        "opencode": "OPENCODE_API_KEY",
    }
    env_var = key_map.get(provider_name)
    return os.getenv(env_var) if env_var else None

def _get_base_url(provider_name: str) -> Optional[str]:
    if provider_name == "ollama":
        return os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
    if provider_name == "opencode":
        return os.getenv("OPENCODE_BASE_URL", "https://opencode.ai/zen/v1")
    return None

async def stream_llm(request: LLMRequest) -> AsyncIterator[LLMChunk]:
    messages = []

    if request.system_prompt:
        messages.append({"role": "system", "content": request.system_prompt})

    messages.extend(request.messages)

    model_string = _build_litellm_model_string(request.provider_name, request.model_id)
    api_key = _get_api_key(request.provider_name)
    base_url = _get_base_url(request.provider_name)

    kwargs = {
        "model": model_string,
        "messages": messages,
        "temperature": request.temperature,
        "max_tokens": request.max_tokens,
        "stream": True,
    }

    if api_key:
        kwargs["api_key"] = api_key
    if base_url:
        kwargs["api_base"] = base_url

    if request.provider_name == "openrouter":
        kwargs["extra_headers"] = {
            "HTTP-Referer": "https://nightcode.dev",
            "X-Title": "NightCode",
        }

    try:
        response = await acompletion(**kwargs)

        async for chunk in response:
            if not chunk.choices:
                continue

            delta = chunk.choices[0].delta
            text = delta.content or ""

            reasoning = ""
            if hasattr(delta, "reasoning_content") and delta.reasoning_content:
                reasoning = delta.reasoning_content
            elif hasattr(delta, "thinking") and delta.thinking:
                reasoning = delta.thinking

            input_tokens = 0
            output_tokens = 0
            if hasattr(chunk, "usage") and chunk.usage:
                input_tokens = chunk.usage.prompt_tokens or 0
                output_tokens = chunk.usage.completion_tokens or 0

            if text or reasoning:
                yield LLMChunk(
                    text=text,
                    reasoning=reasoning,
                    input_tokens=input_tokens,
                    output_tokens=output_tokens,
                )

        yield LLMChunk(is_done=True)

    except Exception as e:
        error_msg = f"[{request.provider_name}/{request.model_id}] {type(e).__name__}: {e}"
        try:
            raise type(e)(error_msg) from e
        except Exception:
            raise RuntimeError(error_msg) from e


async def call_llm(request: LLMRequest) -> LLMResponse:
    messages = []

    if request.system_prompt:
        messages.append({"role": "system", "content": request.system_prompt})

    messages.extend(request.messages)

    model_string = _build_litellm_model_string(request.provider_name, request.model_id)
    api_key = _get_api_key(request.provider_name)
    base_url = _get_base_url(request.provider_name)

    kwargs = {
        "model": model_string,
        "messages": messages,
        "temperature": request.temperature,
        "max_tokens": request.max_tokens,
        "stream": False,
    }

    if api_key:
        kwargs["api_key"] = api_key
    if base_url:
        kwargs["api_base"] = base_url

    if request.provider_name == "openrouter":
        kwargs["extra_headers"] = {
            "HTTP-Referer": "https://nightcode.dev",
            "X-Title": "NightCode",
        }

    try:
        response = await acompletion(**kwargs)

        choice = response.choices[0] if response.choices else None
        text = choice.message.content or "" if choice else ""
        reasoning = ""

        if choice and hasattr(choice.message, "reasoning_content") and choice.message.reasoning_content:
            reasoning = choice.message.reasoning_content
        elif choice and hasattr(choice.message, "thinking") and choice.message.thinking:
            reasoning = choice.message.thinking

        input_tokens = 0
        output_tokens = 0
        if hasattr(response, "usage") and response.usage:
            input_tokens = response.usage.prompt_tokens or 0
            output_tokens = response.usage.completion_tokens or 0

        return LLMResponse(
            text=text,
            reasoning=reasoning,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )

    except Exception as e:
        error_msg = f"[{request.provider_name}/{request.model_id}] {type(e).__name__}: {e}"
        try:
            raise type(e)(error_msg) from e
        except Exception:
            raise RuntimeError(error_msg) from e
