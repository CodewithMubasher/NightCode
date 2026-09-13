import { useState, useEffect } from "react"
import { Search, Plus, Ellipsis } from "lucide-react"
import { useNavigate } from "@tanstack/react-router"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { fetchWorkspaces, createWorkspace, deleteWorkspace } from "@/lib/backend-runtime"

interface Workspace {
  id: string
  name: string
  description: string
  created_at: string
}

function formatDate(ts: string): string {
  return new Date(ts).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
}

export function Workspaces() {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")

  useEffect(() => {
    fetchWorkspaces().then((ws) => {
      setWorkspaces(ws)
      setLoading(false)
    })
  }, [])

  const handleCreate = async () => {
    if (!name.trim()) return
    const id = await createWorkspace(name.trim(), description.trim())
    if (id) {
      // Refresh the list
      const ws = await fetchWorkspaces()
      setWorkspaces(ws)
      setName("")
      setDescription("")
      setOpen(false)
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await deleteWorkspace(id)
    if (ok) {
      const ws = await fetchWorkspaces()
      setWorkspaces(ws)
    }
  }

  return (
    <div className="flex flex-col h-full pt-5 px-14">
      <div className="flex items-center justify-between py-5 shrink-0">
        <div>
          <h1 className="text-xl font-semibold text-white/90">Workspaces</h1>
          <p className="text-sm text-white/50 mt-0.5">Create, organize and manage your projects</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-white/40" />
            <input
              type="text"
              placeholder="Search workspaces..."
              className="w-56 pl-9 pr-3 py-1.5 rounded-full bg-white/5 border border-white/10 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
            />
          </div>
          <Button variant="default" className="rounded-full gap-2" onClick={() => setOpen(true)}>
            <Plus className="size-4" />
            <span>New workspace</span>
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-auto pb-6">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <div className="size-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : workspaces.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center">
              <div className="size-12 rounded-xl bg-white/5 flex items-center justify-center mx-auto mb-3">
                <Plus className="size-5 text-white/30" />
              </div>
              <p className="text-sm text-white/40">No workspaces yet</p>
              <p className="text-xs text-white/25 mt-1">Create one to get started</p>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-4">
            {workspaces.map((ws) => (
              <div
                key={ws.id}
                className="relative flex flex-col p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors cursor-pointer"
                onClick={() => navigate({ to: "/workspaces/$workspaceId", params: { workspaceId: ws.id } })}
              >
                <div className="absolute top-3 right-3">
                  <DropdownMenu
                    trigger={
                      <button className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer">
                        <Ellipsis className="size-4 text-white/50" />
                      </button>
                    }
                    align="right"
                  >
                    <DropdownMenuItem variant="destructive" onClick={(e) => { e.stopPropagation(); handleDelete(ws.id) }}>
                      <span>Delete</span>
                    </DropdownMenuItem>
                  </DropdownMenu>
                </div>
                <h3 className="text-sm font-medium text-white/90 pr-6">{ws.name}</h3>
                {ws.description && (
                  <p className="text-xs text-white/50 mt-1 line-clamp-2">{ws.description}</p>
                )}
                <div className="mt-auto pt-3">
                  <span className="text-[11px] text-white/30">{formatDate(ws.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md bg-neutral-900 border-white/10">
          <DialogHeader>
            <DialogTitle className="text-white/90">Create a Project</DialogTitle>
          </DialogHeader>
          <div className="py-2">
            <div className="mb-5">
              <label className="text-sm text-white/60 mb-2 block">What are you working on?</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Project name"
                className="bg-white/5 border-white/10 text-white/80 placeholder:text-white/30 h-9 px-3"
              />
            </div>
            <div>
              <label className="text-sm text-white/60 mb-2 block">What are you trying to achieve?</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe your goal..."
                rows={3}
                className="w-full rounded-2xl bg-white/5 border border-white/10 px-3 py-2 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors resize-none"
              />
            </div>
          </div>
          <DialogFooter className="gap-2">
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate}>
              Create project
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
