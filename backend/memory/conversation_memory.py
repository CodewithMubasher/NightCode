from sqlmodel import Session, select
from database.models import Message

def load_memory(
    session: Session,
    conversation_id: str,
    max_messages: int = 20
) -> list[dict]:
    messages = session.exec(
        select(Message)
        .where(Message.conversation_id == conversation_id)
        .order_by(Message.created_at.desc())
        .limit(max_messages)
    ).all()

    messages = list(reversed(messages))

    return [
        {"role": m.role, "content": m.content}
        for m in messages
        if m.content.strip()
    ]
