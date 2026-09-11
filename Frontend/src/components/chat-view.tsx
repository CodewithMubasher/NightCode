import { useEffect, useRef, useState, useCallback } from "react"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import { ToolTimeline } from "@/components/tool-timeline"
import { Eclipse } from "lucide-react"
import { emitFakeRuntime } from "@/lib/runtime"

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

  const [phase, setPhase] = useState<Phase>("idle")
  const [toolEvents, setToolEvents] = useState<ToolEvent[]>([])
  const [initialText, setInitialText] = useState("")
  const [summaryText, setSummaryText] = useState("")

  const phaseRef = useRef<Phase>("idle")
  const toolEventsRef = useRef<ToolEvent[]>([])
  const initialTextRef = useRef("")
  const cleanupRef = useRef<(() => void) | null>(null)
  const hasTriggeredRef = useRef(false)

  const setPhaseStable = useCallback((p: Phase) => {
    phaseRef.current = p
    setPhase(p)
  }, [])

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
    setPhaseStable("typing-initial")
    setToolEvents([])
    toolEventsRef.current = []
    setInitialText("")
    initialTextRef.current = ""
    setSummaryText("")

    const cleanup = emitFakeRuntime({
      onEvent: (event) => {
        if (event.type === "tool.started" || event.type === "tool.completed") {
          setToolEvents((prev) => {
            const next = [...prev, event as ToolEvent]
            toolEventsRef.current = next
            return next
          })
        }
      },
      onInitialResponse: (text) => {
        typeText(text, (full) => {
          setInitialText(full)
          initialTextRef.current = full
        }, () => {
          setPhaseStable("showing-timeline")
        })
      },
      onSummary: (text) => {
        if (phaseRef.current !== "showing-timeline") return
        setPhaseStable("typing-summary")
        typeText(text, (full) => {
          setSummaryText(full)
        }, () => {
          addMessage(chatId, {
            role: "assistant",
            content: text,
            initialText: initialTextRef.current,
            toolEvents: toolEventsRef.current,
          })
          setPhaseStable("idle")
          setInitialText("")
          initialTextRef.current = ""
          setSummaryText("")
          setToolEvents([])
        })
      },
    })

    cleanupRef.current = cleanup
  }, [chatId, addMessage, setPhaseStable])

  const typeText = useCallback((text: string, onProgress: (text: string) => void, onDone: () => void) => {
    let currentIndex = 0
    const chars = text.split("")
    const typeInterval = setInterval(() => {
      if (currentIndex < chars.length) {
        onProgress(chars.slice(0, currentIndex + 1).join(""))
        currentIndex++
      } else {
        clearInterval(typeInterval)
        onDone()
      }
    }, 20)
  }, [])

  const handleSend = useCallback((message: string) => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    hasTriggeredRef.current = true
    addMessage(chatId, { role: "user", content: message })

    startRuntime()
  }, [chatId, addMessage, startRuntime])

  const isTypingInitial = phase === "typing-initial"
  const showTimeline = phase === "showing-timeline" || phase === "typing-summary"
  const isTypingSummary = phase === "typing-summary"
  const isLive = phase !== "idle"

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
                    {msg.initialText && (
                      <p className="text-white/90 whitespace-pre-wrap leading-6">
                        {msg.initialText}
                      </p>
                    )}
                    {msg.toolEvents && msg.toolEvents.length > 0 && (
                      <ToolTimeline
                        isAgentStarted={true}
                        isAgentCompleted={true}
                        toolEvents={msg.toolEvents}
                      />
                    )}
                    <p className="text-white/90 whitespace-pre-wrap leading-6">
                      {msg.content}
                    </p>
                  </div>
                ) : (
                  <div className="rounded-2xl px-4 py-2 bg-primary text-primary-foreground">
                    <p className="whitespace-pre-wrap">{msg.content}</p>
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
