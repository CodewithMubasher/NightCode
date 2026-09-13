export type RuntimeEvent =
  | AssistantDeltaEvent
  | ToolStartedEvent
  | ToolCompletedEvent
  | ToolFailedEvent
  | AgentCompletedEvent
  | AgentErrorEvent
  | ArtifactCreatedEvent
  | TurnStartedEvent
  | TurnCompletedEvent
  | TurnErrorEvent
  | TurnRetryEvent

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

export interface ArtifactCreatedEvent {
  type: "artifact.created"
  id: string
  segmentId: string
  artifactId: string
  toolCallId?: string
  name: string
  content: string
  artifactType: "document" | "code"
  language?: string
  timestamp: number
}

// Backend turn-lifecycle events (emitted by the real agent loop, distinct
// from the mock runtime's agent.* events). turn.error is the one that
// actually carries provider/model failures (empty response, auth, rate
// limit, etc) and must be surfaced to the user — silently dropping it makes
// real failures look like a successful empty reply.
export interface TurnStartedEvent {
  type: "turn.started"
  id: string
  timestamp: number
}

export interface TurnCompletedEvent {
  type: "turn.completed"
  id: string
  timestamp: number
}

export interface TurnErrorEvent {
  type: "turn.error"
  id: string
  segmentId?: string
  error: string
  code?: string
  retryable?: boolean
  timestamp: number
}

// Non-terminal: the provider hit a retryable error (rate limit, transient
// 5xx) and is about to back off and try again. The turn is still in
// progress — do not treat this as a failure, just show the user what's
// happening instead of going silent for the whole backoff window.
export interface TurnRetryEvent {
  type: "turn.retry"
  id: string
  segmentId?: string
  error: string
  attempt: number
  maxAttempt: number
  retryAfterMs: number
  timestamp: number
}

export function generateEventId(): string {
  return crypto.randomUUID()
}
