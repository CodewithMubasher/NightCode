import { useMemo, useCallback, useState, useRef, useEffect } from "react"
import { useNavigate } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import { formatFileSize } from "@/components/attachment-card"
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  Attachment,
  AttachmentContent,
  AttachmentTitle,
  AttachmentDescription,
  AttachmentBadge,
} from "@/components/ui/attachment"
import { Search, Plus, X, GitBranch } from "lucide-react"
import type { AttachmentPart } from "@/types/message"
import { fetchWorkspaces, fetchArtifacts } from "@/lib/backend-runtime"

interface Workspace {
  id: string
  name: string
  description: string
  created_at: string
}

interface ContextFile {
  id: string
  name: string
  size: number
}

interface GitData {
  remoteUrl: string
  totalCommits: number
  recentCommits: { hash: string; message: string; date: string }[]
}

function loadGitData(workspaceId: string): GitData {
  try {
    const raw = localStorage.getItem(`nightcode-git-${workspaceId}`)
    if (raw) return JSON.parse(raw) as GitData
  } catch {}
  return {
    remoteUrl: "",
    totalCommits: 0,
    recentCommits: [],
  }
}

function saveGitData(workspaceId: string, data: GitData) {
  try {
    localStorage.setItem(`nightcode-git-${workspaceId}`, JSON.stringify(data))
  } catch {}
}

function loadContextFiles(workspaceId: string): ContextFile[] {
  try {
    const raw = localStorage.getItem(`nightcode-context-${workspaceId}`)
    if (!raw) return []
    return JSON.parse(raw) as ContextFile[]
  } catch {
    return []
  }
}

function saveContextFiles(workspaceId: string, files: ContextFile[]) {
  try {
    localStorage.setItem(`nightcode-context-${workspaceId}`, JSON.stringify(files))
  } catch {}
}

function getFileExtensionBadge(name: string): string {
  return name.split(".").pop()?.toUpperCase() ?? "FILE"
}

const ACCEPTED_EXTENSIONS = new Set([".ts", ".tsx", ".js", ".jsx", ".py", ".json", ".css", ".html", ".md", ".yaml", ".yml", ".xml", ".sql", ".sh", ".rb", ".go", ".rs", ".java", ".c", ".cpp", ".cs", ".vue", ".svelte", ".txt"])

function isAcceptedFile(file: File): boolean {
  const ext = "." + file.name.split(".").pop()?.toLowerCase()
  return ACCEPTED_EXTENSIONS.has(ext)
}

export function WorkspaceView({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate()
  const { chats, createChat, addMessage } = useChats()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loadingWorkspaces, setLoadingWorkspaces] = useState(true)

  useEffect(() => {
    fetchWorkspaces().then((ws) => {
      setWorkspaces(ws)
      setLoadingWorkspaces(false)
    })
  }, [])

  const workspace = useMemo(() => workspaces.find((w) => w.id === workspaceId), [workspaces, workspaceId])

  const [contextFiles, setContextFiles] = useState<ContextFile[]>(() => loadContextFiles(workspaceId))
  const [gitData, setGitData] = useState<GitData>(() => loadGitData(workspaceId))
  const fileInputRef = useRef<HTMLInputElement>(null)

  const workspaceChats = useMemo(
    () => chats.filter((c) => c.workspaceId === workspaceId),
    [chats, workspaceId]
  )

  const handleSend = useCallback((message: string, attachments: AttachmentPart[]) => {
    const chatId = createChat(message, workspaceId)
    const parts = [
      ...(attachments.length > 0 ? attachments : []),
      ...(message ? [{ type: "text" as const, text: message }] : []),
    ]
    addMessage(chatId, "user", parts)
    navigate({ to: "/chat/$chatId", params: { chatId } })
  }, [createChat, addMessage, navigate, workspaceId])

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files) return
    const newFiles: ContextFile[] = []
    for (const file of Array.from(files)) {
      if (!isAcceptedFile(file)) continue
      newFiles.push({
        id: crypto.randomUUID(),
        name: file.name,
        size: file.size,
      })
    }
    if (newFiles.length > 0) {
      setContextFiles((prev) => {
        const next = [...prev, ...newFiles]
        saveContextFiles(workspaceId, next)
        return next
      })
    }
    e.target.value = ""
  }, [workspaceId])

  const formatDate = (ts: string) => {
    return new Date(ts).toLocaleDateString("en-US", { month: "short", day: "numeric" })
  }

  if (loadingWorkspaces) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="size-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full overflow-auto scrollbar-hide">
      <div className="px-14 pt-8 shrink-0">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink
                render={<button className="cursor-pointer" />}
                onClick={() => navigate({ to: "/workspaces" })}
              >
                Workspaces
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className="text-white/70">
                {workspace?.name ?? "Untitled"}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* Two columns */}
      <div className="flex items-start gap-10 mt-40 px-14">
        {/* Left column */}
        <div className="flex-1 min-w-0 flex flex-col">
          <PromptInput onSend={handleSend} heading={`Ready to work on ${workspace?.name ?? "Untitled"}`} compact />

          {/* Recents */}
          <div className="mt-8">
            <h3 className="text-xs font-medium text-white/40 mb-3">Recents</h3>
            {workspaceChats.length === 0 ? (
              <p className="text-xs text-white/30">No chats yet in this workspace</p>
            ) : (
              <div className="space-y-1">
                {workspaceChats.map((chat) => (
                  <Attachment
                    key={chat.id}
                    orientation="horizontal"
                    className="w-full min-w-0 min-h-0 cursor-pointer bg-transparent hover:bg-white/2 transition-colors"
                    onClick={() => navigate({ to: "/chat/$chatId", params: { chatId: chat.id } })}
                  >
                    <AttachmentContent className="px-3 py-0.2 flex items-center justify-between w-full">
                      <AttachmentTitle className="truncate text-sm text-white/90">
                        {chat.title}
                      </AttachmentTitle>
                      <AttachmentDescription className="shrink-0 ml-3">
                        {formatDate(String(chat.createdAt))}
                      </AttachmentDescription>
                    </AttachmentContent>
                  </Attachment>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right sidebar */}
        <div className="w-[320px] -mt-3 flex flex-col gap-3">
          {/* Git & Github card */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-medium text-white/90">Git & Github</h3>
              <GitBranch className="size-4 text-white/40" />
            </div>

            <div className="space-y-3">
              <div>
                <label className="text-xs text-white/40 mb-1 block">Git remote url</label>
                <input
                  type="url"
                  placeholder="https://github.com/user/repo.git"
                  value={gitData.remoteUrl}
                  onChange={(e) => {
                    const next = { ...gitData, remoteUrl: e.target.value }
                    setGitData(next)
                    saveGitData(workspaceId, next)
                  }}
                  className="w-full px-3 py-1.5 rounded-lg border border-white/10 bg-white/5 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
                />
              </div>

              {gitData.remoteUrl && (
                <>
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-white/40">Total commits</span>
                    <span className="text-white/70 font-medium">{gitData.totalCommits}</span>
                  </div>

                  {gitData.recentCommits.length > 0 && (
                    <div className="space-y-1.5">
                      <span className="text-xs text-white/40">Recent commits</span>
                      {gitData.recentCommits.map((commit) => (
                        <div key={commit.hash} className="flex items-start gap-2">
                          <span className="text-xs text-[#a3004c] font-mono shrink-0">{commit.hash}</span>
                          <span className="text-xs text-white/60 truncate">{commit.message}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          </div>

          {/* Instructions card */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-white/90">Instructions</h3>
              <button className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer">
                <Plus className="size-4 text-white/50" />
              </button>
            </div>
            <p className="text-xs text-white/40">Add instructions to tailor NightCode's responses</p>
          </div>

          {/* Context section */}
          <div className="rounded-2xl border border-white/10 bg-white/5 p-4 flex flex-col">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-medium text-white/90">Context</h3>
              <div className="flex items-center gap-1">
                <button className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer">
                  <Search className="size-4 text-white/50" />
                </button>
                <button
                  className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <Plus className="size-4 text-white/50" />
                </button>
              </div>
            </div>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={[...ACCEPTED_EXTENSIONS].join(",")}
              onChange={handleFileUpload}
              className="hidden"
            />
            {contextFiles.length === 0 ? (
              <p className="text-xs text-white/30">No files added yet</p>
            ) : (
              <div className="grid grid-cols-2 gap-2">
                {contextFiles.map((file) => (
                  <Attachment
                    key={file.id}
                    orientation="vertical"
                    className="w-full cursor-pointer bg-transparent hover:bg-white/5 transition-colors"
                  >
                    <AttachmentContent className="px-2 py-1.5 relative">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setContextFiles((prev) => {
                            const next = prev.filter((f) => f.id !== file.id)
                            saveContextFiles(workspaceId, next)
                            return next
                          })
                        }}
                        className="absolute top-1 right-1 p-0.5 rounded-md opacity-0 group-hover/attachment:opacity-100 hover:bg-white/10 transition-all cursor-pointer"
                      >
                        <X className="size-3 text-white/50" />
                      </button>
                      <AttachmentTitle className="whitespace-normal line-clamp-2 text-white/90">
                        {file.name}
                      </AttachmentTitle>
                      <AttachmentDescription className="text-white/50">
                        {formatFileSize(file.size)}
                      </AttachmentDescription>
                      <AttachmentBadge className="bg-white/5 text-white/50 border-white/10">{getFileExtensionBadge(file.name)}</AttachmentBadge>
                    </AttachmentContent>
                  </Attachment>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
