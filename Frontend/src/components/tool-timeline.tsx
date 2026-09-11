import { useState, useEffect, useRef } from "react"
import { ChevronDown } from "lucide-react"
import { TimelineNode } from "@/components/timeline-node"

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

interface ToolTimelineProps {
  isAgentStarted: boolean
  isAgentCompleted: boolean
  toolEvents: ToolEvent[]
  summary?: string
  startCollapsed?: boolean
}

export function ToolTimeline({ isAgentStarted, isAgentCompleted, toolEvents, summary: summaryProp, startCollapsed = true }: ToolTimelineProps) {
  const [isExpanded, setIsExpanded] = useState(!startCollapsed)
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

  const hasFailed = toolEvents.some((e) => e.type === "tool.failed")

  const getToolDisplay = (event: ToolEvent) => {
    const isError = event.type === "tool.failed"
    switch (event.name) {
      case "read_file":
        return { icon: "file-text" as const, label: "Reading file", fileName: event.input, isError, error: event.error }
      case "edit_file":
        return { icon: "file-pen" as const, label: "Edit file", fileName: event.input, linesRemoved: 7, linesAdded: 13, isError, error: event.error }
      default:
        return { icon: "file-text" as const, label: event.name, isError, error: event.error }
    }
  }

  const failedCount = toolEvents.filter((e) => e.type === "tool.failed").length
  const successCount = toolEvents.length - failedCount
  const allFailed = failedCount > 0 && successCount === 0
  const hasMixed = failedCount > 0 && successCount > 0

  const summary = summaryProp ?? (allFailed
    ? `Failed (${failedCount})`
    : hasMixed
      ? `${successCount} done, ${failedCount} failed`
      : "Working...")

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
        className={`flex items-center gap-1.5 cursor-pointer hover:text-white/70 ${hasFailed ? "text-red-400/70" : "text-white/50"}`}
      >
        <span className={`text-[14px] ${!isAgentCompleted && !hasFailed ? "tl-shimmer" : ""}`}>{summary}</span>
        <ChevronDown className={`size-3 ${hasFailed ? "text-red-400/50" : "text-white/50"} transition-transform duration-200 ${isExpanded ? "" : "-rotate-90"}`} />
      </button>

      <div className={`ml-1 grid transition-all duration-200 ease-in-out ${isExpanded ? "grid-rows-[1fr] opacity-100 mt-2 mb-3" : "grid-rows-[0fr] opacity-0"}`}>
        <div className="overflow-hidden">
          {toolEvents.map((event, index) => {
            const display = getToolDisplay(event)
            const isLastEvent = index === toolEvents.length - 1

            return (
              <TimelineNode
                key={event.id}
                icon={display.icon}
                label={display.label}
                fileName={display.fileName}
                linesRemoved={display.linesRemoved}
                linesAdded={display.linesAdded}
                error={display.error}
                isError={display.isError}
                isLast={isLastEvent && !isAgentCompleted}
                showLine={!(isLastEvent && !isAgentCompleted)}
              />
            )
          })}

          {isAgentCompleted && hasFailed && (
            <TimelineNode
              icon="check"
              label="Failed"
              isError={true}
              isLast={true}
              showLine={false}
            />
          )}

          {isAgentCompleted && !hasFailed && (
            <TimelineNode
              icon="check"
              label="Done ✓"
              isLast={true}
              showLine={false}
            />
          )}
        </div>
      </div>
    </div>
  )
}
