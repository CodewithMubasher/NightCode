import { type RuntimeEvent, generateEventId } from "@/types/events"

export type ScenarioCallbacks = {
  onEvent: (event: RuntimeEvent) => void
}

export type Scenario = (callbacks: ScenarioCallbacks) => () => void

function typeText(
  text: string,
  onDelta: (text: string) => void,
  onDone: () => void
): () => void {
  let currentIndex = 0
  let cancelled = false
  const chars = text.split("")
  const typeInterval = setInterval(() => {
    if (cancelled) {
      clearInterval(typeInterval)
      return
    }
    if (currentIndex < chars.length) {
      onDelta(chars.slice(0, currentIndex + 1).join(""))
      currentIndex++
    } else {
      clearInterval(typeInterval)
      onDone()
    }
  }, 7)
  return () => { cancelled = true; clearInterval(typeInterval) }
}

function emitDelta(text: string, cb: ScenarioCallbacks): RuntimeEvent {
  const event: RuntimeEvent = {
    type: "assistant.delta",
    id: generateEventId(),
    text,
    timestamp: Date.now(),
  }
  cb.onEvent(event)
  return event
}

function emitDone(cb: ScenarioCallbacks): RuntimeEvent {
  const event: RuntimeEvent = {
    type: "assistant.delta",
    id: generateEventId(),
    text: "__DONE__",
    timestamp: Date.now(),
  }
  cb.onEvent(event)
  return event
}

// Scenario 01: Simple response (no tools)
export const simpleResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)

  timers.push(setTimeout(() => {
    const text = "Sure! Here's a quick answer: the component is working correctly. No changes needed."
    const cancelType = typeText(text,
      (t) => emitDelta(t, cb),
      () => emitDone(cb)
    )
    timers.push({ clearTimeout: cancelType } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Scenario 02: Streaming response (text in small chunks)
export const streamingResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)

  timers.push(setTimeout(() => {
    const chunks = [
      "I've analyzed the codebase. ",
      "Here are my findings: ",
      "\n\n1. The type definitions need updating. ",
      "2. Some imports are unused. ",
      "3. The component structure could be cleaner. ",
      "\nLet me fix these issues for you."
    ]
    let fullText = ""
    let i = 0

    const interval = setInterval(() => {
      if (i < chunks.length) {
        fullText += chunks[i]
        emitDelta(fullText, cb)
        i++
      } else {
        clearInterval(interval)
        emitDone(cb)
      }
    }, 300)

    timers.push({ clearTimeout: () => clearInterval(interval) } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Scenario 03: Read file
export const readFile: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me check that file for you.",
      (t) => emitDelta(t, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId, name: "read_file", input: "src/components/Button.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId, name: "read_file", output: "export function Button() { ... }", timestamp: Date.now() })
          cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })

          typeText("I've read the Button component. It's a simple functional component that renders a button with variant props. The types look correct.",
            (t) => emitDelta(t, cb),
            () => emitDone(cb)
          )
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 04: Edit file
export const editFile: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("I'll update that component for you.",
      (t) => emitDelta(t, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId, name: "edit_file", input: "src/components/Input.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId, name: "edit_file", output: "Updated props interface", timestamp: Date.now() })
          cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })

          typeText("Done! I've updated the Input component with the new props interface. The changes include better TypeScript support and improved accessibility attributes.",
            (t) => emitDelta(t, cb),
            () => emitDone(cb)
          )
        }, 3000))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 05: Multiple tools (sequential)
export const multipleTools: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const readId = generateEventId()
  const editId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me read the file first, then make the edits.",
      (t) => emitDelta(t, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId: readId, name: "read_file", input: "src/utils/helpers.ts", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId: readId, name: "read_file", timestamp: Date.now() })

          timers.push(setTimeout(() => {
            cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId: editId, name: "edit_file", input: "src/utils/helpers.ts", timestamp: Date.now() })

            timers.push(setTimeout(() => {
              cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId: editId, name: "edit_file", timestamp: Date.now() })
              cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })

              typeText("I've read the helpers file and updated the utility functions. Added proper error handling and improved the type definitions.",
                (t) => emitDelta(t, cb),
                () => emitDone(cb)
              )
            }, 2500))
          }, 500))
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 06: Parallel tools (sequential internally)
export const parallelTools: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const read1Id = generateEventId()
  const read2Id = generateEventId()
  const editId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("I'll check both files and update them.",
      (t) => emitDelta(t, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId: read1Id, name: "read_file", input: "src/components/Header.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId: read1Id, name: "read_file", timestamp: Date.now() })

          timers.push(setTimeout(() => {
            cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId: read2Id, name: "read_file", input: "src/components/Footer.tsx", timestamp: Date.now() })

            timers.push(setTimeout(() => {
              cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId: read2Id, name: "read_file", timestamp: Date.now() })

              timers.push(setTimeout(() => {
                cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId: editId, name: "edit_file", input: "src/components/Header.tsx", timestamp: Date.now() })

                timers.push(setTimeout(() => {
                  cb.onEvent({ type: "tool.completed", id: generateEventId(), toolCallId: editId, name: "edit_file", timestamp: Date.now() })
                  cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })

                  typeText("I've reviewed both files and updated the Header component. The Footer looks good as is.",
                    (t) => emitDelta(t, cb),
                    () => emitDone(cb)
                  )
                }, 2000))
              }, 500))
            }, 1500))
          }, 500))
        }, 1500))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 07: Tool failure
export const toolFailure: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me try to read that file...",
      (t) => emitDelta(t, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), toolCallId, name: "read_file", input: "src/missing.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.failed", id: generateEventId(), toolCallId, name: "read_file", error: "File not found: src/missing.tsx", timestamp: Date.now() })
          cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })

          typeText("I couldn't find the file. It may have been moved or deleted. Could you check the path and try again?",
            (t) => emitDelta(t, cb),
            () => emitDone(cb)
          )
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 08: Agent failure
export const agentFailure: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)

  timers.push(setTimeout(() => {
    typeText("Let me analyze that for you...",
      (t) => emitDelta(t, cb),
      () => {
        timers.push(setTimeout(() => {
          cb.onEvent({ type: "agent.error", id: generateEventId(), error: "Rate limit exceeded. Please try again in 30 seconds.", timestamp: Date.now() })
        }, 1500))
      }
    )
  }, 500))

  return cleanup
}

// Scenario 09: Cancellation (events stop mid-stream)
export const cancellation: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  let cancelled = false

  const cleanup = () => {
    cancelled = true
    timers.forEach(clearTimeout)
  }

  timers.push(setTimeout(() => {
    if (cancelled) return
    const text = "I'm going to provide a very long response that will take a while to type out completely..."
    let currentIndex = 0
    const chars = text.split("")

    const interval = setInterval(() => {
      if (cancelled) {
        clearInterval(interval)
        return
      }
      if (currentIndex < chars.length) {
        emitDelta(chars.slice(0, currentIndex + 1).join(""), cb)
        currentIndex++
      } else {
        clearInterval(interval)
        emitDone(cb)
      }
    }, 50)

    timers.push({ clearTimeout: () => clearInterval(interval) } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Scenario 10: Huge response
export const hugeResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)

  timers.push(setTimeout(() => {
    const hugeText = `I've completed a comprehensive review of your codebase. Here are my detailed findings:

## Architecture Analysis

The project follows a clean modular structure with clear separation of concerns. The component hierarchy is well-organized, and the state management approach using React Context is appropriate for this scale.

### Strengths:
1. **Type Safety** — TypeScript is used throughout with proper type definitions
2. **Component Design** — Components are small, focused, and reusable
3. **State Management** — Context API provides clean data flow without unnecessary complexity
4. **Styling** — Tailwind CSS ensures consistent design system usage

### Areas for Improvement:

1. **Error Boundaries** — Add React Error Boundaries around critical UI sections
2. **Loading States** — Implement skeleton loaders for better perceived performance
3. **Accessibility** — Add ARIA labels and keyboard navigation support
4. **Testing** — Add unit tests for utility functions and integration tests for components

### Recommendations:

- Consider adding a proper error handling layer
- Implement retry logic for failed API calls
- Add performance monitoring for production
- Consider migrating to React Server Components for better initial load

The codebase is in good shape overall. These improvements would make it production-ready.`

    typeText(hugeText,
      (t) => emitDelta(t, cb),
      () => emitDone(cb)
    )
  }, 500))

  return cleanup
}

// Scenario selector based on user message
export function selectScenario(message: string): Scenario {
  const lower = message.toLowerCase()

  if (lower.includes("fail") || lower.includes("error") || lower.includes("broken")) {
    return toolFailure
  }
  if (lower.includes("crash") || lower.includes("die") || lower.includes("abort")) {
    return agentFailure
  }
  if (lower.includes("cancel") || lower.includes("stop") || lower.includes("quit")) {
    return cancellation
  }
  if (lower.includes("big") || lower.includes("large") || lower.includes("huge") || lower.includes("long")) {
    return hugeResponse
  }
  if (lower.includes("parallel") || lower.includes("concurrent")) {
    return parallelTools
  }
  if (lower.includes("multiple") || lower.includes("many") || lower.includes("both")) {
    return multipleTools
  }
  if (lower.includes("edit") || lower.includes("modify") || lower.includes("change") || lower.includes("update")) {
    return editFile
  }
  if (lower.includes("read") || lower.includes("check") || lower.includes("look") || lower.includes("view")) {
    return readFile
  }
  if (lower.includes("stream") || lower.includes("slow") || lower.includes("type")) {
    return streamingResponse
  }

  return simpleResponse
}
