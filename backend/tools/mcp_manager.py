import asyncio
import json
from typing import Optional

MCP_SERVER_CONFIG: dict[str, dict] = {}


def configure_servers(config: dict[str, dict]) -> None:
    MCP_SERVER_CONFIG.clear()
    MCP_SERVER_CONFIG.update(config)


class MCPManager:
    def __init__(self):
        self._processes: dict[str, asyncio.subprocess.Process] = {}
        self._readers: dict[str, asyncio.StreamReader] = {}
        self._writers: dict[str, asyncio.StreamWriter] = {}
        self._server_tools: dict[str, list[dict]] = {}
        self._pending_id = 0

    async def start_server(self, server_name: str) -> bool:
        if server_name in self._processes:
            proc = self._processes[server_name]
            if proc.returncode is None:
                return True
            del self._processes[server_name]

        config = MCP_SERVER_CONFIG.get(server_name)
        if not config:
            return False

        try:
            proc = await asyncio.create_subprocess_exec(
                config["command"],
                *config.get("args", []),
                stdin=asyncio.subprocess.PIPE,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            self._processes[server_name] = proc
            self._readers[server_name] = proc.stdout
            self._writers[server_name] = proc.stdin

            tools = await self._list_tools(server_name)
            self._server_tools[server_name] = tools
            return True
        except Exception:
            return False

    async def stop_server(self, server_name: str) -> None:
        proc = self._processes.pop(server_name, None)
        if proc and proc.returncode is None:
            try:
                proc.terminate()
                await asyncio.wait_for(proc.wait(), timeout=5)
            except Exception:
                proc.kill()
        self._readers.pop(server_name, None)
        self._writers.pop(server_name, None)
        self._server_tools.pop(server_name, None)

    async def stop_all(self) -> None:
        for name in list(self._processes.keys()):
            await self.stop_server(name)

    def get_tools_for_server(self, server_name: str) -> list[dict]:
        return self._server_tools.get(server_name, [])

    def _next_id(self) -> int:
        self._pending_id += 1
        return self._pending_id

    async def _send_request(self, server_name: str, method: str, params: dict | None = None) -> dict:
        writer = self._writers.get(server_name)
        reader = self._readers.get(server_name)
        if not writer or not reader:
            return {"error": "server not connected"}

        req_id = self._next_id()
        request = {
            "jsonrpc": "2.0",
            "id": req_id,
            "method": method,
        }
        if params:
            request["params"] = params

        data = json.dumps(request, ensure_ascii=False) + "\n"
        writer.write(data.encode())
        await writer.drain()

        line = await asyncio.wait_for(reader.readline(), timeout=10)
        response = json.loads(line.decode())
        return response.get("result", {})

    async def _list_tools(self, server_name: str) -> list[dict]:
        try:
            result = await self._send_request(server_name, "tools/list")
            return result.get("tools", [])
        except Exception:
            return []

    async def call_tool(self, server_name: str, tool_name: str, args: dict) -> str:
        stripped_name = tool_name.split("__", 2)[-1] if "__" in tool_name else tool_name
        try:
            result = await self._send_request(
                server_name,
                "tools/call",
                {"name": stripped_name, "arguments": args},
            )
            content = result.get("content", [])
            text_parts = []
            for item in content:
                if isinstance(item, dict):
                    text_parts.append(item.get("text", json.dumps(item)))
                else:
                    text_parts.append(str(item))
            return "\n".join(text_parts) if text_parts else json.dumps(result)
        except Exception as e:
            return f"Error: {e}"

    def health(self) -> dict[str, str]:
        result = {}
        for name, proc in self._processes.items():
            if proc.returncode is None:
                result[name] = "running"
            elif proc.returncode == 0:
                result[name] = "exited"
            else:
                result[name] = f"crashed (code {proc.returncode})"
        for name in MCP_SERVER_CONFIG:
            if name not in result:
                result[name] = "stopped"
        return result
