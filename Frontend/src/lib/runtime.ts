import { generateEventId } from "@/types/events"

export interface RuntimeEvent {
  id: string
  type: string
  name?: string
  input?: string
  timestamp: number
}

type EventHandler = (event: RuntimeEvent) => void

const initialResponses = [
  "Let me check the file then edit it according to your need.",
  "I'll look into that and make the necessary changes.",
  "Let me read the file first and update it.",
  "I'll examine the code and fix it for you.",
]

const summaryResponses = [
  "I've reviewed the file structure and made the necessary updates. The component now uses the correct prop types, and I've cleaned up the unused imports. The changes follow the existing code conventions and should integrate seamlessly with the rest of the codebase.",
  "Done! I've updated the code to fix the type definitions and improved the overall structure. The refactoring includes better error handling, optimized imports, and clearer variable naming. Everything compiles without warnings.",
  "All set. I've applied the modifications successfully — the component now properly handles edge cases, the state management is cleaner, and the API calls are more efficient. No breaking changes were introduced.",
  "Finished! The refactoring is complete. I've reorganized the module structure, extracted reusable utilities, and added proper TypeScript types throughout. The code is now more maintainable and follows best practices.",
]

const toolSequence = [
  { delay: 1500, type: "tool.started", name: "read_file", input: "input.tsx" },
  { delay: 3500, type: "tool.completed", name: "read_file" },
  { delay: 4000, type: "tool.started", name: "edit_file", input: "input.tsx" },
  { delay: 6000, type: "tool.completed", name: "edit_file" },
  { delay: 6500, type: "agent.completed" },
]

export interface RuntimeCallbacks {
  onEvent: EventHandler
  onInitialResponse: (text: string) => void
  onSummary: (text: string) => void
}

export function emitFakeRuntime(callbacks: RuntimeCallbacks) {
  const timers: ReturnType<typeof setTimeout>[] = []

  const initialResponse = initialResponses[Math.floor(Math.random() * initialResponses.length)]
  const summaryResponse = summaryResponses[Math.floor(Math.random() * summaryResponses.length)]

  const initialTimer = setTimeout(() => {
    callbacks.onInitialResponse(initialResponse)
  }, 500)
  timers.push(initialTimer)

  toolSequence.forEach((step) => {
    const timer = setTimeout(() => {
      callbacks.onEvent({
        id: generateEventId(),
        type: step.type,
        name: step.name,
        input: step.input,
        timestamp: Date.now(),
      })
    }, step.delay)
    timers.push(timer)
  })

  const summaryTimer = setTimeout(() => {
    callbacks.onSummary(summaryResponse)
  }, 7500)
  timers.push(summaryTimer)

  return () => timers.forEach(clearTimeout)
}
