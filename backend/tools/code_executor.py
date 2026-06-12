import asyncio
import ast
from typing import Any

ALLOWED_IMPORTS = {
    "math", "json", "random", "datetime", "collections", "itertools",
    "functools", "string", "re", "decimal", "pathlib",
}

READONLY_OS_FUNCS = {"getcwd", "listdir", "walk", "scandir", "stat", "lstat", "getpid", "cpu_count", "uname", "name"}

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
                base = name.split(".")[0] if name else ""
                if base and base not in ALLOWED_IMPORTS:
                    return False, (
                        f"Import '{name}' is not allowed. "
                        f"Allowed: {', '.join(sorted(ALLOWED_IMPORTS))}. "
                        f"For file ops use write_file/read_file/list_directory tools."
                    )
        if isinstance(node, ast.Call):
            if isinstance(node.func, ast.Attribute):
                if isinstance(node.func.value, ast.Name) and node.func.value.id == "os":
                    if node.func.attr not in READONLY_OS_FUNCS:
                        return False, f"os.{node.func.attr}() is blocked (read-only os functions only: {', '.join(sorted(READONLY_OS_FUNCS))})"

    return True, ""


async def execute_code(code: str, timeout: int = 10) -> str:
    is_safe, reason = _is_safe_code(code)
    if not is_safe:
        raise ToolError(reason)

    import io
    import contextlib

    output = io.StringIO()
    local_vars: dict[str, Any] = {}

    safe_builtins = dict(__builtins__) if isinstance(__builtins__, dict) else __builtins__.__dict__.copy()
    safe_builtins["__import__"] = __import__
    safe_builtins["open"] = open
    safe_builtins["dir"] = dir
    safe_builtins["vars"] = vars

    def run():
        with contextlib.redirect_stdout(output):
            exec(
                compile(code, "<nightcode>", "exec"),
                {"__builtins__": safe_builtins, "os": __import__("os")},
                local_vars,
            )

    try:
        loop = asyncio.get_event_loop()
        await asyncio.wait_for(
            loop.run_in_executor(None, run),
            timeout=timeout,
        )
        result = output.getvalue()
        return result if result else "Code executed successfully (no output)."
    except asyncio.TimeoutError:
        raise ToolError(f"Execution timed out after {timeout}s.")
    except Exception as e:
        raise ToolError(f"Execution error: {type(e).__name__}: {e}")
