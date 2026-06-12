from fastmcp import FastMCP
from tools.web_search import web_search, ToolError as WebToolError
from tools.code_executor import execute_code
from tools.file_ops import read_file, write_file, list_directory, grep_files, ToolError as FileToolError
import os

ToolError = WebToolError

mcp = FastMCP("NightCode Tools")

@mcp.tool()
async def search_web(query: str, num_results: int = 5) -> str:
    """Search the web for current information."""
    return await web_search(query, num_results)

@mcp.tool()
async def run_code(code: str) -> str:
    """Execute Python code and return the output."""
    return await execute_code(code)

@mcp.tool()
async def read_file_tool(path: str) -> str:
    """Read a file from the filesystem. Path must be absolute."""
    return await read_file(path)

@mcp.tool()
async def write_file_tool(path: str, content: str) -> str:
    """Write content to a file. Creates parent directories if needed."""
    return await write_file(path, content)

@mcp.tool()
async def list_directory_tool(path: str, recursive: bool = False) -> str:
    """List files and directories at the given path."""
    return await list_directory(path, recursive)

@mcp.tool()
async def grep_files_tool(pattern: str, path: str, file_pattern: str = "*") -> str:
    """Search for a regex pattern in files within a directory."""
    return await grep_files(pattern, path, file_pattern)
