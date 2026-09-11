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

export function ToolTimeline({ isAgentStarted, isAgentCompleted, toolEvents, startCollapsed = true }: ToolTimelineProps) {
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
    <div>
      <button
        onClick={toggleExpanded}
        className="flex items-center gap-1.5 text-white/50 cursor-pointer hover:text-white/70"
      >
        <span className="text-[14px]">{summary}</span>
        <ChevronDown className={`size-3 text-white/50 transition-transform duration-200 ${isExpanded ? "" : "-rotate-90"}`} />
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
                isLast={isLastEvent && !isAgentCompleted}
                showLine={!(isLastEvent && !isAgentCompleted)}
              />
            )
          })}

          {isAgentCompleted && (
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
