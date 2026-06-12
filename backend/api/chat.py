from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from pydantic import BaseModel
from sqlmodel import Session, select
from database.connection import get_session
from database.models import Conversation, Message, UsageLog
from core.skills_injector import build_system_prompt
from memory.conversation_memory import load_memory
from graphs.chat_graph import run_chat_graph
from graphs.plan_graph import run_plan_graph
from graphs.build_graph import run_build_graph
from core.stream_normalizer import sse_error, sse_done
import json
import uuid
from datetime import datetime

router = APIRouter()

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


async def _stream_response(request: ChatRequest, session: Session):
    provider_name, model_id = _parse_provider_and_model(request.model, request.provider)

    history = load_memory(session, request.chat_id, max_messages=20)

    current_messages = [{"role": m.role, "content": m.content} for m in request.messages]

    all_messages = history if history else current_messages

    system_prompt = build_system_prompt(
        session=session,
        mode=request.mode,
        active_skill_ids=request.active_skill_ids,
    )

    user_query = next(
        (m["content"] for m in reversed(all_messages) if m["role"] == "user"),
        ""
    )

    _assistant_content = ""

    async def _tracking_stream(source):
        nonlocal _assistant_content
        async for event in source:
            if event.startswith("data: ") and event != "data: [DONE]\n\n":
                try:
                    data = json.loads(event[6:].strip())
                    if "chunk" in data:
                        _assistant_content += data["chunk"]
                    elif "final_answer" in data:
                        _assistant_content += data["final_answer"]
                except Exception:
                    pass
            yield event

    if request.mode == "plan":
        stream = run_plan_graph(
            messages=all_messages,
            system_prompt=system_prompt,
            model_id=model_id,
            provider_name=provider_name,
            user_query=user_query,
        )
    elif request.mode == "build":
        stream = run_build_graph(
            messages=all_messages,
            system_prompt=system_prompt,
            model_id=model_id,
            provider_name=provider_name,
            user_query=user_query,
        )
    else:
        stream = run_chat_graph(
            messages=all_messages,
            system_prompt=system_prompt,
            model_id=model_id,
            provider_name=provider_name,
        )

    async for chunk in _tracking_stream(stream):
        yield chunk

    try:
        last_user = next((m for m in reversed(request.messages) if m.role == "user"), None)

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
                    created_at=datetime.utcnow(),
                ))

        if _assistant_content:
            session.add(Message(
                id=str(uuid.uuid4()),
                conversation_id=request.chat_id,
                role="assistant",
                content=_assistant_content,
                mode=request.mode,
                model_id=model_id,
                created_at=datetime.utcnow(),
            ))

        conv = session.exec(
            select(Conversation).where(Conversation.id == request.chat_id)
        ).first()
        if not conv:
            session.add(Conversation(
                id=request.chat_id,
                title=last_user.content[:40] if last_user else "New Chat",
                mode=request.mode,
                model_id=model_id,
                provider_name=provider_name,
                created_at=datetime.utcnow(),
                updated_at=datetime.utcnow(),
            ))
        else:
            conv.updated_at = datetime.utcnow()
            session.add(conv)

        session.commit()
    except Exception as e:
        print(f"[DB] Failed to save messages: {e}")


@router.post("/api/chat/stream")
async def chat_stream(request: ChatRequest, session: Session = Depends(get_session)):
    return StreamingResponse(
        _stream_response(request, session),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
