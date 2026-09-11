import { FileText, FilePen, CircleCheck, Loader2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"

interface TimelineNodeProps {
  icon: "file-text" | "file-pen" | "check" | "loading"
  label: string
  fileName?: string
  linesRemoved?: number
  linesAdded?: number
  error?: string
  isError?: boolean
  isLast?: boolean
  showLine?: boolean
}

const iconMap = {
  "file-text": FileText,
  "file-pen": FilePen,
  check: CircleCheck,
  loading: Loader2,
}

export function TimelineNode({ icon, label, fileName, linesRemoved, linesAdded, error, isError = false, isLast = false, showLine = true }: TimelineNodeProps) {
  const Icon = iconMap[icon]

  return (
    <div className="flex flex-col">
      <div className="flex items-center gap-2">
        <div className={`flex-shrink-0 ${icon === "loading" ? "animate-spin" : ""}`}>
          <Icon className={`size-4 ${isError ? "text-red-400" : "text-white/60"}`} />
        </div>
        <Badge
          variant="secondary"
          className={`text-xs gap-1.5 py-1 px-2.5 ${
            isError
              ? "bg-red-500/10 text-red-400 border-red-500/20"
              : "bg-white/10 text-white/80 border-white/10"
          }`}
        >
          {error && <span className="font-medium">{error}</span>}
          {!error && <span className="font-medium">{label}</span>}
          {!error && fileName && (
            <span className="text-white/50 text-[10px]">{fileName}</span>
          )}
          {linesRemoved !== undefined && linesAdded !== undefined && (
            <span className="text-[10px] font-mono">
              <span className="text-red-400">-{linesRemoved}</span>
              {" "}
              <span className="text-green-400">+{linesAdded}</span>
            </span>
          )}
        </Badge>
      </div>
      {showLine && !isLast && (
        <div className={`w-px h-5 ${isError ? "bg-red-500/20" : "bg-white/20"} ml-[7px]`} />
      )}
    </div>
  )
}
