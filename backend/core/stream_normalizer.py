import json


def _sse_serialize(data: dict) -> str:
    return json.dumps(data, ensure_ascii=False, separators=(',', ':'))


def sse(data: dict) -> str:
    return f"data: {_sse_serialize(data)}\n\n"


def sse_done() -> str:
    return "data: [DONE]\n\n"


def sse_chunk(text: str) -> str:
    return sse({"chunk": text})


def sse_thinking(text: str) -> str:
    return sse({"thinking": text})


def sse_reasoning(text: str) -> str:
    return sse({"reasoning": text})


def sse_decision(text: str) -> str:
    return sse({"decision": text})


def sse_observation(text: str) -> str:
    return sse({"observation": text})


def sse_synthesis(text: str) -> str:
    return sse({"synthesis": text})


def sse_final_answer(text: str) -> str:
    return sse({"final_answer": text})


def sse_tool_call(tool_id: str, tool_name: str, args: dict) -> str:
    return sse({"tool_call": {
        "id": tool_id,
        "name": tool_name,
        "tool": tool_name,
        "args": args,
        "input": args,
    }})


def sse_tool_result(tool_id: str, tool_name: str, result: str, status: str) -> str:
    return sse({"tool_result": {
        "id": tool_id,
        "name": tool_name,
        "output": result,
        "result": result,
        "status": status,
    }})


def sse_error(message: str) -> str:
    return sse({"error": message})
