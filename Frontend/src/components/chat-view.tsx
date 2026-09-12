import { useEffect, useRef, useState, useCallback, useMemo } from "react"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import { ToolTimeline } from "@/components/tool-timeline"
import { MarkdownRenderer } from "@/components/markdown-renderer"
import { ArtifactPanel } from "@/components/artifact-panel"
import {
  MessageScrollerProvider,
  MessageScroller,
  MessageScrollerViewport,
  MessageScrollerContent,
  MessageScrollerItem,
  MessageScrollerButton,
} from "@/components/ui/message-scroller"
import { Eclipse, Copy, ThumbsUp, ThumbsDown, RotateCcw, FileText } from "lucide-react"
import { emitFakeRuntime } from "@/lib/runtime"
import { emitBackendRuntime, cancelBackendRun } from "@/lib/backend-runtime"
import type { RuntimeEvent, ArtifactCreatedEvent } from "@/types/events"
import type { AttachmentPart, TurnSegment, ToolCallEntry } from "@/types/message"
import { AttachmentCard, isMarkdownFile } from "@/components/attachment-card"

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

interface LiveToolEvent {
  id: string
  toolCallId: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: unknown
  output?: unknown
  error?: string
  duration?: number
  timestamp: number
}

function accumulateSegments(events: RuntimeEvent[]): TurnSegment[] {
  const segmentOrder: string[] = []
  const textSegments = new Map<string, string>()
  const toolSegments = new Map<string, ToolCallEntry[]>()

  for (const event of events) {
    if (event.type === "assistant.delta") {
      if (event.text === "__DONE__") continue
      if (!segmentOrder.includes(event.segmentId)) {
        segmentOrder.push(event.segmentId)
      }
      textSegments.set(event.segmentId, event.text)
    } else if (event.type === "tool.started") {
      if (!segmentOrder.includes(event.segmentId)) {
        segmentOrder.push(event.segmentId)
      }
      if (!toolSegments.has(event.segmentId)) {
        toolSegments.set(event.segmentId, [])
      }
      toolSegments.get(event.segmentId)!.push({
        toolCallId: event.toolCallId,
        name: event.name,
        status: "running",
        input: event.input,
        startedAt: event.timestamp,
      })
    } else if (event.type === "tool.completed") {
      const calls = toolSegments.get(event.segmentId)
      if (calls) {
        const call = calls.find((c) => c.toolCallId === event.toolCallId)
        if (call) {
          call.status = "completed"
          call.output = event.output
          call.completedAt = event.timestamp
        }
      }
    } else if (event.type === "tool.failed") {
      const calls = toolSegments.get(event.segmentId)
      if (calls) {
        const call = calls.find((c) => c.toolCallId === event.toolCallId)
        if (call) {
          call.status = "failed"
          call.error = event.error
          call.completedAt = event.timestamp
        }
      }
    }
  }

  return segmentOrder.map((id) => {
    if (toolSegments.has(id)) {
      return { type: "tool-group", id, calls: toolSegments.get(id)! }
    }
    return { type: "text", id, text: textSegments.get(id) || "" }
  })
}

export function ChatView({ chatId }: ChatViewProps) {
  const { getChat, addMessage, addArtifact, isArtifactPanelOpen, openArtifact, openArtifactPanel, closeArtifactPanel } = useChats()
  const chat = getChat(chatId)

  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [currentText, setCurrentText] = useState("")
  const [errorText, setErrorText] = useState("")

  const eventsRef = useRef<RuntimeEvent[]>([])
  const currentTextRef = useRef("")
  const cleanupRef = useRef<(() => void) | null>(null)
  const hasTriggeredRef = useRef(false)

  const segments = useMemo(
    () => accumulateSegments(events),
    [events]
  )

  const agentError = events.some((e) => e.type === "agent.error")
  const hasDone = events.some((e) => e.type === "assistant.delta" && e.text === "__DONE__")

  let phase: "idle" | "active" | "error" = "idle"
  if (agentError) {
    phase = "error"
  } else if (hasDone) {
    phase = "idle"
  } else if (events.length > 0) {
    phase = "active"
  }

  const isLive = phase !== "idle"

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
    setCurrentText("")
    currentTextRef.current = ""
    setErrorText("")

    const useMock = import.meta.env.VITE_USE_MOCK === "true"

    if (useMock) {
      const cleanup = emitFakeRuntime({
        onEvent: (event) => {
          setEvents((prev) => {
            const next = [...prev, event]
            eventsRef.current = next
            return next
          })

          if (event.type === "assistant.delta") {
            if (event.text === "__DONE__") {
              const snapshot = eventsRef.current
              const accumulated = accumulateSegments(snapshot)

              addMessage(chatId, "assistant", [], accumulated)

              setCurrentText("")
              currentTextRef.current = ""
              setEvents([])
              eventsRef.current = []
            } else {
              setCurrentText(event.text)
              currentTextRef.current = event.text
            }
          }

          if (event.type === "agent.error") {
            setErrorText(event.error)
            addMessage(chatId, "assistant", [
              { type: "text", text: `Error: ${event.error}` },
            ])
            setCurrentText("")
            currentTextRef.current = ""
            setEvents([])
            eventsRef.current = []
          }

          if (event.type === "artifact.created") {
            const artifactEvent = event as ArtifactCreatedEvent
            addArtifact(chatId, {
              type: "artifact",
              id: artifactEvent.artifactId,
              name: artifactEvent.name,
              content: artifactEvent.content,
              artifactType: artifactEvent.artifactType,
              language: artifactEvent.language,
              createdAt: artifactEvent.timestamp,
            })
          }
        },
      }, userMessage ?? "")

      cleanupRef.current = cleanup
    } else {
      const workspaceId = chat?.workspaceId
      const cleanup = emitBackendRuntime({
        onEvent: (event) => {
          setEvents((prev) => {
            const next = [...prev, event]
            eventsRef.current = next
            return next
          })

          if (event.type === "assistant.delta") {
            if (event.text === "__DONE__") {
              const snapshot = eventsRef.current
              const accumulated = accumulateSegments(snapshot)

              addMessage(chatId, "assistant", [], accumulated)

              setCurrentText("")
              currentTextRef.current = ""
              setEvents([])
              eventsRef.current = []
            } else {
              setCurrentText(event.text)
              currentTextRef.current = event.text
            }
          }

          if (event.type === "agent.error") {
            setErrorText(event.error)
            addMessage(chatId, "assistant", [
              { type: "text", text: `Error: ${event.error}` },
            ])
            setCurrentText("")
            currentTextRef.current = ""
            setEvents([])
            eventsRef.current = []
          }

          if (event.type === "artifact.created") {
            const artifactEvent = event as ArtifactCreatedEvent
            addArtifact(chatId, {
              type: "artifact",
              id: artifactEvent.artifactId,
              name: artifactEvent.name,
              content: artifactEvent.content,
              artifactType: artifactEvent.artifactType,
              language: artifactEvent.language,
              createdAt: artifactEvent.timestamp,
            })
          }
        },
      }, userMessage ?? "", chatId, workspaceId)

      cleanupRef.current = cleanup
    }
  }, [chatId, chat?.workspaceId, addMessage, addArtifact])

  const handleSend = useCallback((message: string, attachments: AttachmentPart[] = []) => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    hasTriggeredRef.current = true
    const parts = [
      ...(attachments.length > 0 ? attachments : []),
      ...(message ? [{ type: "text" as const, text: message }] : []),
    ]
    addMessage(chatId, "user", parts)

    startRuntime(message)
  }, [chatId, addMessage, startRuntime])

  const handleCancel = useCallback(() => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }

    // Also cancel on the backend
    const workspaceId = chat?.workspaceId
    cancelBackendRun(chatId, workspaceId)

    if (currentTextRef.current || segments.length > 0) {
      addMessage(chatId, "assistant", [], segments)
    }

    setCurrentText("")
    currentTextRef.current = ""
    setEvents([])
    eventsRef.current = []
  }, [chatId, chat?.workspaceId, addMessage, segments])

  const handleOpenToolArtifact = useCallback((call: ToolCallEntry) => {
    if (call.name === "read_file") return
    const artifactId = `tool-${call.toolCallId}`
    const input = typeof call.input === "string" ? call.input : ""
    const output = typeof call.output === "string" ? call.output : ""
    const name = input.split("/").pop() || call.name

    addArtifact(chatId, {
      type: "artifact",
      id: artifactId,
      name,
      content: output || "(no output)",
      artifactType: call.name === "shell" ? "code" : "code",
      language: call.name === "shell" ? "bash" : name.split(".").pop(),
      createdAt: Date.now(),
    })
    openArtifact(artifactId)
  }, [chatId, addArtifact, openArtifact])

  const handleOpenAttachment = useCallback((name: string, content: string, type: "document" | "code", language?: string) => {
    const artifactId = `att-${name}`
    addArtifact(chatId, {
      type: "artifact",
      id: artifactId,
      name,
      content,
      artifactType: type,
      language,
      createdAt: Date.now(),
    })
    openArtifact(artifactId)
  }, [chatId, addArtifact, openArtifact])

  const toggleArtifactPanel = useCallback(() => {
    if (isArtifactPanelOpen) {
      closeArtifactPanel()
    } else {
      openArtifactPanel()
    }
  }, [isArtifactPanelOpen, openArtifactPanel, closeArtifactPanel])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.repeat || event.metaKey || event.ctrlKey || event.altKey) return
      const tag = (event.target as HTMLElement).tagName
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return
      if (event.key.toLowerCase() === "d") {
        if (isArtifactPanelOpen) {
          closeArtifactPanel()
        } else {
          openArtifactPanel()
        }
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [isArtifactPanelOpen, openArtifactPanel, closeArtifactPanel])

  const isError = phase === "error"
  const isGenerating = phase !== "idle"

  return (
    <div className="flex h-full min-h-0">
      <div className={`flex flex-col flex-1 min-w-0 transition-all duration-300 ease-in-out ${isArtifactPanelOpen ? "" : ""}`}>
        <div className="relative">
          <button
            onClick={toggleArtifactPanel}
            className={`absolute top-2 right-2 z-10 p-1.5 rounded-lg transition-colors cursor-pointer ${isArtifactPanelOpen ? "bg-white/10 text-white/90" : "text-white/60 hover:text-white/80 hover:bg-white/5"}`}
            title="Toggle Artifacts"
          >
            <FileText className="size-4" />
          </button>
        </div>
        <MessageScrollerProvider autoScroll scrollEdgeThreshold={80} scrollPreviousItemPeek={120}>
        <MessageScroller>
          <MessageScrollerViewport preserveScrollOnPrepend className="pt-4 pb-0 scrollbar-hide">
            <MessageScrollerContent className="mx-auto max-w-3xl space-y-4 px-4 gap-4">
              {chat?.messages.length === 0 && !isLive && (
                <MessageScrollerItem messageId="empty-state">
                  <div className="flex items-center justify-center h-64 text-white/40">
                    <p>Start a conversation...</p>
                  </div>
                </MessageScrollerItem>
              )}
              {chat && chat.messages.length > 0 && (
                <MessageScrollerItem messageId="timestamp-divider">
                  <div className="text-center text-xs text-white/40 pb-2">
                    {formatTimestamp(chat.createdAt)}
                  </div>
                </MessageScrollerItem>
              )}

              {chat?.messages.map((msg) => (
                <MessageScrollerItem
                  key={msg.id}
                  messageId={msg.id}
                  scrollAnchor={msg.role === "user"}
                >
                  <div
                    className={`${msg.role === "user" ? "flex justify-end" : "flex justify-start gap-2 group relative"}`}
                  >
                    {msg.role === "assistant" && (
                      <div className="flex-shrink-0">
                        <Eclipse className="size-6 text-primary" />
                      </div>
                    )}
                    <div className={`max-w-[80%] ${msg.role === "assistant" ? "flex-1 min-w-0" : ""}`}>
                      {msg.role === "assistant" ? (
                        <div className="flex flex-col ml-1">
                          {msg.segments && msg.segments.length > 0 && (
                            <>
                              {msg.segments.map((segment) => {
                                if (segment.type === "text" && segment.text) {
                                  return <div key={segment.id} className="mb-1"><MarkdownRenderer content={segment.text} /></div>
                                }
                                if (segment.type === "tool-group" && segment.calls.length > 0) {
                                  const toolEventsForTimeline: LiveToolEvent[] = segment.calls.map((call) => ({
                                    id: call.toolCallId,
                                    toolCallId: call.toolCallId,
                                    type: call.status === "completed" ? "tool.completed" as const : call.status === "failed" ? "tool.failed" as const : "tool.started" as const,
                                    name: call.name,
                                    input: call.input,
                                    output: call.output,
                                    error: call.error,
                                    timestamp: call.startedAt,
                                  }))
                                  return (
                                    <ToolTimeline
                                      key={segment.id}
                                      isAgentStarted={true}
                                      isAgentCompleted={true}
                                      toolEvents={toolEventsForTimeline}
                                      toolCalls={segment.calls}
                                      onOpenArtifact={handleOpenToolArtifact}
                                    />
                                  )
                                }
                                return null
                              })}
                            </>
                          )}
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
                        <div className="flex flex-col items-end gap-1">
                          <div className="flex gap-2 overflow-x-auto snap-x snap-mandatory scrollbar-none">
                            {msg.parts.filter((p): p is AttachmentPart => p.type === "attachment").map((att) => {
                              const getFileLanguage = (n: string) => {
                                const ext = n.split(".").pop()?.toLowerCase() || ""
                                const map: Record<string, string> = { tsx: "tsx", ts: "typescript", jsx: "jsx", js: "javascript", py: "python", md: "md", json: "json", css: "css", html: "html" }
                                return map[ext] || ext
                              }
                              return (
                                <AttachmentCard
                                  key={att.id}
                                  attachment={att}
                                  onClick={att.content ? () => handleOpenAttachment(att.name, att.content!, isMarkdownFile(att.name) ? "document" : "code", getFileLanguage(att.name)) : undefined}
                                />
                              )
                            })}
                          </div>
                          <div className="rounded-2xl px-4 py-2 bg-primary text-primary-foreground">
                            <p className="whitespace-pre-wrap">
                              {msg.parts.filter((p) => p.type === "text").map((p) => p.text).join("")}
                            </p>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </MessageScrollerItem>
              ))}

              {isLive && (
                <MessageScrollerItem messageId="live-streaming">
                  <div className="flex justify-start gap-2 items-start">
                    <div className="flex-shrink-0">
                      <Eclipse className="size-6 text-primary" />
                    </div>
                    <div className="max-w-[80%] flex-1 min-w-0">
                      <div className="flex flex-col ml-1">
                        {segments.map((segment) => {
                          if (segment.type === "text" && segment.text) {
                            return <div key={segment.id} className="mb-1"><MarkdownRenderer content={segment.text} isStreaming /></div>
                          }
                          if (segment.type === "tool-group" && segment.calls.length > 0) {
                            const toolEventsForTimeline: LiveToolEvent[] = segment.calls.map((call) => ({
                              id: call.toolCallId,
                              toolCallId: call.toolCallId,
                              type: call.status === "completed" ? "tool.completed" as const : call.status === "failed" ? "tool.failed" as const : "tool.started" as const,
                              name: call.name,
                              input: call.input,
                              output: call.output,
                              error: call.error,
                              timestamp: call.startedAt,
                            }))
                            const isGroupCompleted = segment.calls.every((c) => c.status === "completed" || c.status === "failed")
                            return (
                              <ToolTimeline
                                key={segment.id}
                                isAgentStarted={true}
                                isAgentCompleted={isGroupCompleted}
                                toolEvents={toolEventsForTimeline}
                                toolCalls={segment.calls}
                                startCollapsed={false}
                                onOpenArtifact={handleOpenToolArtifact}
                              />
                            )
                          }
                          return null
                        })}

                        {isError && errorText && (
                          <p className="text-red-400/80 whitespace-pre-wrap leading-6">
                            {errorText}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                </MessageScrollerItem>
              )}
            </MessageScrollerContent>
          </MessageScrollerViewport>
          <MessageScrollerButton direction="end" />
        </MessageScroller>
      </MessageScrollerProvider>
        <PromptInput
          onSend={handleSend}
          onCancel={handleCancel}
          isInChat
          isGenerating={isGenerating}
        />
      </div>
      <div
        className={`shrink-0 overflow-hidden transition-[width] duration-300 ease-in-out ${isArtifactPanelOpen ? "w-[400px]" : "w-0"}`}
      >
        <div className="w-[400px] h-full">
          <ArtifactPanel chatId={chatId} />
        </div>
      </div>
    </div>
  )
}
