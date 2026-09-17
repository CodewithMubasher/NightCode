"""
win-control-mcp.py
------------------
A FastMCP server that gives AI super-simple Windows control.
AI provides intent  MCP handles OS-level complexity.

V1 Tools (12):
  open_app, type_text, scroll, screenshot,
  open_file, create_file, create_folder,
  run_command, google_search, youtube_search,
  open_url, lock_screen
"""

import os
import sys
import time
import base64
import subprocess
import webbrowser
from io import BytesIO
from pathlib import Path

import pyautogui
import pyperclip
from fastmcp import FastMCP

# ------------------------------------------------------------------
# Setup
# ------------------------------------------------------------------
pyautogui.FAILSAFE = False          # don't abort on corner mouse move
pyautogui.PAUSE = 0.05              # small delay between pyautogui calls

mcp = FastMCP("win-control-mcp")


# ------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------
def _ok(msg: str, **extra):
    return {"success": True, "message": msg, **extra}


def _err(msg: str):
    return {"success": False, "message": msg}


def _normalize_path(path: str) -> str:
    """Expand ~, env vars, and make absolute."""
    path = os.path.expandvars(os.path.expanduser(path.strip().strip('"').strip("'")))
    return os.path.abspath(path)


def _focus_start_search():
    """Press Win key to open Start menu search."""
    pyautogui.press("win")
    time.sleep(0.6)     # let Start menu animate in


# ------------------------------------------------------------------
# 1. APP CONTROL
# ------------------------------------------------------------------
@mcp.tool()
def open_app(name: str) -> dict:
    """
    Open any Windows app by name using the Start menu.
    Example: open_app("notepad"), open_app("chrome"), open_app("vs code")
    """
    if not name or not name.strip():
        return _err("App name is required.")
    try:
        _focus_start_search()
        # Clear any leftover text
        pyautogui.hotkey("ctrl", "a")
        pyautogui.press("backspace")
        time.sleep(0.1)

        # Type the app name (clipboard avoids keyboard-layout issues)
        pyperclip.copy(name.strip())
        pyautogui.hotkey("ctrl", "v")
        time.sleep(0.8)                 # let search results appear
        pyautogui.press("enter")
        time.sleep(1.0)                 # give app time to launch
        return _ok(f"Opened app: {name}")
    except Exception as e:
        return _err(f"Failed to open '{name}': {e}")


# ------------------------------------------------------------------
# 2. INPUT
# ------------------------------------------------------------------
@mcp.tool()
def type_text(text: str) -> dict:
    """
    Type text into the currently focused window.
    Uses clipboard paste for unicode reliability.
    """
    if text is None:
        return _err("No text provided.")
    try:
        pyperclip.copy(text)
        pyautogui.hotkey("ctrl", "v")
        return _ok(f"Typed {len(text)} characters.")
    except Exception as e:
        return _err(f"Failed to type text: {e}")


@mcp.tool()
def scroll(direction: str, amount: int = 500) -> dict:
    """
    Scroll the active window.
    direction: 'up' or 'down'
    amount: pixels to scroll (default 500)
    """
    d = direction.strip().lower()
    if d not in ("up", "down"):
        return _err("direction must be 'up' or 'down'.")
    try:
        clicks = amount if d == "up" else -amount
        pyautogui.scroll(clicks)
        return _ok(f"Scrolled {d} by {amount}.")
    except Exception as e:
        return _err(f"Scroll failed: {e}")


# ------------------------------------------------------------------
# 3. SCREEN
# ------------------------------------------------------------------
@mcp.tool()
def screenshot() -> dict:
    """
    Capture the full screen and return it as a base64-encoded PNG.
    AI can decode + view the image.
    """
    try:
        img = pyautogui.screenshot()
        buf = BytesIO()
        img.save(buf, format="PNG")
        b64 = base64.b64encode(buf.getvalue()).decode("utf-8")
        return {
            "success": True,
            "message": "Screenshot captured.",
            "format": "png",
            "width": img.width,
            "height": img.height,
            "image_base64": b64,
        }
    except Exception as e:
        return _err(f"Screenshot failed: {e}")


# ------------------------------------------------------------------
# 4. FILES / FOLDERS
# ------------------------------------------------------------------
@mcp.tool()
def open_file(path: str) -> dict:
    """
    Open any file or folder using the Windows default application.
    Handles ~, env vars, quotes, forward/back slashes.
    """
    try:
        p = _normalize_path(path)
        if not os.path.exists(p):
            return _err(f"Path does not exist: {p}")
        os.startfile(p)  # type: ignore[attr-defined]
        return _ok(f"Opened: {p}")
    except Exception as e:
        return _err(f"Failed to open '{path}': {e}")


@mcp.tool()
def create_file(path: str, content: str = "") -> dict:
    """
    Create a file (and any missing parent folders).
    Optionally write content.
    """
    try:
        p = _normalize_path(path)
        parent = os.path.dirname(p)
        if parent:
            os.makedirs(parent, exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(content or "")
        return _ok(f"Created file: {p}", path=p, bytes=len(content or ""))
    except Exception as e:
        return _err(f"Failed to create file '{path}': {e}")


@mcp.tool()
def create_folder(path: str) -> dict:
    """Create a folder (including any missing parents)."""
    try:
        p = _normalize_path(path)
        os.makedirs(p, exist_ok=True)
        return _ok(f"Created folder: {p}", path=p)
    except Exception as e:
        return _err(f"Failed to create folder '{path}': {e}")


# ------------------------------------------------------------------
# 5. COMMAND EXECUTION (via visible PowerShell window)
# ------------------------------------------------------------------
@mcp.tool()
def run_command(powershell: str) -> dict:
    """
    Run a PowerShell command in a visible terminal window.
    Under the hood: opens PowerShell via Start menu, types the command, presses Enter.
    Great for AI to "show the user" what's running.
    """
    if not powershell or not powershell.strip():
        return _err("Command is required.")
    try:
        # Open PowerShell using the same Start-menu approach
        _focus_start_search()
        pyautogui.hotkey("ctrl", "a")
        pyautogui.press("backspace")
        time.sleep(0.1)
        pyperclip.copy("powershell")
        pyautogui.hotkey("ctrl", "v")
        time.sleep(0.8)
        pyautogui.press("enter")
        time.sleep(1.8)                 # wait for PowerShell window

        # Type the command
        pyperclip.copy(powershell.strip())
        pyautogui.hotkey("ctrl", "v")
        time.sleep(0.3)
        pyautogui.press("enter")
        return _ok(f"Executed PowerShell: {powershell}")
    except Exception as e:
        return _err(f"run_command failed: {e}")


# ------------------------------------------------------------------
# 6. WEB
# ------------------------------------------------------------------
@mcp.tool()
def open_url(url: str) -> dict:
    """Open any URL in the default browser. Auto-adds https:// if missing."""
    if not url or not url.strip():
        return _err("URL is required.")
    u = url.strip()
    if not (u.startswith("http://") or u.startswith("https://")):
        u = "https://" + u
    try:
        webbrowser.open(u)
        return _ok(f"Opened URL: {u}")
    except Exception as e:
        return _err(f"Failed to open URL: {e}")


@mcp.tool()
def google_search(query: str) -> dict:
    """Search Google for the given query."""
    if not query or not query.strip():
        return _err("Query is required.")
    from urllib.parse import quote_plus
    url = f"https://www.google.com/search?q={quote_plus(query.strip())}"
    try:
        webbrowser.open(url)
        return _ok(f"Google search: {query}")
    except Exception as e:
        return _err(f"Google search failed: {e}")


@mcp.tool()
def youtube_search(query: str) -> dict:
    """Search YouTube for the given query."""
    if not query or not query.strip():
        return _err("Query is required.")
    from urllib.parse import quote_plus
    url = f"https://www.youtube.com/results?search_query={quote_plus(query.strip())}"
    try:
        webbrowser.open(url)
        return _ok(f"YouTube search: {query}")
    except Exception as e:
        return _err(f"YouTube search failed: {e}")


# ------------------------------------------------------------------
# 7. SYSTEM
# ------------------------------------------------------------------
@mcp.tool()
def lock_screen() -> dict:
    """Lock the Windows workstation (Win+L)."""
    try:
        pyautogui.hotkey("win", "l")
        return _ok("Screen locked.")
    except Exception as e:
        return _err(f"Lock failed: {e}")


# ------------------------------------------------------------------
# Entry point
# ------------------------------------------------------------------
if __name__ == "__main__":
    mcp.run()