import type { ToolCallEntry } from "@/types/message"
import { FileText, FilePen, FilePlus2, Terminal, Search, Globe, Wrench, Folder, GitBranch, GitCommitHorizontal, Plug, type LucideIcon } from "lucide-react"

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
  write_file: {
    icon: FilePlus2,
    label: "Create file",
    getLabel: (entry) => {
      if (entry.status === "running") return "Creating file"
      // Backend reports newFile:false when the file already existed and was
      // overwritten rather than newly created — reflect that distinction.
      if (entry.output && typeof entry.output === "object" && "newFile" in entry.output) {
        return (entry.output as { newFile: boolean }).newFile ? "Created file" : "Updated file"
      }
      return "Created file"
    },
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
      if (!cmd) return null

      // Backend returns a structured { stdout, stderr, exitCode, timedOut }
      // object, not a bare string — pull the parts out for display.
      const outputObj = entry.output && typeof entry.output === "object"
        ? (entry.output as { stdout?: string; stderr?: string; exitCode?: number; timedOut?: boolean })
        : undefined
      const stdout = outputObj?.stdout ?? ""
      const stderr = outputObj?.stderr ?? ""
      const exitCode = outputObj?.exitCode
      const timedOut = outputObj?.timedOut

      return (
        <div className="mt-2 text-xs space-y-2">
          <div>
            <div className="text-white/40 mb-1 font-mono text-[10px]">bash</div>
            <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/70 font-mono text-[11px] leading-4">
              {cmd}
            </pre>
          </div>
          {timedOut && (
            <div className="text-red-400/80 font-mono text-[10px]">Command timed out</div>
          )}
          {stdout && (
            <div>
              <div className="text-white/40 mb-1 font-mono text-[10px]">Output</div>
              <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/60 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
                {stdout}
              </pre>
            </div>
          )}
          {stderr && (
            <div>
              <div className="text-white/40 mb-1 font-mono text-[10px]">Stderr{typeof exitCode === "number" && exitCode !== 0 ? ` (exit ${exitCode})` : ""}</div>
              <pre className="rounded-lg bg-red-500/5 border border-red-500/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-red-300/70 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
                {stderr}
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
  list_dir: {
    icon: Folder,
    label: "List directory",
    getLabel: (entry) => entry.status === "running" ? "Listing directory" : "Listed directory",
    getFileName: (entry) => extractFileName(entry.input) || ".",
    renderDetail: (entry) => {
      const path = typeof entry.input === "string"
        ? entry.input
        : entry.input && typeof entry.input === "object" && "path" in entry.input
          ? String((entry.input as { path: string }).path) || "."
          : "."
      const output = entry.output && typeof entry.output === "object"
        ? (entry.output as { entries?: { name: string; isDir: boolean; size: number }[] })
        : undefined
      const entries = output?.entries
      return (
        <div className="mt-2 text-xs space-y-2">
          <div>
            <div className="text-white/40 mb-1 font-mono text-[10px]">path</div>
            <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/70 font-mono text-[11px] leading-4">
              {path}
            </pre>
          </div>
          {entries && (
            <div>
              <div className="text-white/40 mb-1 font-mono text-[10px]">Output</div>
              {entries.length === 0 ? (
                <pre className="rounded-lg bg-white/5 border border-white/10 p-3 text-white/40 font-mono text-[11px] leading-4">
                  (empty directory)
                </pre>
              ) : (
                <pre className="rounded-lg bg-white/5 border border-white/10 p-3 whitespace-pre-wrap break-all hide-scrollbar text-white/60 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
                  {entries.map((e) => `${e.isDir ? "d " : "- "}${e.name}${e.isDir ? "/" : `  (${e.size}b)`}`).join("\n")}
                </pre>
              )}
            </div>
          )}
        </div>
      )
    },
  },
  grep: {
    icon: Search,
    label: "Search",
    getLabel: (entry) => entry.status === "running" ? "Searching" : "Searched",
    getFileName: (entry) => {
      if (entry.input && typeof entry.input === "object" && "pattern" in entry.input) {
        return String((entry.input as { pattern: string }).pattern)
      }
      return undefined
    },
    renderDetail: (entry) => {
      const output = entry.output && typeof entry.output === "object"
        ? (entry.output as { matches?: { file: string; line_number: number; line_content: string }[] })
        : undefined
      const matches = output?.matches
      if (!matches || matches.length === 0) return null
      return (
        <div className="mt-2 text-xs">
          <pre className="rounded-lg bg-white/5 border border-white/10 p-3 overflow-x-auto hide-scrollbar text-white/70 font-mono text-[11px] leading-4 max-h-48 overflow-y-auto">
            {matches.map((m) => `${m.file}:${m.line_number}: ${m.line_content}`).join("\n")}
          </pre>
        </div>
      )
    },
  },
  glob: {
    icon: Search,
    label: "Find files",
    getLabel: (entry) => entry.status === "running" ? "Finding files" : "Found files",
    getFileName: (entry) => {
      if (entry.input && typeof entry.input === "object" && "pattern" in entry.input) {
        return String((entry.input as { pattern: string }).pattern)
      }
      return undefined
    },
  },
  git_status: {
    icon: GitBranch,
    label: "Git status",
    getLabel: (entry) => entry.status === "running" ? "Checking git status" : "Checked git status",
  },
  git_diff: {
    icon: GitCommitHorizontal,
    label: "Git diff",
    getLabel: (entry) => entry.status === "running" ? "Reading git diff" : "Read git diff",
    getFileName: (entry) => extractFileName(entry.input),
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
  // MCP tools use prefix "mcp_{connectorID}_{toolName}"
  if (toolName.startsWith("mcp_")) {
    // Extract the actual tool name (after the second underscore)
    const parts = toolName.split("_")
    const actualToolName = parts.length >= 3 ? parts.slice(2).join("_") : toolName
    return {
      icon: Plug,
      label: actualToolName,
      getLabel: (entry) => {
        if (entry.status === "running") return `Running ${actualToolName}`
        if (entry.status === "completed") return mcpVerb(actualToolName)
        return actualToolName
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
  }
  return toolRenderers[toolName] ?? fallbackRenderer
}

// mcpVerb maps MCP tool names to user-friendly headings.
function mcpVerb(toolName: string): string {
  const map: Record<string, string> = {
    open_app: "Opened app",
    open_url: "Opened URL",
    open_file: "Opened file",
    type_text: "Typed text",
    scroll: "Scrolled",
    screenshot: "Took screenshot",
    create_file: "Created file",
    create_folder: "Created folder",
    run_command: "Ran command",
    google_search: "Searched Google",
    youtube_search: "Searched YouTube",
    lock_screen: "Locked screen",
  }
  return map[toolName] ?? `Ran ${toolName}`
}

// summaryPhraseForGroup returns the short phrase for a single tool name given
// how many times it was called in this group, e.g. "Ran 3 commands",
// "Created 1 file", "List directory". Falls back to the tool's registered
// label (pluralized with a count) for tools without bespoke phrasing.
function summaryPhraseForGroup(name: string, count: number): string {
  switch (name) {
    case "write_file":
      return `Created ${count} file${count > 1 ? "s" : ""}`
    case "edit_file":
      return `Edit ${count} file${count > 1 ? "s" : ""}`
    case "read_file":
      return `Read ${count} file${count > 1 ? "s" : ""}`
    case "shell":
      return count === 1 ? "Ran command" : `Ran ${count} commands`
    case "list_dir":
      return count === 1 ? "List directory" : `List directory (${count})`
    case "grep":
      return count === 1 ? "Search" : `Search (${count})`
    case "glob":
      return count === 1 ? "Find files" : `Find files (${count})`
    case "git_status":
      return "Git status"
    case "git_diff":
      return count === 1 ? "Git diff" : `Git diff (${count})`
    default: {
      // MCP tools: extract the actual tool name for display
      if (name.startsWith("mcp_")) {
        const parts = name.split("_")
        const actualName = parts.length >= 3 ? parts.slice(2).join("_") : name
        const verb = mcpVerb(actualName)
        return count > 1 ? `${verb} (${count})` : verb
      }
      const label = getToolRenderer(name).label
      return count > 1 ? `${label} (${count})` : label
    }
  }
}

export function computeToolSummary(calls: ToolCallEntry[]): string {
  const failedCount = calls.filter((c) => c.status === "failed").length
  const successCount = calls.length - failedCount
  const allFailed = failedCount > 0 && successCount === 0
  const hasMixed = failedCount > 0 && successCount > 0

  if (allFailed) return `Failed (${failedCount})`
  if (hasMixed) return `${successCount} done, ${failedCount} failed`

  // Count calls per tool name, preserving first-occurrence order so the
  // heading reads in the same order the tools actually ran (e.g. "Ran 2
  // commands, List directory" rather than an arbitrary/alphabetical order).
  const order: string[] = []
  const counts = new Map<string, number>()
  for (const c of calls) {
    if (!counts.has(c.name)) order.push(c.name)
    counts.set(c.name, (counts.get(c.name) ?? 0) + 1)
  }

  const phrases = order.map((name) => summaryPhraseForGroup(name, counts.get(name)!))

  // Cap at 3 distinct phrases so a long mixed batch doesn't produce an
  // unreadable heading — collapse the rest into "+N more".
  const maxPhrases = 3
  if (phrases.length > maxPhrases) {
    const shown = phrases.slice(0, maxPhrases)
    const remaining = phrases.length - maxPhrases
    return `${shown.join(", ")} +${remaining} more`
  }

  return phrases.join(", ")
}
