import { FileText, FilePen, CircleCheck, Loader2, AlertCircle, ChevronRight, type LucideIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"

interface TimelineNodeProps {
  icon?: "file-text" | "file-pen" | "check" | "loading"
  iconComponent?: LucideIcon
  iconType?: "default" | "loading" | "error" | "check"
  label: string
  fileName?: string
  error?: string
  isError?: boolean
  isLast?: boolean
  showLine?: boolean
  isParallel?: boolean
  detail?: React.ReactNode
  isDetailExpanded?: boolean
  onToggleDetail?: () => void
  onOpenArtifact?: () => void
}

const legacyIconMap = {
  "file-text": FileText,
  "file-pen": FilePen,
  check: CircleCheck,
  loading: Loader2,
}

export function TimelineNode({
  icon,
  iconComponent,
  iconType = "default",
  label,
  fileName,
  error,
  isError = false,
  isLast = false,
  showLine = true,
  isParallel = false,
  detail,
  isDetailExpanded = false,
  onToggleDetail,
  onOpenArtifact,
}: TimelineNodeProps) {
  const Icon = iconComponent ?? legacyIconMap[icon ?? "file-text"] ?? FileText
  const showCheck = iconType === "check" || icon === "check"
  const showError = iconType === "error" || isError
  const hasDetail = !!detail
  const lineColor = showError ? "bg-red-500/20" : "bg-white/20"

  return (
    <div className="flex">
      <div className="flex flex-col items-center w-4 mr-2 mt-2 -mb-2">
        <div className="flex-shrink-0">
          {showCheck ? (
            <CircleCheck className={`size-4 ${showError ? "text-red-400" : "text-white/60"}`} />
          ) : showError ? (
            <AlertCircle className="size-4 text-red-400" />
          ) : iconType === "loading" || icon === "loading" ? (
            <Loader2 className="size-4 text-white/60 animate-spin" />
          ) : (
            <Icon className="size-4 text-white/60" />
          )}
        </div>
        <div className={`flex-1 w-px ${showLine && !isLast ? lineColor : "bg-transparent"} ${isParallel ? "min-h-1" : "min-h-5"}`} />
      </div>

      <div className="flex-1 min-w-0 mb-2 mt-1.5">
        <div
          className={`flex items-center gap-1.5 ${onOpenArtifact || hasDetail ? "cursor-pointer hover:opacity-80" : ""}`}
          onClick={onOpenArtifact ?? (hasDetail ? onToggleDetail : undefined)}
        >
          <Badge
            variant="secondary"
            className={`text-xs gap-1.5 py-1 px-2.5 ${
              showError
                ? "bg-red-500/10 text-red-400 border-red-500/20"
                : "bg-white/10 text-white/80 border-white/10"
            }`}
          >
            {error && <span className="font-medium">{error}</span>}
            {!error && <span className="font-medium">{label}</span>}
            {!error && fileName && (
              <span className="text-white/50 text-[10px]">{fileName}</span>
            )}
          </Badge>
          {hasDetail && (
            <ChevronRight
              className={`size-3 text-white/30 transition-transform duration-150 ${isDetailExpanded ? "rotate-90" : ""}`}
            />
          )}
        </div>
        {hasDetail && (
          <div className={`grid transition-all duration-200 ease-in-out ${isDetailExpanded ? "grid-rows-[1fr] opacity-100 mt-1" : "grid-rows-[0fr] opacity-0"}`}>
            <div className="overflow-hidden">
              <style>{`
                .hide-scrollbar::-webkit-scrollbar { display: none; }
                .hide-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }
              `}</style>
              {detail}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
