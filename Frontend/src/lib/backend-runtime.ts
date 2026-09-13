import type { RuntimeEvent } from "@/types/events"

const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:3001"

export interface Artifact {
  id: string
  chat_id: string
  name: string
  artifact_type: string
  language: string
  content: string
  created_at: string
}

export interface ChatRow {
  id: string
  workspace_id: string
  title: string
  created_at: string
  updated_at: string
}

export interface MessageRow {
  id: string
  chat_id: string
  role: string
  segments: string
  created_at: string
}

export interface Workspace {
  id: string
  name: string
  description: string
  created_at: string
}

export interface ModelOption {
  provider: string // "gemini" | "groq" | "opencode-zen" | "openrouter" | "web2api"
  id: string
  label: string
  default: boolean
}

/**
 * Fetches the real, currently-usable models from the backend (only models
 * for providers that have an API key configured are returned).
 */
export async function fetchModels(): Promise<ModelOption[]> {
  const url = `${API_BASE}/api/models`
  try {
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    return await response.json()
  } catch (e) {
    console.warn("Failed to fetch models:", e)
    return []
  }
}

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
  model?: ModelOption | null,
): () => void {
  const controller = new AbortController()

  const ws = workspaceId || "default"
  const url = `${API_BASE}/api/workspaces/${ws}/chats/${chatId}/messages`

  fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      message,
      ...(model ? { provider: model.provider, model: model.id } : {}),
    }),
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

/**
 * Fetches artifacts for a chat from the backend.
 */
export async function fetchArtifacts(chatId: string, workspaceId?: string): Promise<Artifact[]> {
  const ws = workspaceId || "default"
  const url = `${API_BASE}/api/workspaces/${ws}/chats/${chatId}/artifacts`
  try {
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    return await response.json()
  } catch (e) {
    console.warn("Failed to fetch artifacts:", e)
    return []
  }
}

export async function fetchWorkspaces(): Promise<Workspace[]> {
  const url = `${API_BASE}/api/workspaces`
  try {
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    return await response.json()
  } catch (e) {
    console.warn("Failed to fetch workspaces:", e)
    return []
  }
}

export async function createWorkspace(name: string, description: string): Promise<string | null> {
  const url = `${API_BASE}/api/workspaces`
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, description }),
    })
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    const data = await response.json()
    return data.id
  } catch (e) {
    console.warn("Failed to create workspace:", e)
    return null
  }
}

export async function deleteWorkspace(workspaceId: string): Promise<boolean> {
  const url = `${API_BASE}/api/workspaces/${workspaceId}`
  try {
    const response = await fetch(url, { method: "DELETE" })
    return response.ok
  } catch (e) {
    console.warn("Failed to delete workspace:", e)
    return false
  }
}

export async function fetchChats(workspaceId: string): Promise<ChatRow[]> {
  const url = `${API_BASE}/api/workspaces/${workspaceId}/chats`
  try {
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    return await response.json()
  } catch (e) {
    console.warn("Failed to fetch chats:", e)
    return []
  }
}

export async function fetchMessages(chatId: string): Promise<MessageRow[]> {
  const url = `${API_BASE}/api/workspaces/default/chats/${chatId}/messages`
  try {
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    return await response.json()
  } catch (e) {
    console.warn("Failed to fetch messages:", e)
    return []
  }
}
