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

const sampleResponses = [
  "I've updated the input component with the new props and fixed the type definitions.",
  "Done! I've made the necessary changes to improve the code structure.",
  "All set. The modifications have been applied successfully.",
  "Finished! I've refactored the component as requested.",
]

interface ToolEvent {
  id: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: string
  output?: string
  duration?: number
  timestamp: number
}

export function ChatView({ chatId }: ChatViewProps) {
  const { getChat, addMessage } = useChats()
  const chat = getChat(chatId)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const [isAgentStarted, setIsAgentStarted] = useState(false)
  const [isAgentCompleted, setIsAgentCompleted] = useState(false)
  const [toolEvents, setToolEvents] = useState<ToolEvent[]>([])
  const toolEventsRef = useRef<ToolEvent[]>([])
  const [typingText, setTypingText] = useState("")
  const [isTyping, setIsTyping] = useState(false)
  const cleanupRef = useRef<(() => void) | null>(null)
  const hasTriggeredRef = useRef(false)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [chat?.messages, isTyping, typingText, toolEvents])

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
    setIsAgentStarted(true)
    setIsAgentCompleted(false)
    setToolEvents([])
    toolEventsRef.current = []
    setIsTyping(false)
    setTypingText("")

    const cleanup = emitFakeRuntime(
      (event) => {
        if (event.type === "agent.started") {
          setIsAgentStarted(true)
        } else if (event.type === "tool.started" || event.type === "tool.completed") {
          setToolEvents((prev) => {
            const next = [...prev, event as ToolEvent]
            toolEventsRef.current = next
            return next
          })
        } else if (event.type === "agent.completed") {
          setIsAgentCompleted(true)
        }
      },
      () => {
        const response = sampleResponses[Math.floor(Math.random() * sampleResponses.length)]

        // Give the timeline a beat to auto-collapse to its "Done" summary,
        // then start typing the response in as content of the SAME turn
        // (no separate bubble, no second icon, nothing unmounts).
        setTimeout(() => {
          setIsTyping(true)

          let currentIndex = 0
          const chars = response.split("")
          const typeInterval = setInterval(() => {
            if (currentIndex < chars.length) {
              setTypingText(chars.slice(0, currentIndex + 1).join(""))
              currentIndex++
            } else {
              clearInterval(typeInterval)
              addMessage(chatId, {
                role: "assistant",
                content: response,
                toolEvents: toolEventsRef.current,
              })
              // Reset the live-turn state; the message list now owns this
              // response, and isAgentStarted resets on the next send.
              setIsTyping(false)
              setTypingText("")
              setIsAgentStarted(false)
            }
          }, 20)
        }, 700)
      }
    )

    cleanupRef.current = cleanup
  }, [chatId, addMessage])

  const handleSend = useCallback((message: string) => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    hasTriggeredRef.current = true
    addMessage(chatId, { role: "user", content: message })

    startRuntime()
  }, [chatId, addMessage, startRuntime])

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto pt-4 pb-0">
        <div className="mx-auto max-w-3xl space-y-4 px-4">
          {chat?.messages.length === 0 && !isAgentStarted && (
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
                <div className="flex-shrink-0 mt-1">
                  <Eclipse className="size-6 text-primary" />
                </div>
              )}
              <div className={`max-w-[80%] ${msg.role === "assistant" ? "flex-1 min-w-0" : ""}`}>
                {msg.role === "assistant" && msg.toolEvents && msg.toolEvents.length > 0 && (
                  <ToolTimeline
                    isAgentStarted={true}
                    isAgentCompleted={true}
                    toolEvents={msg.toolEvents}
                    startCollapsed
                  />
                )}
                <div
                  className={`rounded-2xl px-4 py-2 text-sm ${
                    msg.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-white/5 text-white"
                  }`}
                >
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
              </div>
            </div>
          ))}

          {isAgentStarted && (
            <div className="flex justify-start gap-2">
              <div className="flex-shrink-0 mt-1">
                <Eclipse className="size-6 text-primary" />
              </div>
              <div className="max-w-[80%] flex-1 min-w-0">
                <ToolTimeline
                  isAgentStarted={isAgentStarted}
                  isAgentCompleted={isAgentCompleted}
                  toolEvents={toolEvents}
                />
                {isTyping && (
                  <div className="rounded-2xl px-4 py-2 text-sm text-white mt-1">
                    <p className="whitespace-pre-wrap">
                      {typingText}
                      <span className="inline-block w-0.5 h-4 bg-white/70 ml-0.5 animate-pulse" />
                    </p>
                  </div>
                )}
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
