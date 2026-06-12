from typing import Callable, Optional
from dataclasses import dataclass, field
from enum import Enum
import asyncio
import time
import json


class ToolStatus(str, Enum):
    HEALTHY = "healthy"
    STARTING = "starting"
    OFFLINE = "offline"
    FAILED = "failed"


@dataclass
class ToolInfo:
    name: str
    description: str
    parameters: dict
    source: str
    server_name: Optional[str] = None
    timeout: int = 30
    status: ToolStatus = ToolStatus.HEALTHY
    status_message: str = ""
    fn: Optional[Callable] = None
    manager_ref: Optional[object] = None


@dataclass
class ToolResult:
    tool_call_id: str
    name: str
    output: str
    status: str
    duration_ms: int = 0


class ToolRegistry:
    def __init__(self):
        self._tools: dict[str, ToolInfo] = {}
        self._in_flight: dict[str, asyncio.Task] = {}

    def register_python(
        self,
        name: str,
        fn: Callable,
        description: str,
        parameters: Optional[dict] = None,
        timeout: int = 30,
    ) -> None:
        if parameters is None:
            parameters = {"type": "object", "properties": {}}
        self._tools[name] = ToolInfo(
            name=name,
            description=description,
            parameters=parameters,
            source="python",
            timeout=timeout,
            status=ToolStatus.HEALTHY,
            fn=fn,
        )

    def register_mcp(
        self,
        name: str,
        server_name: str,
        description: str,
        parameters: dict,
        timeout: int = 30,
        manager: object = None,
    ) -> None:
        self._tools[name] = ToolInfo(
            name=name,
            description=description,
            parameters=parameters,
            source="mcp",
            server_name=server_name,
            timeout=timeout,
            status=ToolStatus.OFFLINE,
            manager_ref=manager,
        )

    def resolve_tool_name(self, litellm_name: str) -> str:
        return litellm_name

    def get(self, name: str) -> Optional[ToolInfo]:
        return self._tools.get(name)

    def list_tools(self) -> list[ToolInfo]:
        return list(self._tools.values())

    def get_llm_tools(self) -> list[dict]:
        result = []
        for info in self._tools.values():
            if info.status == ToolStatus.OFFLINE:
                continue
            result.append({
                "type": "function",
                "function": {
                    "name": info.name,
                    "description": info.description,
                    "parameters": info.parameters,
                },
            })
        return result

    def get_tool_status(self, name: str) -> ToolStatus:
        info = self._tools.get(name)
        return info.status if info else ToolStatus.OFFLINE

    def get_all_statuses(self) -> dict[str, str]:
        return {n: t.status.value for n, t in self._tools.items()}

    def set_status(self, name: str, status: ToolStatus, message: str = "") -> None:
        info = self._tools.get(name)
        if info:
            info.status = status
            info.status_message = message

    async def execute(self, tool_call_id: str, name: str, args: dict) -> ToolResult:
        info = self._tools.get(name)
        if not info:
            return ToolResult(
                tool_call_id=tool_call_id,
                name=name,
                output=f"unknown tool: {name}",
                status="failed",
            )

        if info.status == ToolStatus.OFFLINE and info.source == "mcp":
            if info.manager_ref and hasattr(info.manager_ref, "start_server"):
                self.set_status(name, ToolStatus.STARTING, "starting MCP server...")
                ok = await info.manager_ref.start_server(info.server_name)
                if not ok:
                    self.set_status(name, ToolStatus.FAILED, "MCP server failed to start")
                    return ToolResult(
                        tool_call_id=tool_call_id,
                        name=name,
                        output="MCP server failed to start",
                        status="failed",
                    )
                self.set_status(name, ToolStatus.HEALTHY)

        start = time.monotonic()
        try:
            if info.source == "python" and info.fn:
                result = await asyncio.wait_for(
                    info.fn(**args),
                    timeout=info.timeout,
                )
            elif info.source == "mcp" and info.manager_ref:
                result = await asyncio.wait_for(
                    info.manager_ref.call_tool(info.server_name, name, args),
                    timeout=info.timeout,
                )
            else:
                return ToolResult(
                    tool_call_id=tool_call_id,
                    name=name,
                    output="no handler registered",
                    status="failed",
                )

            duration_ms = int((time.monotonic() - start) * 1000)
            output_str = str(result)[:500] if result else ""
            return ToolResult(
                tool_call_id=tool_call_id,
                name=name,
                output=output_str,
                status="ok",
                duration_ms=duration_ms,
            )

        except asyncio.TimeoutError:
            duration_ms = int((time.monotonic() - start) * 1000)
            return ToolResult(
                tool_call_id=tool_call_id,
                name=name,
                output=f"timed out after {info.timeout}s",
                status="failed",
                duration_ms=duration_ms,
            )
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            return ToolResult(
                tool_call_id=tool_call_id,
                name=name,
                output=f"{type(e).__name__}: {e}",
                status="failed",
                duration_ms=duration_ms,
            )
