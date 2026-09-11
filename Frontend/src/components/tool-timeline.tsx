import { useState, useEffect, useRef } from "react"
import { ChevronDown } from "lucide-react"
import { TimelineNode } from "@/components/timeline-node"

interface ToolEvent {
  id: string
  type: "tool.started" | "tool.completed" | "tool.failed"
  name: string
  input?: string
  output?: string
  duration?: number
  timestamp: number
}

interface ToolTimelineProps {
  isAgentStarted: boolean
  isAgentCompleted: boolean
  toolEvents: ToolEvent[]
  startCollapsed?: boolean
}

export function ToolTimeline({ isAgentStarted, isAgentCompleted, toolEvents, startCollapsed = false }: ToolTimelineProps) {
  const [isExpanded, setIsExpanded] = useState(!startCollapsed)
  const userToggledRef = useRef(false)

  // Auto-collapse the detail list once the agent finishes, unless the
  // user has manually expanded/collapsed it themselves.
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

  const getToolDisplay = (event: ToolEvent) => {
    switch (event.name) {
      case "read_file":
        return { icon: "file-text" as const, label: "Reading file", fileName: event.input }
      case "edit_file":
        return { icon: "file-pen" as const, label: "Edit file", fileName: event.input, linesRemoved: 7, linesAdded: 13 }
      default:
        return { icon: "file-text" as const, label: event.name }
    }
  }

  const editCount = toolEvents.filter((e) => e.name === "edit_file").length
  const readCount = toolEvents.filter((e) => e.name === "read_file").length
  const summary = isAgentCompleted
    ? editCount > 0
      ? `Edit ${editCount} file${editCount > 1 ? "s" : ""}`
      : readCount > 0
        ? `Read ${readCount} file${readCount > 1 ? "s" : ""}`
        : "Done"
    : "Working..."

  return (
    <div className="py-2">
      <button
        onClick={toggleExpanded}
        className="flex items-center gap-1.5 text-white/70 cursor-pointer hover:text-white/90 transition-colors"
      >
        <span className="text-sm">{summary}</span>
        <ChevronDown className={`size-3 text-white/50 transition-transform duration-200 ${isExpanded ? "" : "-rotate-90"}`} />
      </button>

      <div className={`ml-1 mt-2 transition-all duration-300 ease-in-out overflow-hidden ${isExpanded ? "max-h-[400px] opacity-100" : "max-h-0 opacity-0"}`}>
          {toolEvents.map((event) => {
            const display = getToolDisplay(event)

            return (
              <TimelineNode
                key={event.id}
                icon={display.icon}
                label={display.label}
                fileName={display.fileName}
                linesRemoved={display.linesRemoved}
                linesAdded={display.linesAdded}
                isLast={false}
                showLine={true}
              />
            )
          })}

        {isAgentCompleted && (
          <div className="mt-1">
            <TimelineNode
              icon="check"
              label="Done ✓"
              isLast={true}
              showLine={false}
            />
          </div>
        )}
      </div>
    </div>
  )
}
