import asyncio
import ast
from typing import Any

SAFE_BUILTINS = {
    "print", "len", "range", "enumerate", "zip", "map", "filter",
    "sorted", "reversed", "sum", "min", "max", "abs", "round",
    "int", "float", "str", "bool", "list", "dict", "tuple", "set",
    "isinstance", "type", "hasattr", "getattr",
}

BLOCKED_IMPORTS = {
    "os", "sys", "subprocess", "socket", "shutil", "pathlib",
    "importlib", "ctypes", "multiprocessing", "threading",
}

class ToolError(Exception):
    pass

def _is_safe_code(code: str) -> tuple[bool, str]:
    try:
        tree = ast.parse(code)
    except SyntaxError as e:
        return False, f"Syntax error: {e}"

    for node in ast.walk(tree):
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            names = [a.name for a in node.names] if isinstance(node, ast.Import) else [node.module]
            for name in names:
                if name and name.split(".")[0] in BLOCKED_IMPORTS:
                    return False, (
                        f"Import '{name}' is blocked for security. "
                        f"Available alternatives: use built-in functions only. "
                        f"For math: use math module. For data: use lists/dicts. "
                        f"For file operations: use the read_file, write_file, list_directory tools instead of os/pathlib."
                    )

    return True, ""

async def execute_code(code: str, timeout: int = 10) -> str:
    is_safe, reason = _is_safe_code(code)
    if not is_safe:
        raise ToolError(reason)

    import io
    import contextlib

    output = io.StringIO()
    local_vars: dict[str, Any] = {}

    def run():
        with contextlib.redirect_stdout(output):
            exec(
                compile(code, "<nightcode>", "exec"),
                {"__builtins__": {k: __builtins__[k] for k in SAFE_BUILTINS if k in __builtins__}},
                local_vars,
            )

    try:
        loop = asyncio.get_event_loop()
        await asyncio.wait_for(
            loop.run_in_executor(None, run),
            timeout=timeout
        )
        result = output.getvalue()
        return result if result else "Code executed successfully (no output)."
    except asyncio.TimeoutError:
        raise ToolError(f"Execution timed out after {timeout}s.")
    except Exception as e:
        raise ToolError(f"Execution error: {type(e).__name__}: {e}")
