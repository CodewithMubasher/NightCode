import os
import re
from pathlib import Path

class ToolError(Exception):
    pass

async def read_file(path: str) -> str:
    """Read a file from the filesystem. Path must be absolute."""
    p = Path(path)
    if not p.is_absolute():
        raise ToolError("path must be absolute")
    if not p.exists():
        raise ToolError(f"file not found: {path}")
    if p.is_dir():
        raise ToolError(f"path is a directory, not a file: {path}")
    if p.stat().st_size > 1_000_000:
        raise ToolError("file too large (max 1MB)")
    try:
        return p.read_text(encoding="utf-8", errors="replace")
    except Exception as e:
        raise ToolError(f"Error reading file: {e}")

async def write_file(path: str, content: str) -> str:
    """Write content to a file. Creates parent directories if needed."""
    p = Path(path)
    if not p.is_absolute():
        raise ToolError("path must be absolute")
    p.parent.mkdir(parents=True, exist_ok=True)
    try:
        p.write_text(content, encoding="utf-8")
        return f"Written {len(content)} bytes to {path}"
    except Exception as e:
        raise ToolError(f"Error writing file: {e}")

async def list_directory(path: str, recursive: bool = False) -> str:
    """List files and directories at the given path.
    
    Args:
        path: Absolute directory path.
        recursive: If True, list recursively (default: False).
    """
    p = Path(path)
    if not p.is_absolute():
        raise ToolError("path must be absolute")
    if not p.exists():
        raise ToolError(f"directory not found: {path}")
    if not p.is_dir():
        raise ToolError(f"path is not a directory: {path}")

    try:
        if recursive:
            items = list(p.rglob("*"))
        else:
            items = list(p.iterdir())

        lines = [f"Contents of: {path}\n"]
        for item in sorted(items, key=lambda x: (not x.is_dir(), x.name.lower())):
            suffix = "/" if item.is_dir() else ""
            size = item.stat().st_size if item.is_file() else 0
            size_str = f" ({size} bytes)" if size else ""
            lines.append(f"  {item.name}{suffix}{size_str}")
        return "\n".join(lines)
    except PermissionError as e:
        raise ToolError(f"Permission denied: {e}")
    except Exception as e:
        raise ToolError(f"Error listing directory: {e}")

async def grep_files(pattern: str, path: str, file_pattern: str = "*") -> str:
    """Search for a regex pattern in files within a directory.
    
    Args:
        pattern: Regex pattern to search for.
        path: Absolute directory path to search in.
        file_pattern: Glob pattern for files to search (default: '*').
    """
    p = Path(path)
    if not p.is_absolute():
        raise ToolError("path must be absolute")
    if not p.exists():
        raise ToolError(f"directory not found: {path}")
    if not p.is_dir():
        raise ToolError(f"path is not a directory: {path}")

    try:
        compiled = re.compile(pattern, re.IGNORECASE)
    except re.error as e:
        raise ToolError(f"Invalid regex pattern: {e}")

    matches = []
    max_results = 50
    for file_path in p.rglob(file_pattern):
        if not file_path.is_file():
            continue
        if file_path.stat().st_size > 500_000:
            continue
        try:
            text = file_path.read_text(encoding="utf-8", errors="replace")
            for i, line in enumerate(text.splitlines(), 1):
                if compiled.search(line):
                    rel = file_path.relative_to(p)
                    matches.append(f"{rel}:{i}: {line.strip()[:200]}")
                    if len(matches) >= max_results:
                        break
            if len(matches) >= max_results:
                break
        except Exception:
            continue

    if not matches:
        return f"No matches found for '{pattern}' in {path}"

    header = f"Found {len(matches)} match(es) for '{pattern}' in {path}:\n"
    return header + "\n".join(matches[:max_results])
