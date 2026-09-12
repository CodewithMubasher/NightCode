export type RuntimeEvent =
  | AssistantDeltaEvent
  | ToolStartedEvent
  | ToolCompletedEvent
  | ToolFailedEvent
  | AgentCompletedEvent
  | AgentErrorEvent

export interface AssistantDeltaEvent {
  type: "assistant.delta"
  id: string
  segmentId: string
  text: string
  timestamp: number
}

export interface ToolStartedEvent {
  type: "tool.started"
  id: string
  segmentId: string
  toolCallId: string
  name: string
  input?: unknown
  timestamp: number
}

export interface ToolCompletedEvent {
  type: "tool.completed"
  id: string
  segmentId: string
  toolCallId: string
  name: string
  output?: unknown
  duration?: number
  timestamp: number
}

export interface ToolFailedEvent {
  type: "tool.failed"
  id: string
  segmentId: string
  toolCallId: string
  name: string
  error: string
  timestamp: number
}

export interface AgentCompletedEvent {
  type: "agent.completed"
  id: string
  timestamp: number
}

export interface AgentErrorEvent {
  type: "agent.error"
  id: string
  error: string
  timestamp: number
}

export function generateEventId(): string {
  return crypto.randomUUID()
}
