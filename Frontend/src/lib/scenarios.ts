import { type RuntimeEvent, generateEventId } from "@/types/events"

export type ScenarioCallbacks = {
  onEvent: (event: RuntimeEvent) => void
}

export type Scenario = (callbacks: ScenarioCallbacks) => () => void

function typeText(
  text: string,
  segmentId: string,
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
  }, 1)
  return () => { cancelled = true; clearInterval(typeInterval) }
}

function emitDelta(text: string, segmentId: string, cb: ScenarioCallbacks): RuntimeEvent {
  const event: RuntimeEvent = {
    type: "assistant.delta",
    id: generateEventId(),
    segmentId,
    text,
    timestamp: Date.now(),
  }
  cb.onEvent(event)
  return event
}

function emitDone(segmentId: string, cb: ScenarioCallbacks): RuntimeEvent {
  const event: RuntimeEvent = {
    type: "assistant.delta",
    id: generateEventId(),
    segmentId,
    text: "__DONE__",
    timestamp: Date.now(),
  }
  cb.onEvent(event)
  return event
}

// Case 1: Plain text only, no tools
export const simpleResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const segId = generateEventId()

  timers.push(setTimeout(() => {
    const text = "Sure! Here's a quick answer: the component is working correctly. No changes needed."
    const cancelType = typeText(text, segId,
      (t) => emitDelta(t, segId, cb),
      () => {
        cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
        emitDone(segId, cb)
      }
    )
    timers.push({ clearTimeout: cancelType } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Streaming response (text in small chunks)
export const streamingResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const segId = generateEventId()

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
        emitDelta(fullText, segId, cb)
        i++
      } else {
        clearInterval(interval)
        cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
        emitDone(segId, cb)
      }
    }, 300)

    timers.push({ clearTimeout: () => clearInterval(interval) } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Case 2: Text → one tool call → text
export const readFile: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSegId = generateEventId()
  const toolSegId = generateEventId()
  const summarySegId = generateEventId()
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me check that file for you.", textSegId,
      (t) => emitDelta(t, textSegId, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", input: "src/components/Button.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", output: "export function Button() {\n  return <button>Click</button>\n}", timestamp: Date.now() })

          typeText("I've read the Button component. It's a simple functional component that renders a button with variant props. The types look correct.", summarySegId,
            (t) => emitDelta(t, summarySegId, cb),
            () => {
              cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
              emitDone(summarySegId, cb)
            }
          )
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Case 2b: Text → one edit → text
export const editFile: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSegId = generateEventId()
  const toolSegId = generateEventId()
  const summarySegId = generateEventId()
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("I'll update that component for you.", textSegId,
      (t) => emitDelta(t, textSegId, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "edit_file", input: "src/components/Input.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "edit_file", output: "Updated props interface", timestamp: Date.now() })

          typeText("Done! I've updated the Input component with the new props interface.", summarySegId,
            (t) => emitDelta(t, summarySegId, cb),
            () => {
              cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
              emitDone(summarySegId, cb)
            }
          )
        }, 3000))
      }
    )
  }, 500))

  return cleanup
}

// Case 3: Text → tool → text → tool → text (interleaved)
export const interleavedTools: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSeg1 = generateEventId()
  const toolSeg1 = generateEventId()
  const textSeg2 = generateEventId()
  const toolSeg2 = generateEventId()
  const textSeg3 = generateEventId()
  const readId = generateEventId()
  const editId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me read the file first.", textSeg1,
      (t) => emitDelta(t, textSeg1, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg1, toolCallId: readId, name: "read_file", input: "src/utils/helpers.ts", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg1, toolCallId: readId, name: "read_file", output: "export function helpers() { ... }", timestamp: Date.now() })

          typeText("Now I see the issue. Let me fix it.", textSeg2,
            (t) => emitDelta(t, textSeg2, cb),
            () => {
              cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId, name: "edit_file", input: "src/utils/helpers.ts", timestamp: Date.now() })

              timers.push(setTimeout(() => {
                cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId, name: "edit_file", output: "Fixed error handling", timestamp: Date.now() })

                typeText("All done. I've read the helpers file and fixed the error handling.", textSeg3,
                  (t) => emitDelta(t, textSeg3, cb),
                  () => {
                    cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
                    emitDone(textSeg3, cb)
                  }
                )
              }, 2500))
            }
          )
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Case 4: Multiple tool calls fired in parallel within one group
export const parallelTools: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSegId = generateEventId()
  const toolSegId = generateEventId()
  const summarySegId = generateEventId()
  const read1Id = generateEventId()
  const read2Id = generateEventId()

  timers.push(setTimeout(() => {
    typeText("I'll check both files at once.", textSegId,
      (t) => emitDelta(t, textSegId, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId: read1Id, name: "read_file", input: "src/components/Header.tsx", timestamp: Date.now() })
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId: read2Id, name: "read_file", input: "src/components/Footer.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSegId, toolCallId: read1Id, name: "read_file", output: "export function Header() { ... }", timestamp: Date.now() })

          timers.push(setTimeout(() => {
            cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSegId, toolCallId: read2Id, name: "read_file", output: "export function Footer() { ... }", timestamp: Date.now() })

            typeText("I've reviewed both components. They look good.", summarySegId,
              (t) => emitDelta(t, summarySegId, cb),
              () => {
                cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
                emitDone(summarySegId, cb)
              }
            )
          }, 1500))
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Case 5: Tool failure followed by recovery text
export const toolFailure: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSeg1 = generateEventId()
  const toolSegId = generateEventId()
  const textSeg2 = generateEventId()
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me try to read that file...", textSeg1,
      (t) => emitDelta(t, textSeg1, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", input: "src/missing.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.failed", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", error: "File not found: src/missing.tsx", timestamp: Date.now() })

          typeText("I couldn't find the file. It may have been moved or deleted. Could you check the path and try again?", textSeg2,
            (t) => emitDelta(t, textSeg2, cb),
            () => {
              cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
              emitDone(textSeg2, cb)
            }
          )
        }, 2000))
      }
    )
  }, 500))

  return cleanup
}

// Case 6: Tool calls only, zero text
export const toolOnly: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const toolSegId = generateEventId()
  const toolCallId = generateEventId()

  timers.push(setTimeout(() => {
    cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", input: "src/config.ts", timestamp: Date.now() })

    timers.push(setTimeout(() => {
      cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSegId, toolCallId, name: "read_file", output: "export const config = { ... }", timestamp: Date.now() })
      cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
      cb.onEvent({ type: "assistant.delta", id: generateEventId(), segmentId: generateEventId(), text: "__DONE__", timestamp: Date.now() })
    }, 2000))
  }, 500))

  return cleanup
}

// Agent failure
export const agentFailure: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const segId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me analyze that for you...", segId,
      (t) => emitDelta(t, segId, cb),
      () => {
        timers.push(setTimeout(() => {
          cb.onEvent({ type: "agent.error", id: generateEventId(), error: "Rate limit exceeded. Please try again in 30 seconds.", timestamp: Date.now() })
        }, 1500))
      }
    )
  }, 500))

  return cleanup
}

// Cancellation
export const cancellation: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  let cancelled = false
  const segId = generateEventId()

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
        emitDelta(chars.slice(0, currentIndex + 1).join(""), segId, cb)
        currentIndex++
      } else {
        clearInterval(interval)
        emitDone(segId, cb)
      }
    }, 50)

    timers.push({ clearTimeout: () => clearInterval(interval) } as unknown as ReturnType<typeof setTimeout>)
  }, 500))

  return cleanup
}

// Huge response
export const hugeResponse: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const segId = generateEventId()

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

    typeText(hugeText, segId,
      (t) => emitDelta(t, segId, cb),
      () => {
        cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
        emitDone(segId, cb)
      }
    )
  }, 500))

  return cleanup
}

// Full workflow: text → read file → text (describe) → 3 edit files → text
export const fullWorkflow: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSeg1 = generateEventId()
  const toolSeg1 = generateEventId()
  const textSeg2 = generateEventId()
  const toolSeg2 = generateEventId()
  const textSeg3 = generateEventId()
  const readId = generateEventId()
  const editId1 = generateEventId()
  const editId2 = generateEventId()
  const editId3 = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me start by reading the component file to understand its current structure.", textSeg1,
      (t) => emitDelta(t, textSeg1, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg1, toolCallId: readId, name: "read_file", input: "src/components/AuthForm.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg1, toolCallId: readId, name: "read_file", output: "export function AuthForm({ onSubmit }: Props) { const [email, setEmail] = useState(\"\"); return <form>...</form> }", timestamp: Date.now() })

          typeText("I can see the AuthForm component. It has a basic email/password form with useState hooks, but it's missing validation, error handling, and a loading state. Let me add all three.", textSeg2,
            (t) => emitDelta(t, textSeg2, cb),
            () => {
              cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId1, name: "edit_file", input: "src/components/AuthForm.tsx", timestamp: Date.now() })

              timers.push(setTimeout(() => {
                cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId1, name: "edit_file", output: "Added email validation regex and password length check", timestamp: Date.now() })

                cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId2, name: "edit_file", input: "src/components/AuthForm.tsx", timestamp: Date.now() })

                timers.push(setTimeout(() => {
                  cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId2, name: "edit_file", output: "Wrapped onSubmit in try/catch with error state display", timestamp: Date.now() })

                  cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId3, name: "edit_file", input: "src/components/AuthForm.tsx", timestamp: Date.now() })

                  timers.push(setTimeout(() => {
                    cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg2, toolCallId: editId3, name: "edit_file", output: "Added isLoading prop and disabled submit button during request", timestamp: Date.now() })

                    typeText("All three changes are in. The form now validates email format, enforces an 8-character minimum password, shows inline error messages, and disables the button while a request is in flight.", textSeg3,
                      (t) => emitDelta(t, textSeg3, cb),
                      () => {
                        cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
                        emitDone(textSeg3, cb)
                      }
                    )
                  }, 2000))
                }, 2000))
              }, 2000))
            }
          )
        }, 2500))
      }
    )
  }, 500))

  return cleanup
}

// Ran command: text → shell command → text
export const ranCommand: Scenario = (cb) => {
  const timers: ReturnType<typeof setTimeout>[] = []
  const cleanup = () => timers.forEach(clearTimeout)
  const textSeg1 = generateEventId()
  const toolSeg = generateEventId()
  const textSeg2 = generateEventId()
  const shellId = generateEventId()

  timers.push(setTimeout(() => {
    typeText("Let me set up the project structure and copy the files into place.", textSeg1,
      (t) => emitDelta(t, textSeg1, cb),
      () => {
        cb.onEvent({ type: "tool.started", id: generateEventId(), segmentId: toolSeg, toolCallId: shellId, name: "shell", input: "mkdir -p /home/claude/fix && cp /mnt/user-data/uploads/timeline-node.tsx /home/claude/fix/timeline-node.tsx && cp /mnt/user-data/uploads/tool-timeline.tsx /home/claude/fix/tool-timeline.tsx", timestamp: Date.now() })

        timers.push(setTimeout(() => {
          cb.onEvent({ type: "tool.completed", id: generateEventId(), segmentId: toolSeg, toolCallId: shellId, name: "shell", output: "exit code 0", timestamp: Date.now() })

          typeText("Done. Both files have been copied to the working directory.", textSeg2,
            (t) => emitDelta(t, textSeg2, cb),
            () => {
              cb.onEvent({ type: "agent.completed", id: generateEventId(), timestamp: Date.now() })
              emitDone(textSeg2, cb)
            }
          )
        }, 3000))
      }
    )
  }, 500))

  return cleanup
}

// Scenario selector based on user message
export function selectScenario(message: string): Scenario {
  const lower = message.toLowerCase()

  if (lower.includes("run") || lower.includes("command") || lower.includes("shell") || lower.includes("exec")) {
    return ranCommand
  }
  if (lower.includes("full") || lower.includes("workflow")) {
    return fullWorkflow
  }
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
  if (lower.includes("interleave") || lower.includes("alternating") || lower.includes("both")) {
    return interleavedTools
  }
  if (lower.includes("parallel") || lower.includes("concurrent")) {
    return parallelTools
  }
  if (lower.includes("only") || lower.includes("no text") || lower.includes("silent")) {
    return toolOnly
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
