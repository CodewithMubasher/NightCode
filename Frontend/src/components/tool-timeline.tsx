import { useState, useEffect, useRef } from "react"
import { ChevronDown } from "lucide-react"
import { TimelineNode } from "@/components/timeline-node"
import { getToolRenderer, computeToolSummary } from "@/lib/tool-registry"
import type { ToolCallEntry } from "@/types/message"

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

interface ToolTimelineProps {
  isAgentStarted: boolean
  isAgentCompleted: boolean
  toolEvents: LiveToolEvent[]
  toolCalls?: ToolCallEntry[]
  summary?: string
  startCollapsed?: boolean
  onOpenArtifact?: (call: ToolCallEntry) => void
}

function areCallsParallel(calls: ToolCallEntry[]): boolean {
  if (calls.length <= 1) return false
  const sorted = [...calls].sort((a, b) => a.startedAt - b.startedAt)
  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i].startedAt - sorted[i - 1].startedAt < 500) return true
  }
  return false
}

export function ToolTimeline({ isAgentStarted, isAgentCompleted, toolEvents, toolCalls, summary: summaryProp, startCollapsed = true, onOpenArtifact }: ToolTimelineProps) {
  const [isExpanded, setIsExpanded] = useState(!startCollapsed)
  const [expandedDetail, setExpandedDetail] = useState<string | null>(null)
  const userToggledRef = useRef(false)

  useEffect(() => {
    if (!isAgentCompleted || userToggledRef.current) return
    const timer = setTimeout(() => setIsExpanded(false), 600)
    return () => clearTimeout(timer)
  }, [isAgentCompleted])

  const toggleExpanded = () => {
    userToggledRef.current = true
    setIsExpanded((prev) => !prev)
  }

  if (!isAgentStarted) return null

  const displayCalls: ToolCallEntry[] = toolCalls ?? toolEvents.map((e) => ({
    toolCallId: e.toolCallId,
    name: e.name,
    status: e.type === "tool.failed" ? "failed" as const : e.type === "tool.completed" ? "completed" as const : "running" as const,
    input: e.input,
    output: e.output,
    error: e.error,
    startedAt: e.timestamp,
  }))

  if (displayCalls.length === 0) return null

  const hasFailed = displayCalls.some((c) => c.status === "failed")
  const isParallel = areCallsParallel(displayCalls)

  const summary = summaryProp ?? computeToolSummary(displayCalls)

  return (
    <div>
      <style>{`
        @keyframes shimmer {
          0% { background-position: -200% 0; }
          100% { background-position: 200% 0; }
        }
        .tl-shimmer {
          background: linear-gradient(
            90deg,
            rgba(255,255,255,0.5) 0%,
            rgba(255,255,255,0.8) 50%,
            rgba(255,255,255,0.5) 100%
          );
          background-size: 200% 100%;
          -webkit-background-clip: text;
          background-clip: text;
          -webkit-text-fill-color: transparent;
          animation: shimmer 1.5s ease-in-out infinite;
        }
      `}</style>
      <button
        onClick={toggleExpanded}
        className={`flex items-center gap-1.5 -pt-1 cursor-pointer hover:text-white/70 ${hasFailed ? "text-red-400/70" : "text-white/50"}`}
      >
        <span className={`text-[14px] ${!isAgentCompleted && !hasFailed ? "tl-shimmer" : ""}`}>{summary}</span>
        <ChevronDown className={`size-3 ${hasFailed ? "text-red-400/50" : "text-white/50"} transition-transform duration-200 ${isExpanded ? "" : "-rotate-90"}`} />
      </button>

      <div className={`ml-1 grid transition-all duration-200 ease-in-out ${isExpanded ? "grid-rows-[1fr] opacity-100 mt-1 mb-2" : "grid-rows-[0fr] opacity-0"}`}>
        <div className="overflow-hidden pt-0.5 pb-0">
          {displayCalls.map((call, index) => {
            const renderer = getToolRenderer(call.name)
            const isError = call.status === "failed"
            const isLast = index === displayCalls.length - 1
            const fileName = renderer.getFileName?.(call)
            const isDetailExpanded = expandedDetail === call.toolCallId
            const detail = !isError && renderer.renderDetail ? renderer.renderDetail(call) : null

            return (
              <TimelineNode
                key={call.toolCallId}
                iconComponent={renderer.icon}
                iconType={isError ? "error" : call.status === "running" ? "loading" : "default"}
                label={renderer.getLabel?.(call) ?? renderer.label}
                fileName={fileName}
                error={call.error}
                isError={isError}
                isLast={isLast && !isAgentCompleted}
                showLine={!(isLast && !isAgentCompleted)}
                isParallel={isParallel}
                detail={detail}
                isDetailExpanded={isDetailExpanded}
                onToggleDetail={detail ? () => setExpandedDetail(isDetailExpanded ? null : call.toolCallId) : undefined}
                onOpenArtifact={call.status !== "running" && call.name !== "read_file" && onOpenArtifact ? () => onOpenArtifact(call) : undefined}
              />
            )
          })}

          {isAgentCompleted && hasFailed && (
            <TimelineNode
              iconType="check"
              label="Failed"
              isError={true}
              isLast={true}
              showLine={false}
            />
          )}

          {isAgentCompleted && !hasFailed && (
            <TimelineNode
              iconType="check"
              label="Done "
              isLast={true}
              showLine={false}
            />
          )}
        </div>
      </div>
    </div>
  )
}
