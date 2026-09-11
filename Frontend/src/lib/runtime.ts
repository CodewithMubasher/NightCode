import { generateEventId } from "@/types/events"

export interface RuntimeEvent {
  id: string
  type: string
  name?: string
  input?: string
  timestamp: number
}

type EventHandler = (event: RuntimeEvent) => void

const toolSequence = [
  { delay: 0, type: "agent.started" },
  { delay: 1000, type: "tool.started", name: "read_file", input: "input.tsx" },
  { delay: 3000, type: "tool.completed", name: "read_file" },
  { delay: 3500, type: "tool.started", name: "edit_file", input: "input.tsx + 7-13" },
  { delay: 5500, type: "tool.completed", name: "edit_file" },
  { delay: 6000, type: "agent.completed" },
]

export function emitFakeRuntime(onEvent: EventHandler, onComplete: () => void) {
  const timers: ReturnType<typeof setTimeout>[] = []

  toolSequence.forEach((step) => {
    const timer = setTimeout(() => {
      onEvent({
        id: generateEventId(),
        type: step.type,
        name: step.name,
        input: step.input,
        timestamp: Date.now(),
      })
    }, step.delay)
    timers.push(timer)
  })

  const textTimer = setTimeout(() => {
    onComplete()
  }, 7000)
  timers.push(textTimer)

  return () => timers.forEach(clearTimeout)
}
