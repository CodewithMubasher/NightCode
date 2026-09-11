export interface MessageToolEvent {
  id: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: string
  output?: string
  duration?: number
  timestamp: number
}

export interface Message {
  id: string
  role: "user" | "assistant"
  content: string
  timestamp: number
  toolEvents?: MessageToolEvent[]
  initialText?: string
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
