import type { ToolCallEntry } from "@/types/message"
import { FileText, FilePen, Terminal, Search, Globe, Wrench, type LucideIcon } from "lucide-react"

const hideScrollbarStyle = `
  .hide-scrollbar::-webkit-scrollbar { display: none; }
  .hide-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }
`

export interface ToolRenderer {
  icon: LucideIcon
  label: string
  getLabel?: (entry: ToolCallEntry) => string
  getFileName?: (entry: ToolCallEntry) => string | undefined
  renderDetail?: (entry: ToolCallEntry) => React.ReactNode
}

function extractFileName(input: unknown): string | undefined {
  if (typeof input === "string") {
    const parts = input.split("/")
    return parts[parts.length - 1] || input
  }
  if (input && typeof input === "object" && "path" in input) {
    const p = (input as { path: string }).path
    if (typeof p === "string") {
      const parts = p.split("/")
      return parts[parts.length - 1] || p
    }
  }
  return undefined
}

const toolRenderers: Record<string, ToolRenderer> = {
  read_file: {
    icon: FileText,
    label: "Reading file",
    getFileName: (entry) => extractFileName(entry.input),
  },
  edit_file: {
    icon: FilePen,
    label: "Edit file",
    getFileName: (entry) => extractFileName(entry.input),
  },
  shell: {
    icon: Terminal,
    label: "Run command",
    getLabel: (entry) => entry.status === "running" ? "Running command" : "Ran command",
    renderDetail: (entry) => {
      const cmd = typeof entry.input === "string"
        ? entry.input
        : entry.input && typeof entry.input === "object" && "command" in entry.input
          ? String((entry.input as { command: string }).command)
          : undefined
      const output = entry.output
      if (!cmd) return null
      return (
        <div className="mt-2 text-xs space-y-2">
          <div>
            <div className="text-white/40 mb-1 font-mono text-[10px]">bash</div>
            <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/70 font-mono text-[11px] leading-4">
              {cmd}
            </pre>
          </div>
          {typeof output === "string" && output && (
            <div>
              <div className="text-white/40 mb-1 font-mono text-[10px]">Output</div>
              <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/60 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
                {output}
              </pre>
            </div>
          )}
        </div>
      )
    },
  },
  search: {
    icon: Search,
    label: "Search",
    getFileName: (entry) => {
      if (typeof entry.input === "string") return entry.input
      if (entry.input && typeof entry.input === "object" && "query" in entry.input) {
        return String((entry.input as { query: string }).query)
      }
      return undefined
    },
  },
  fetch: {
    icon: Globe,
    label: "Fetch URL",
    getFileName: (entry) => {
      if (typeof entry.input === "string") return entry.input
      if (entry.input && typeof entry.input === "object" && "url" in entry.input) {
        return String((entry.input as { url: string }).url)
      }
      return undefined
    },
  },
}

const fallbackRenderer: ToolRenderer = {
  icon: Wrench,
  label: "Tool",
  getFileName: (entry) => {
    if (typeof entry.input === "string") return entry.input
    if (entry.input && typeof entry.input === "object") {
      const keys = Object.keys(entry.input)
      if (keys.length === 1) return String((entry.input as Record<string, unknown>)[keys[0]])
    }
    return undefined
  },
  renderDetail: (entry) => {
    if (!entry.input && !entry.output) return null
    return (
      <div className="mt-2 text-xs">
        {entry.input && (
          <pre className="rounded-lg bg-white/5 border border-white/10 p-3 overflow-x-auto hide-scrollbar text-white/70 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
            {typeof entry.input === "string" ? entry.input : JSON.stringify(entry.input, null, 2)}
          </pre>
        )}
        {entry.output && (
          <pre className="mt-1 rounded-lg bg-white/5 border border-white/10 p-3 overflow-x-auto hide-scrollbar text-white/60 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
            {typeof entry.output === "string" ? entry.output : JSON.stringify(entry.output, null, 2)}
          </pre>
        )}
      </div>
    )
  },
}

export function getToolRenderer(toolName: string): ToolRenderer {
  return toolRenderers[toolName] ?? fallbackRenderer
}

export function computeToolSummary(calls: ToolCallEntry[]): string {
  const failedCount = calls.filter((c) => c.status === "failed").length
  const successCount = calls.length - failedCount
  const allFailed = failedCount > 0 && successCount === 0
  const hasMixed = failedCount > 0 && successCount > 0

  if (allFailed) return `Failed (${failedCount})`
  if (hasMixed) return `${successCount} done, ${failedCount} failed`

  const editCount = calls.filter((c) => c.name === "edit_file").length
  const readCount = calls.filter((c) => c.name === "read_file").length

  if (editCount > 0) return `Edit ${editCount} file${editCount > 1 ? "s" : ""}`
  if (readCount > 0) return `Read ${readCount} file${readCount > 1 ? "s" : ""}`
  return calls.length === 1 ? `${getToolRenderer(calls[0].name).label}` : `${calls.length} tools`
}
