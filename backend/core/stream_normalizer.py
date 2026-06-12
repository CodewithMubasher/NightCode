import json
import re

EMOJI_PATTERN = re.compile(
    "[\U0001F600-\U0001F64F"
    "\U0001F300-\U0001F5FF"
    "\U0001F680-\U0001F6FF"
    "\U0001F1E0-\U0001F1FF"
    "\U00002702-\U000027B0"
    "\U000024C2-\U0001F251"
    "\U0001F900-\U0001F9FF"
    "\U0001FA00-\U0001FA6F"
    "\U0001FA70-\U0001FAFF"
    "\U00002600-\U000026FF"
    "\U0000200D"
    "\U0000FE0F"
    "\U0000200B-\U0000200F"
    "\U00000300-\U0000036F"
    "\U00002B50"
    "\U00002764"
    "\U00002500-\U00002BEF"
    "\U00002300-\U000023FF"
    "\U000025A0-\U000025FF"
    "]+",
    re.UNICODE,
)

EVENT_VERSION = 1


def strip_emojis(text: str) -> str:
    return EMOJI_PATTERN.sub("", text).strip()


def _sse_serialize(data: dict) -> str:
    return json.dumps(data, ensure_ascii=False, separators=(",", ":"))


def sse(data: dict) -> str:
    return f"data: {_sse_serialize(data)}\n\n"


def sse_done() -> str:
    return "data: [DONE]\n\n"


def sse_chunk(text: str) -> str:
    return sse({"v": EVENT_VERSION, "type": "chunk", "content": strip_emojis(text)})


def sse_thinking(text: str) -> str:
    return sse({"v": EVENT_VERSION, "type": "thinking", "content": strip_emojis(text)})


def sse_reasoning(text: str) -> str:
    return sse({"v": EVENT_VERSION, "type": "reasoning", "content": strip_emojis(text)})


def sse_tool_call(tool_id: str, tool_name: str, args: dict) -> str:
    return sse({
        "v": EVENT_VERSION,
        "type": "tool_call",
        "id": tool_id,
        "name": tool_name,
        "args": args,
    })


def sse_tool_result(tool_id: str, tool_name: str, output: str, status: str) -> str:
    return sse({
        "v": EVENT_VERSION,
        "type": "tool_result",
        "id": tool_id,
        "name": tool_name,
        "output": strip_emojis(output),
        "status": status,
    })


def sse_final(content: str) -> str:
    return sse({"v": EVENT_VERSION, "type": "final", "content": strip_emojis(content)})


def sse_error(message: str) -> str:
    return sse({"v": EVENT_VERSION, "type": "error", "content": strip_emojis(message)})
