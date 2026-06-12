import httpx
import os

SERPER_API_KEY = os.getenv("SERPER_API_KEY", "")

class ToolError(Exception):
    pass

async def web_search(query: str, num_results: int = 5) -> str:
    if not SERPER_API_KEY:
        raise ToolError("Web search unavailable: SERPER_API_KEY not set.")

    async with httpx.AsyncClient() as client:
        try:
            resp = await client.post(
                "https://google.serper.dev/search",
                headers={"X-API-KEY": SERPER_API_KEY, "Content-Type": "application/json"},
                json={"q": query, "num": num_results},
                timeout=10.0,
            )
            data = resp.json()
            results = data.get("organic", [])

            if not results:
                raise ToolError(f"No results found for: {query}")

            lines = [f"Search results for: {query}\n"]
            for i, r in enumerate(results[:num_results], 1):
                lines.append(f"{i}. {r.get('title', '')}")
                lines.append(f"   {r.get('snippet', '')}")
                lines.append(f"   URL: {r.get('link', '')}\n")

            return "\n".join(lines)

        except httpx.HTTPError as e:
            raise ToolError(f"Search request failed: {e}")
        except ToolError:
            raise
        except Exception as e:
            raise ToolError(f"Search error: {e}")
