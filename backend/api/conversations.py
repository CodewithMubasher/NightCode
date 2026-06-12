from fastapi import APIRouter, Depends
from sqlmodel import Session, select
from database.connection import get_session
from database.models import Conversation, Message
from pydantic import BaseModel
from typing import Optional
from datetime import datetime
import uuid

router = APIRouter()

class ConversationCreate(BaseModel):
    id: str
    title: str = "New Chat"
    mode: str = "chat"
    model_id: str = ""
    provider_name: str = ""

class ConversationUpdate(BaseModel):
    title: Optional[str] = None
    mode: Optional[str] = None
    model_id: Optional[str] = None
    provider_name: Optional[str] = None

@router.get("/api/conversations")
def list_conversations(session: Session = Depends(get_session)):
    conversations = session.exec(
        select(Conversation).order_by(Conversation.updated_at.desc())
    ).all()
    return [
        {
            "id": c.id,
            "title": c.title,
            "mode": c.mode,
            "model_id": c.model_id,
            "provider_name": c.provider_name,
            "created_at": c.created_at.isoformat(),
            "updated_at": c.updated_at.isoformat(),
            "message_count": len(c.messages),
        }
        for c in conversations
    ]

@router.post("/api/conversations")
def create_conversation(body: ConversationCreate, session: Session = Depends(get_session)):
    conv = Conversation(
        id=body.id,
        title=body.title,
        mode=body.mode,
        model_id=body.model_id,
        provider_name=body.provider_name,
    )
    session.add(conv)
    session.commit()
    return {"ok": True}

@router.get("/api/conversations/{conv_id}")
def get_conversation(conv_id: str, session: Session = Depends(get_session)):
    conv = session.get(Conversation, conv_id)
    if not conv:
        return {"error": "Conversation not found"}
    return {
        "id": conv.id,
        "title": conv.title,
        "mode": conv.mode,
        "model_id": conv.model_id,
        "provider_name": conv.provider_name,
        "created_at": conv.created_at.isoformat(),
        "updated_at": conv.updated_at.isoformat(),
        "messages": [
            {
                "id": m.id,
                "role": m.role,
                "content": m.content,
                "reasoning": m.reasoning,
                "mode": m.mode,
                "model_id": m.model_id,
                "created_at": m.created_at.isoformat(),
            }
            for m in conv.messages
        ],
    }

@router.patch("/api/conversations/{conv_id}")
def update_conversation(conv_id: str, body: ConversationUpdate, session: Session = Depends(get_session)):
    conv = session.get(Conversation, conv_id)
    if not conv:
        return {"error": "Conversation not found"}
    if body.title is not None:
        conv.title = body.title
    if body.mode is not None:
        conv.mode = body.mode
    if body.model_id is not None:
        conv.model_id = body.model_id
    if body.provider_name is not None:
        conv.provider_name = body.provider_name
    conv.updated_at = datetime.utcnow()
    session.add(conv)
    session.commit()
    return {"ok": True}

@router.delete("/api/conversations/{conv_id}")
def delete_conversation(conv_id: str, session: Session = Depends(get_session)):
    conv = session.get(Conversation, conv_id)
    if not conv:
        return {"error": "Conversation not found"}
    session.delete(conv)
    session.commit()
    return {"ok": True}
