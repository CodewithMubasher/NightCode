from sqlmodel import SQLModel, Field, Relationship
from typing import Optional, List
from datetime import datetime

class Provider(SQLModel, table=True):
    id: Optional[int] = Field(default=None, primary_key=True)
    name: str
    display_name: str
    base_url: Optional[str] = None
    api_key_env: Optional[str] = None
    api_key_value: Optional[str] = None
    is_enabled: bool = True
    is_local: bool = False
    created_at: datetime = Field(default_factory=datetime.utcnow)

class Model(SQLModel, table=True):
    id: Optional[int] = Field(default=None, primary_key=True)
    provider_name: str
    model_id: str
    display_name: str
    context_window: int = 8192
    supports_reasoning: bool = False
    supports_vision: bool = False
    supports_tools: bool = True
    is_enabled: bool = True
    cost_per_1k_input: float = 0.0
    cost_per_1k_output: float = 0.0
    created_at: datetime = Field(default_factory=datetime.utcnow)

class Conversation(SQLModel, table=True):
    id: str = Field(primary_key=True)
    title: str = "New Chat"
    mode: str = "chat"
    model_id: str = ""
    provider_name: str = ""
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    messages: List["Message"] = Relationship(back_populates="conversation")

class Message(SQLModel, table=True):
    id: str = Field(primary_key=True)
    conversation_id: str = Field(foreign_key="conversation.id")
    role: str
    content: str = ""
    reasoning: str = ""
    mode: str = "chat"
    model_id: str = ""
    created_at: datetime = Field(default_factory=datetime.utcnow)
    conversation: Optional[Conversation] = Relationship(back_populates="messages")

class Skill(SQLModel, table=True):
    id: str = Field(primary_key=True)
    name: str
    description: str = ""
    system_prompt: str
    trigger: str = "manual"
    is_enabled: bool = True
    is_builtin: bool = False
    created_at: datetime = Field(default_factory=datetime.utcnow)

class AgentDefinition(SQLModel, table=True):
    id: str = Field(primary_key=True)
    name: str
    description: str = ""
    graph_json: str = "{}"
    created_at: datetime = Field(default_factory=datetime.utcnow)

class AgentRun(SQLModel, table=True):
    id: str = Field(primary_key=True)
    agent_id: str
    status: str = "running"
    steps_json: str = "[]"
    result: str = ""
    created_at: datetime = Field(default_factory=datetime.utcnow)

class UsageLog(SQLModel, table=True):
    id: Optional[int] = Field(default=None, primary_key=True)
    conversation_id: str
    model_id: str
    provider_name: str
    input_tokens: int = 0
    output_tokens: int = 0
    cost_usd: float = 0.0
    mode: str = "chat"
    created_at: datetime = Field(default_factory=datetime.utcnow)
