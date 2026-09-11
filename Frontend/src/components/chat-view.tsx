import { useEffect, useRef, useState, useCallback } from "react"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import { ToolTimeline } from "@/components/tool-timeline"
import { Eclipse } from "lucide-react"
import { emitFakeRuntime } from "@/lib/runtime"
import type { RuntimeEvent, ToolStartedEvent, ToolCompletedEvent } from "@/types/events"

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
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: string
  output?: string
  duration?: number
  timestamp: number
}

type Phase = "idle" | "typing-initial" | "showing-timeline" | "typing-summary"

export function ChatView({ chatId }: ChatViewProps) {
  const { getChat, addMessage } = useChats()
  const chat = getChat(chatId)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [initialText, setInitialText] = useState("")
  const [summaryText, setSummaryText] = useState("")

  const eventsRef = useRef<RuntimeEvent[]>([])
  const initialTextRef = useRef("")
  const cleanupRef = useRef<(() => void) | null>(null)
  const hasTriggeredRef = useRef(false)

  const toolEvents: ToolEvent[] = events
    .filter((e): e is ToolStartedEvent | ToolCompletedEvent =>
      e.type === "tool.started" || e.type === "tool.completed"
    )
    .map((e) => ({
      id: e.id,
      type: e.type,
      name: e.name,
      input: e.type === "tool.started" ? e.input : undefined,
      output: e.type === "tool.completed" ? e.output : undefined,
      timestamp: e.timestamp,
    }))

  const hasInitialText = events.some((e) => e.type === "assistant.delta" && e.text !== "__DONE__")
  const hasToolEvents = toolEvents.length > 0
  const hasAgentCompleted = events.some((e) => e.type === "agent.completed")
  const hasSummaryDone = events.some((e) => e.type === "assistant.delta" && e.text === "__DONE__" && events.indexOf(e) > events.findIndex((ev) => ev.type === "agent.completed"))

  let phase: Phase = "idle"
  if (hasInitialText && !hasToolEvents && !hasAgentCompleted) {
    phase = "typing-initial"
  } else if (hasToolEvents && !hasSummaryDone) {
    phase = "showing-timeline"
  } else if (hasAgentCompleted && !hasSummaryDone) {
    phase = "typing-summary"
  } else if (hasSummaryDone) {
    phase = "idle"
  }

  const isLive = phase !== "idle"

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [chat?.messages, phase, initialText, summaryText, toolEvents])

  useEffect(() => {
    if (!chat || hasTriggeredRef.current) return
    const lastMsg = chat.messages[chat.messages.length - 1]
    if (!lastMsg || lastMsg.role !== "user") return

    const hasAssistantResponse = chat.messages.some((m) => m.role === "assistant")
    if (hasAssistantResponse) return

    hasTriggeredRef.current = true
    startRuntime()
  }, [chat])

  const startRuntime = useCallback(() => {
    setEvents([])
    eventsRef.current = []
    setInitialText("")
    initialTextRef.current = ""
    setSummaryText("")

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
              ...toolEventsRef.current.map((te) => ({ type: "tool-call" as const, toolCallId: te.id, name: te.name, input: te.input })),
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

        if (event.type === "tool.started" || event.type === "tool.completed") {
          toolEventsRef.current = [
            ...toolEventsRef.current,
            {
              id: event.id,
              type: event.type,
              name: event.name,
              input: event.type === "tool.started" ? event.input : undefined,
              output: event.type === "tool.completed" ? event.output : undefined,
              timestamp: event.timestamp,
            },
          ]
        }
      },
    })

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

    startRuntime()
  }, [chatId, addMessage, startRuntime])

  const isTypingInitial = phase === "typing-initial"
  const showTimeline = phase === "showing-timeline" || phase === "typing-summary"
  const isTypingSummary = phase === "typing-summary"

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
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start gap-2"}`}
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
                              type: "tool.started" as const,
                              name: part.name,
                              input: part.input,
                              timestamp: 0,
                            })
                          } else if (part.type === "tool-result") {
                            const existing = acc.find((e) => e.id === part.toolCallId)
                            if (existing) {
                              existing.type = "tool.completed"
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
                      isAgentCompleted={phase === "typing-summary"}
                      toolEvents={toolEvents}
                    />
                  )}

                  {(isTypingSummary || summaryText) && (
                    <p className="text-white/90 whitespace-pre-wrap leading-6">
                      {summaryText}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>
      </div>
      <PromptInput onSend={handleSend} isInChat />
    </div>
  )
}
