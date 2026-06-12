from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from pydantic import BaseModel
from sqlmodel import Session, select
from database.connection import get_session
from database.models import Conversation, Message, UsageLog
from core.skills_injector import build_system_prompt
from memory.conversation_memory import load_memory
from agents.base_agent import Agent
from tools.registry import ToolRegistry
from core.stream_normalizer import sse_error, sse_done
import json
import uuid
import time
from datetime import datetime

router = APIRouter()

STORE_INTERVAL_CHARS = 250
STORE_INTERVAL_SECS = 2.0


class ChatMessage(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    chat_id: str
    messages: list[ChatMessage]
    mode: str = "chat"
    model: str
    provider: str = "openrouter"
    active_skill_ids: list[str] = []


def _parse_provider_and_model(model_str: str, provider: str) -> tuple[str, str]:
    known_prefixes = ["openrouter", "groq", "ollama", "openai", "anthropic", "gemini", "opencode"]
    for prefix in known_prefixes:
        if model_str.startswith(f"{prefix}/"):
            return prefix, model_str[len(prefix)+1:]
    return provider, model_str


async def _stream_response(request: ChatRequest, session: Session, registry: ToolRegistry):
    provider_name, model_id = _parse_provider_and_model(request.model, request.provider)
    history = load_memory(session, request.chat_id, max_messages=20)
    current_messages = [{"role": m.role, "content": m.content} for m in request.messages]
    all_messages = history if history else current_messages

    system_prompt = build_system_prompt(
        session=session,
        mode=request.mode,
        active_skill_ids=request.active_skill_ids,
    )

    agent = Agent(
        mode=request.mode,
        system_prompt=system_prompt,
        tools_enabled=(request.mode == "build"),
        tool_registry=registry,
    )

    last_user = next((m for m in reversed(request.messages) if m.role == "user"), None)

    conv = session.exec(
        select(Conversation).where(Conversation.id == request.chat_id)
    ).first()
    if not conv:
        conv = Conversation(
            id=request.chat_id,
            title=last_user.content[:40] if last_user else "New Chat",
            mode=request.mode,
            model_id=model_id,
            provider_name=provider_name,
            created_at=datetime.utcnow(),
            updated_at=datetime.utcnow(),
        )
        session.add(conv)
    else:
        conv.updated_at = datetime.utcnow()
        session.add(conv)
    session.commit()

    if last_user:
        existing_user = session.exec(
            select(Message).where(
                Message.conversation_id == request.chat_id,
                Message.content == last_user.content,
                Message.role == "user"
            )
        ).first()
        if not existing_user:
            session.add(Message(
                id=str(uuid.uuid4()),
                conversation_id=request.chat_id,
                role="user",
                content=last_user.content,
                mode=request.mode,
                model_id=model_id,
                provider_name=provider_name,
                created_at=datetime.utcnow(),
            ))
            session.commit()

    msg_id = str(uuid.uuid4())
    msg = Message(
        id=msg_id,
        conversation_id=request.chat_id,
        role="assistant",
        content="",
        status="streaming",
        mode=request.mode,
        model_id=model_id,
        provider_name=provider_name,
        created_at=datetime.utcnow(),
    )
    session.add(msg)
    session.commit()

    accumulated_content = ""
    last_store_time = time.monotonic()
    last_store_len = 0

    stream_start = time.monotonic()
    total_input = 0
    total_output = 0

    async for sse_event in agent.run(
        messages=all_messages,
        model_id=model_id,
        provider_name=provider_name,
    ):
        should_store = False

        if sse_event.startswith("data: ") and sse_event != "data: [DONE]\n\n":
            try:
                data = json.loads(sse_event[6:].strip())
                event_type = data.get("type", "")
                content_piece = data.get("content", "")

                if event_type == "chunk":
                    accumulated_content += content_piece
                    acc_len = len(accumulated_content)
                    if acc_len - last_store_len >= STORE_INTERVAL_CHARS or (time.monotonic() - last_store_time) >= STORE_INTERVAL_SECS:
                        should_store = True

                elif event_type == "final":
                    accumulated_content = content_piece
                    should_store = True

                elif event_type == "tool_call":
                    msg.append_tool_call({
                        "id": data.get("id", ""),
                        "name": data.get("name", ""),
                        "args": data.get("args", {}),
                    })
                    should_store = True

                elif event_type == "tool_result":
                    msg.append_tool_result({
                        "id": data.get("id", ""),
                        "name": data.get("name", ""),
                        "output": data.get("output", ""),
                        "status": data.get("status", ""),
                    })
                    should_store = True
            except Exception:
                pass

        yield sse_event

        if should_store:
            msg.content = accumulated_content
            msg.heartbeat_at = datetime.utcnow().isoformat()
            session.add(msg)
            session.commit()
            last_store_time = time.monotonic()
            last_store_len = len(accumulated_content)

    msg.content = accumulated_content
    msg.status = "complete"
    msg.heartbeat_at = datetime.utcnow().isoformat()
    session.add(msg)
    session.commit()

    if accumulated_content:
        latency_ms = int((time.monotonic() - stream_start) * 1000)
        usage = UsageLog(
            conversation_id=request.chat_id,
            message_id=msg_id,
            model_id=model_id,
            provider_name=provider_name,
            input_tokens=0,
            output_tokens=0,
            total_tokens=0,
            latency_ms=latency_ms,
            mode=request.mode,
        )
        session.add(usage)
        session.commit()


@router.post("/api/chat/stream")
async def chat_stream(
    request: ChatRequest,
    session: Session = Depends(get_session),
):
    from tools import registry as tool_registry
    return StreamingResponse(
        _stream_response(request, session, tool_registry),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
