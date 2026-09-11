import { useEffect, useRef, useState, useCallback } from "react"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import { ToolTimeline } from "@/components/tool-timeline"
import { Eclipse, Copy, ThumbsUp, ThumbsDown, RotateCcw } from "lucide-react"
import { emitFakeRuntime } from "@/lib/runtime"
import type { RuntimeEvent } from "@/types/events"

interface ChatViewProps {
  chatId: string
}

function formatTimestamp(ts: number): string {
  const date = new Date(ts)
  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()

  const time = date.toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  })

  if (isToday) return `Today ${time}`
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" }) + " " + time
}

interface ToolEvent {
  id: string
  toolCallId: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: string
  output?: string
  error?: string
  duration?: number
  timestamp: number
}

type Phase = "idle" | "typing-initial" | "showing-timeline" | "typing-summary" | "error"

export function ChatView({ chatId }: ChatViewProps) {
  const { getChat, addMessage } = useChats()
  const chat = getChat(chatId)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [initialText, setInitialText] = useState("")
  const [summaryText, setSummaryText] = useState("")
  const [errorText, setErrorText] = useState("")

  const eventsRef = useRef<RuntimeEvent[]>([])
  const initialTextRef = useRef("")
  const cleanupRef = useRef<(() => void) | null>(null)
  const hasTriggeredRef = useRef(false)

  const toolEvents: ToolEvent[] = (() => {
    const byCallId = new Map<string, ToolEvent>()
    for (const e of events) {
      if (e.type === "tool.started") {
        byCallId.set(e.toolCallId, {
          id: e.id, toolCallId: e.toolCallId, type: "tool.started",
          name: e.name, input: e.input, timestamp: e.timestamp,
        })
      } else if (e.type === "tool.failed") {
        byCallId.set(e.toolCallId, {
          id: e.id, toolCallId: e.toolCallId, type: "tool.failed",
          name: e.name, error: e.error, timestamp: e.timestamp,
        })
      }
    }
    return Array.from(byCallId.values())
  })()

  const hasInitialText = events.some((e) => e.type === "assistant.delta" && e.text !== "__DONE__")
  const hasToolEvents = toolEvents.length > 0
  const hasAgentCompleted = events.some((e) => e.type === "agent.completed")
  const hasAgentError = events.some((e) => e.type === "agent.error")
  const hasSummaryDone = events.some((e) => e.type === "assistant.delta" && e.text === "__DONE__" && events.indexOf(e) > events.findIndex((ev) => ev.type === "agent.completed"))

  let phase: Phase = "idle"
  if (hasAgentError) {
    phase = "error"
  } else if (hasInitialText && !hasToolEvents && !hasAgentCompleted) {
    phase = "typing-initial"
  } else if (hasToolEvents && !hasSummaryDone) {
    phase = "showing-timeline"
  } else if (hasAgentCompleted && !hasSummaryDone) {
    phase = "typing-summary"
  } else if (hasSummaryDone) {
    phase = "idle"
  }

  const isLive = phase !== "idle"

  const toolSummary = (() => {
    const failedCount = events.filter((e) => e.type === "tool.failed").length
    const readCount = events.filter((e) => e.type === "tool.started" && e.name === "read_file").length
    const editCount = events.filter((e) => e.type === "tool.started" && e.name === "edit_file").length
    const successCount = readCount + editCount
    const allFailed = failedCount > 0 && successCount === 0
    const hasMixed = failedCount > 0 && successCount > 0

    if (allFailed) return `Failed (${failedCount})`
    if (hasMixed) return `${successCount} done, ${failedCount} failed`
    if (!hasAgentCompleted) return "Working..."
    if (editCount > 0) return `Edit ${editCount} file${editCount > 1 ? "s" : ""}`
    if (readCount > 0) return `Read ${readCount} file${readCount > 1 ? "s" : ""}`
    return "Done"
  })()

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [chat?.messages, phase, initialText, summaryText, errorText, toolEvents])

  useEffect(() => {
    if (!chat || hasTriggeredRef.current) return
    const lastMsg = chat.messages[chat.messages.length - 1]
    if (!lastMsg || lastMsg.role !== "user") return

    const hasAssistantResponse = chat.messages.some((m) => m.role === "assistant")
    if (hasAssistantResponse) return

    hasTriggeredRef.current = true
    const lastUserMsg = chat.messages.filter((m) => m.role === "user").pop()
    const userText = lastUserMsg?.parts.find((p) => p.type === "text")?.text ?? ""
    startRuntime(userText)
  }, [chat])

  const startRuntime = useCallback((userMessage?: string) => {
    setEvents([])
    eventsRef.current = []
    setInitialText("")
    initialTextRef.current = ""
    setSummaryText("")
    setErrorText("")

    const cleanup = emitFakeRuntime({
      onEvent: (event) => {
        setEvents((prev) => {
          const next = [...prev, event]
          eventsRef.current = next
          return next
        })

        if (event.type === "assistant.delta") {
          if (event.text === "__DONE__") {
            const summaryParts = summaryTextRef.current
            addMessage(chatId, "assistant", [
              { type: "text", text: initialTextRef.current },
              ...toolEventsRef.current.flatMap((te) => [
                { type: "tool-call" as const, toolCallId: te.toolCallId, name: te.name, input: te.input },
                { type: "tool-result" as const, toolCallId: te.toolCallId, name: te.name, error: te.error },
              ]),
              { type: "text", text: summaryParts },
            ])
            setInitialText("")
            initialTextRef.current = ""
            setSummaryText("")
            setEvents([])
            eventsRef.current = []
          } else {
            const isSummary = eventsRef.current.some((e) => e.type === "agent.completed")
            if (isSummary) {
              setSummaryText(event.text)
              summaryTextRef.current = event.text
            } else {
              setInitialText(event.text)
              initialTextRef.current = event.text
            }
          }
        }

        if (event.type === "tool.started" || event.type === "tool.failed") {
          const existing = toolEventsRef.current.find((e) => e.toolCallId === event.toolCallId)
          if (existing) {
            existing.type = event.type
            existing.error = event.type === "tool.failed" ? event.error : undefined
          } else {
            toolEventsRef.current = [
              ...toolEventsRef.current,
              {
                id: event.id,
                toolCallId: event.toolCallId,
                type: event.type,
                name: event.name,
                input: event.type === "tool.started" ? event.input : undefined,
                error: event.type === "tool.failed" ? event.error : undefined,
                timestamp: event.timestamp,
              },
            ]
          }
        }

        if (event.type === "agent.error") {
          setErrorText(event.error)
          addMessage(chatId, "assistant", [
            { type: "text", text: `Error: ${event.error}` },
          ])
          setInitialText("")
          initialTextRef.current = ""
          setSummaryText("")
          setEvents([])
          eventsRef.current = []
        }
      },
    }, userMessage ?? "")

    cleanupRef.current = cleanup
  }, [chatId, addMessage])

  const summaryTextRef = useRef("")
  const toolEventsRef = useRef<ToolEvent[]>([])

  const handleSend = useCallback((message: string) => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    hasTriggeredRef.current = true
    addMessage(chatId, "user", [{ type: "text", text: message }])

    startRuntime(message)
  }, [chatId, addMessage, startRuntime])

  const handleCancel = useCallback(() => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    const summaryParts = summaryTextRef.current
    if (initialTextRef.current || toolEventsRef.current.length > 0 || summaryParts) {
      addMessage(chatId, "assistant", [
        { type: "text", text: initialTextRef.current },
        ...toolEventsRef.current.map((te) => ({ type: "tool-call" as const, toolCallId: te.id, name: te.name, input: te.input })),
        { type: "text", text: summaryParts || "(cancelled)" },
      ])
    }

    setInitialText("")
    initialTextRef.current = ""
    setSummaryText("")
    setEvents([])
    eventsRef.current = []
    toolEventsRef.current = []
  }, [chatId, addMessage])

  const isTypingInitial = phase === "typing-initial"
  const showTimeline = phase === "showing-timeline" || phase === "typing-summary"
  const isTypingSummary = phase === "typing-summary"
  const isError = phase === "error"
  const isGenerating = phase !== "idle"

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="flex-1 overflow-y-auto min-h-0 pt-4 pb-0 scrollbar-hide">
        <div className="mx-auto max-w-3xl space-y-4 px-4">
          {chat?.messages.length === 0 && !isLive && (
            <div className="flex items-center justify-center h-64 text-white/40">
              <p>Start a conversation...</p>
            </div>
          )}
          {chat && chat.messages.length > 0 && (
            <div className="text-center text-xs text-white/40 pb-2">
              {formatTimestamp(chat.createdAt)}
            </div>
          )}

          {chat?.messages.map((msg) => (
            <div
              key={msg.id}
              className={`${msg.role === "user" ? "flex justify-end" : "flex justify-start gap-2 group relative"}`}
            >
              {msg.role === "assistant" && (
                <div className="flex-shrink-0">
                  <Eclipse className="size-6 text-primary" />
                </div>
              )}
              <div className={`max-w-[80%] ${msg.role === "assistant" ? "flex-1 min-w-0" : ""}`}>
                {msg.role === "assistant" ? (
                  <div className="flex flex-col gap-1 ml-1">
                    {(() => {
                      const textParts = msg.parts.filter((p) => p.type === "text")
                      const hasTools = msg.parts.some((p) => p.type === "tool-call")
                      const toolEventsForTimeline = msg.parts
                        .filter((p) => p.type === "tool-call" || p.type === "tool-result")
                        .reduce((acc, part) => {
                          if (part.type === "tool-call") {
                            acc.push({
                              id: part.toolCallId,
                              toolCallId: part.toolCallId,
                              type: "tool.started" as const,
                              name: part.name,
                              input: part.input,
                              timestamp: 0,
                            })
                          } else if (part.type === "tool-result") {
                            const existing = acc.find((e) => e.toolCallId === part.toolCallId)
                            if (existing) {
                              if (part.error) {
                                existing.type = "tool.failed"
                                existing.error = part.error
                              } else {
                                existing.type = "tool.completed"
                              }
                            }
                          }
                          return acc
                        }, [] as ToolEvent[])

                      return (
                        <>
                          {textParts[0] && (
                            <p className="text-white/80 whitespace-pre-wrap leading-6">
                              {textParts[0].text}
                            </p>
                          )}
                          {hasTools && (
                            <ToolTimeline
                              isAgentStarted={true}
                              isAgentCompleted={true}
                              toolEvents={toolEventsForTimeline}
                              summary={(() => {
                                const failed = toolEventsForTimeline.filter((e) => e.type === "tool.failed").length
                                const success = toolEventsForTimeline.length - failed
                                if (failed > 0 && success === 0) return `Failed (${failed})`
                                if (failed > 0 && success > 0) return `${success} done, ${failed} failed`
                                if (toolEventsForTimeline.some((e) => e.name === "edit_file")) return `Edit ${success} file${success > 1 ? "s" : ""}`
                                return `Read ${success} file${success > 1 ? "s" : ""}`
                              })()}
                            />
                          )}
                          {textParts[1] && (
                            <p className="text-white/80 whitespace-pre-wrap leading-6">
                              {textParts[1].text}
                            </p>
                          )}
                        </>
                      )
                    })()}
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity -ml-1 mt-1">
                      <button className="p-1.5 rounded-md text-white/40 hover:text-white/70 hover:bg-white/5 transition-colors cursor-pointer">
                        <Copy className="size-3.5" />
                      </button>
                      <button className="p-1.5 rounded-md text-white/40 hover:text-white/70 hover:bg-white/5 transition-colors cursor-pointer">
                        <ThumbsUp className="size-3.5" />
                      </button>
                      <button className="p-1.5 rounded-md text-white/40 hover:text-white/70 hover:bg-white/5 transition-colors cursor-pointer">
                        <ThumbsDown className="size-3.5" />
                      </button>
                      <button className="p-1.5 rounded-md text-white/40 hover:text-white/70 hover:bg-white/5 transition-colors cursor-pointer">
                        <RotateCcw className="size-3.5" />
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="rounded-2xl px-4 py-2 bg-primary text-primary-foreground">
                    <p className="whitespace-pre-wrap">
                      {msg.parts.filter((p) => p.type === "text").map((p) => p.text).join("")}
                    </p>
                  </div>
                )}
              </div>
            </div>
          ))}

          {isLive && (
            <div className="flex justify-start gap-2 items-start">
              <div className="flex-shrink-0">
                <Eclipse className="size-6 text-primary" />
              </div>
              <div className="max-w-[80%] flex-1 min-w-0">
                <div className="flex flex-col gap-1 ml-1">
                  {(isTypingInitial || initialText) && (
                    <p className="text-white/90 whitespace-pre-wrap leading-6">
                      {initialText}
                    </p>
                  )}

                  {showTimeline && (
                    <ToolTimeline
                      isAgentStarted={true}
                      isAgentCompleted={hasAgentCompleted}
                      toolEvents={toolEvents}
                      summary={toolSummary}
                    />
                  )}

                  {(isTypingSummary || summaryText) && (
                    <p className="text-white/90 whitespace-pre-wrap leading-6">
                      {summaryText}
                    </p>
                  )}

                  {isError && errorText && (
                    <p className="text-red-400/80 whitespace-pre-wrap leading-6">
                      {errorText}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>
      </div>
      <PromptInput
        onSend={handleSend}
        onCancel={handleCancel}
        isInChat
        isGenerating={isGenerating}
      />
    </div>
  )
}
