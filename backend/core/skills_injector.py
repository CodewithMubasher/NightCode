from sqlmodel import Session, select
from database.models import Skill

def build_system_prompt(
    session: Session,
    mode: str,
    active_skill_ids: list[str] | None = None,
    base_prompt: str | None = None,
) -> str:
    parts = []

    if base_prompt:
        parts.append(base_prompt)
    else:
        mode_prompts = {
            "chat": "You are NightCode, a personal AI OS assistant. Be direct, precise, and helpful.",
            "plan": (
                "You are NightCode in Plan mode. "
                "Think step by step before responding. "
                "Structure your response as a clear, actionable plan. "
                "Use headers and numbered steps."
            ),
            "build": (
                "You are NightCode in Build mode. "
                "You have access to tools. Use them to accomplish the task. "
                "Explain what you're doing at each step."
            ),
        }
        parts.append(mode_prompts.get(mode, mode_prompts["chat"]))

    always_skills = session.exec(
        select(Skill).where(
            Skill.trigger == "always",
            Skill.is_enabled == True
        )
    ).all()

    for skill in always_skills:
        parts.append(f"\n--- {skill.name} ---\n{skill.system_prompt}")

    if active_skill_ids:
        manual_skills = session.exec(
            select(Skill).where(
                Skill.id.in_(active_skill_ids),
                Skill.is_enabled == True
            )
        ).all()
        for skill in manual_skills:
            parts.append(f"\n--- {skill.name} ---\n{skill.system_prompt}")

    return "\n\n".join(parts)
