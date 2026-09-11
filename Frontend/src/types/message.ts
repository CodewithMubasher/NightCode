export interface TextPart {
  type: "text"
  text: string
}

export interface ToolCallPart {
  type: "tool-call"
  toolCallId: string
  name: string
  input?: string
}

export interface ToolResultPart {
  type: "tool-result"
  toolCallId: string
  name: string
  output?: string
  duration?: number
  error?: string
}

export interface ArtifactPart {
  type: "artifact"
  id: string
  name: string
  content: string
}

export type MessagePart = TextPart | ToolCallPart | ToolResultPart | ArtifactPart

export interface Message {
  id: string
  role: "user" | "assistant"
  parts: MessagePart[]
  timestamp: number
}

export interface Chat {
  id: string
  title: string
  messages: Message[]
  createdAt: number
}

export function generateId(): string {
  return crypto.randomUUID()
}

export function truncateTitle(text: string, maxLength = 50): string {
  if (text.length <= maxLength) return text
  return text.slice(0, maxLength) + "..."
}
