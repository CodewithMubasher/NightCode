import type { RuntimeEvent } from "@/types/events"

const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:3001"

export interface BackendRuntimeCallbacks {
  onEvent: (event: RuntimeEvent) => void
}

/**
 * Starts a real SSE stream from the Go backend.
 * Returns an abort function to cancel the stream.
 */
export function emitBackendRuntime(
  callbacks: BackendRuntimeCallbacks,
  message: string,
  chatId: string,
  workspaceId?: string,
): () => void {
  const controller = new AbortController()

  const ws = workspaceId || "default"
  const url = `${API_BASE}/api/workspaces/${ws}/chats/${chatId}/messages`

  fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message }),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`)
      }

      const reader = response.body?.getReader()
      if (!reader) throw new Error("No response body")

      const decoder = new TextDecoder()
      let buffer = ""

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        // Parse SSE: lines starting with "data: " followed by blank line
        const lines = buffer.split("\n")
        buffer = lines.pop() || ""

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const jsonStr = line.slice(6).trim()
            if (!jsonStr) continue
            try {
              const event = JSON.parse(jsonStr) as RuntimeEvent
              callbacks.onEvent(event)
            } catch (e) {
              console.warn("Failed to parse SSE event:", jsonStr, e)
            }
          }
        }
      }
    })
    .catch((err) => {
      if (err.name === "AbortError") return // user cancelled
      console.error("Backend runtime error:", err)
      callbacks.onEvent({
        type: "agent.error",
        id: crypto.randomUUID(),
        timestamp: Date.now(),
        error: err instanceof Error ? err.message : "Unknown error",
      })
    })

  return () => controller.abort()
}

/**
 * Cancels an in-flight run on the backend.
 */
export async function cancelBackendRun(chatId: string, workspaceId?: string): Promise<void> {
  const ws = workspaceId || "default"
  const url = `${API_BASE}/api/workspaces/${ws}/chats/${chatId}/runs/current`
  try {
    await fetch(url, { method: "DELETE" })
  } catch (e) {
    console.warn("Failed to cancel run:", e)
  }
}
