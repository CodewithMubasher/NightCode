from sqlmodel import SQLModel, Field, Relationship
from typing import Optional, List
from datetime import datetime
import json


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
    status: str = "complete"
    tool_calls: Optional[str] = None
    tool_results: Optional[str] = None
    heartbeat_at: Optional[str] = None
    mode: str = "chat"
    model_id: str = ""
    provider_name: str = ""
    created_at: datetime = Field(default_factory=datetime.utcnow)
    conversation: Optional[Conversation] = Relationship(back_populates="messages")

    def set_tool_calls(self, calls: list[dict]) -> None:
        self.tool_calls = json.dumps(calls, ensure_ascii=False)

    def get_tool_calls(self) -> list[dict]:
        if not self.tool_calls:
            return []
        return json.loads(self.tool_calls)

    def set_tool_results(self, results: list[dict]) -> None:
        self.tool_results = json.dumps(results, ensure_ascii=False)

    def get_tool_results(self) -> list[dict]:
        if not self.tool_results:
            return []
        return json.loads(self.tool_results)

    def append_tool_call(self, tc: dict) -> None:
        calls = self.get_tool_calls()
        calls.append(tc)
        self.set_tool_calls(calls)

    def append_tool_result(self, tr: dict) -> None:
        results = self.get_tool_results()
        results.append(tr)
        self.set_tool_results(results)


class Skill(SQLModel, table=True):
    id: str = Field(primary_key=True)
    name: str
    description: str = ""
    system_prompt: str
    trigger: str = "manual"
    is_enabled: bool = True
    is_builtin: bool = False
    created_at: datetime = Field(default_factory=datetime.utcnow)


class UsageLog(SQLModel, table=True):
    id: Optional[int] = Field(default=None, primary_key=True)
    conversation_id: str
    message_id: str = ""
    model_id: str
    provider_name: str
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    cost_usd: float = 0.0
    latency_ms: int = 0
    mode: str = "chat"
    created_at: datetime = Field(default_factory=datetime.utcnow)
