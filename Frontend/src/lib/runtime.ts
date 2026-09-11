import { type RuntimeEvent, generateEventId } from "@/types/events"

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

const readToolCallId = generateEventId()
const editToolCallId = generateEventId()

function typeText(
  text: string,
  onDelta: (text: string) => void,
  onDone: () => void
) {
  let currentIndex = 0
  const chars = text.split("")
  const typeInterval = setInterval(() => {
    if (currentIndex < chars.length) {
      onDelta(chars.slice(0, currentIndex + 1).join(""))
      currentIndex++
    } else {
      clearInterval(typeInterval)
      onDone()
    }
  }, 20)
}

export interface RuntimeCallbacks {
  onEvent: EventHandler
}

export function emitFakeRuntime(callbacks: RuntimeCallbacks) {
  const timers: ReturnType<typeof setTimeout>[] = []
  const initialResponse = initialResponses[Math.floor(Math.random() * initialResponses.length)]
  const summaryResponse = summaryResponses[Math.floor(Math.random() * summaryResponses.length)]

  const cleanupTyping = { current: null as (() => void) | null }

  const cleanup = () => {
    timers.forEach(clearTimeout)
    cleanupTyping.current?.()
  }

  const onInitialDelta = (text: string) => {
    callbacks.onEvent({
      type: "assistant.delta",
      id: generateEventId(),
      text,
      timestamp: Date.now(),
    })
  }

  const onInitialDone = () => {
    setTimeout(() => {
      callbacks.onEvent({
        type: "tool.started",
        id: generateEventId(),
        toolCallId: readToolCallId,
        name: "read_file",
        input: "input.tsx",
        timestamp: Date.now(),
      })

      timers.push(setTimeout(() => {
        callbacks.onEvent({
          type: "tool.completed",
          id: generateEventId(),
          toolCallId: readToolCallId,
          name: "read_file",
          timestamp: Date.now(),
        })
      }, 2000))

      timers.push(setTimeout(() => {
        callbacks.onEvent({
          type: "tool.started",
          id: generateEventId(),
          toolCallId: editToolCallId,
          name: "edit_file",
          input: "input.tsx",
          timestamp: Date.now(),
        })
      }, 2500))

      timers.push(setTimeout(() => {
        callbacks.onEvent({
          type: "tool.completed",
          id: generateEventId(),
          toolCallId: editToolCallId,
          name: "edit_file",
          timestamp: Date.now(),
        })
      }, 4500))

      timers.push(setTimeout(() => {
        callbacks.onEvent({
          type: "agent.completed",
          id: generateEventId(),
          timestamp: Date.now(),
        })

        typeText(
          summaryResponse,
          (text) => {
            callbacks.onEvent({
              type: "assistant.delta",
              id: generateEventId(),
              text,
              timestamp: Date.now(),
            })
          },
          () => {
            callbacks.onEvent({
              type: "assistant.delta",
              id: generateEventId(),
              text: "__DONE__",
              timestamp: Date.now(),
            })
          }
        )
      }, 5000))
    }, 500)
  }

  timers.push(setTimeout(() => {
    typeText(initialResponse, onInitialDelta, onInitialDone)
  }, 500))

  return cleanup
}
