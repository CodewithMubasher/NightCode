export interface ToolEvent {
  id: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  toolId: string
  name: string
  input?: string
  output?: string
  duration?: number
  error?: string
  timestamp: number
}

export interface AgentEvent {
  id: string
  type: "agent.started" | "agent.completed" | "agent.error"
  error?: string
  timestamp: number
}

export interface MessageEvent {
  id: string
  type: "user.message" | "assistant.delta" | "assistant.message.completed"
  content?: string
  text?: string
  timestamp: number
}

export type RuntimeEvent = ToolEvent | AgentEvent | MessageEvent

export function generateEventId(): string {
  return crypto.randomUUID()
}
