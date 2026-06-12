from fastapi import APIRouter, Depends
from sqlmodel import Session, select
from database.connection import get_session
from database.models import Skill, AgentDefinition
from pydantic import BaseModel
from typing import Optional
import uuid

router = APIRouter()

class SkillCreate(BaseModel):
    name: str
    description: str = ""
    system_prompt: str
    trigger: str = "manual"
    is_enabled: bool = True

class SkillUpdate(BaseModel):
    name: Optional[str] = None
    description: Optional[str] = None
    system_prompt: Optional[str] = None
    trigger: Optional[str] = None
    is_enabled: Optional[bool] = None

class AgentCreate(BaseModel):
    name: str
    description: str = ""
    graph_json: str = "{}"

@router.get("/api/skills")
def list_skills(session: Session = Depends(get_session)):
    skills = session.exec(select(Skill)).all()
    return [
        {
            "id": s.id,
            "name": s.name,
            "description": s.description,
            "system_prompt": s.system_prompt,
            "trigger": s.trigger,
            "is_enabled": s.is_enabled,
            "is_builtin": s.is_builtin,
        }
        for s in skills
    ]

@router.post("/api/skills")
def create_skill(body: SkillCreate, session: Session = Depends(get_session)):
    skill = Skill(
        id=str(uuid.uuid4()),
        name=body.name,
        description=body.description,
        system_prompt=body.system_prompt,
        trigger=body.trigger,
        is_enabled=body.is_enabled,
    )
    session.add(skill)
    session.commit()
    return {"id": skill.id, "ok": True}

@router.patch("/api/skills/{skill_id}")
def update_skill(skill_id: str, body: SkillUpdate, session: Session = Depends(get_session)):
    skill = session.get(Skill, skill_id)
    if not skill:
        return {"error": "Skill not found"}
    if body.name is not None:
        skill.name = body.name
    if body.description is not None:
        skill.description = body.description
    if body.system_prompt is not None:
        skill.system_prompt = body.system_prompt
    if body.trigger is not None:
        skill.trigger = body.trigger
    if body.is_enabled is not None:
        skill.is_enabled = body.is_enabled
    session.add(skill)
    session.commit()
    return {"ok": True}

@router.delete("/api/skills/{skill_id}")
def delete_skill(skill_id: str, session: Session = Depends(get_session)):
    skill = session.get(Skill, skill_id)
    if not skill:
        return {"error": "Skill not found"}
    session.delete(skill)
    session.commit()
    return {"ok": True}

@router.get("/api/agents")
def list_agents(session: Session = Depends(get_session)):
    agents = session.exec(select(AgentDefinition)).all()
    return [
        {"id": a.id, "name": a.name, "description": a.description}
        for a in agents
    ]

@router.post("/api/agents")
def create_agent(body: AgentCreate, session: Session = Depends(get_session)):
    agent = AgentDefinition(
        id=str(uuid.uuid4()),
        name=body.name,
        description=body.description,
        graph_json=body.graph_json,
    )
    session.add(agent)
    session.commit()
    return {"id": agent.id, "ok": True}
