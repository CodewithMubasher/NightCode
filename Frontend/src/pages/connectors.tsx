import { useState, useEffect } from "react"
import { Search, Plus, Ellipsis, Plug, Terminal, Power, PowerOff } from "lucide-react"
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
import { fetchConnectors, createConnector, deleteConnector, toggleConnector, type Connector } from "@/lib/backend-runtime"

function formatDate(ts: string): string {
  return new Date(ts).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
}

export function Connectors() {
  const [connectors, setConnectors] = useState<Connector[]>([])
  const [loading, setLoading] = useState(true)
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [command, setCommand] = useState("")

  useEffect(() => {
    fetchConnectors().then((cs) => {
      setConnectors(cs)
      setLoading(false)
    })
  }, [])

  const handleCreate = async () => {
    if (!name.trim() || !command.trim()) return
    const id = await createConnector(name.trim(), command.trim())
    if (id) {
      const cs = await fetchConnectors()
      setConnectors(cs)
      setName("")
      setCommand("")
      setOpen(false)
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await deleteConnector(id)
    if (ok) {
      const cs = await fetchConnectors()
      setConnectors(cs)
    }
  }

  const handleToggle = async (id: string, enabled: boolean) => {
    const ok = await toggleConnector(id, enabled)
    if (ok) {
      setConnectors((prev) =>
        prev.map((c) => (c.id === id ? { ...c, enabled } : c))
      )
    }
  }

  return (
    <div className="flex flex-col h-full pt-5 px-14">
      <div className="flex items-center justify-between py-5 shrink-0">
        <div>
          <h1 className="text-xl font-semibold text-white/90">Connectors</h1>
          <p className="text-sm text-white/50 mt-0.5">Connect MCP servers to extend AI capabilities</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-white/40" />
            <input
              type="text"
              placeholder="Search connectors..."
              className="w-56 pl-9 pr-3 py-1.5 rounded-full bg-white/5 border border-white/10 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
            />
          </div>
          <Button variant="default" className="rounded-full gap-2" onClick={() => setOpen(true)}>
            <Plus className="size-4" />
            <span>New connector</span>
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-auto pb-6">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <div className="size-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : connectors.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center">
              <div className="size-12 rounded-xl bg-white/5 flex items-center justify-center mx-auto mb-3">
                <Plug className="size-5 text-white/30" />
              </div>
              <p className="text-sm text-white/40">No connectors yet</p>
              <p className="text-xs text-white/25 mt-1">Add an MCP server to get started</p>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-4">
            {connectors.map((c) => (
              <div
                key={c.id}
                className="relative flex flex-col p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="size-9 rounded-xl bg-white/5 flex items-center justify-center">
                      <Plug className="size-4 text-white/50" />
                    </div>
                    <div>
                      <h3 className="text-sm font-medium text-white/90">{c.name}</h3>
                      <p className="text-xs text-white/40 mt-0.5 font-mono">{c.command}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleToggle(c.id, !c.enabled) }}
                      className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                        c.enabled
                          ? "bg-green-500/10 text-green-400 hover:bg-green-500/20"
                          : "bg-white/5 text-white/30 hover:bg-white/10"
                      }`}
                      title={c.enabled ? "Disable" : "Enable"}
                    >
                      {c.enabled ? <Power className="size-3.5" /> : <PowerOff className="size-3.5" />}
                    </button>
                    <DropdownMenu
                      trigger={
                        <button className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer">
                          <Ellipsis className="size-4 text-white/50" />
                        </button>
                      }
                      align="right"
                    >
                      <DropdownMenuItem variant="destructive" onClick={(e) => { e.stopPropagation(); handleDelete(c.id) }}>
                        <span>Delete</span>
                      </DropdownMenuItem>
                    </DropdownMenu>
                  </div>
                </div>
                {c.args && c.args !== "[]" && (
                  <p className="text-xs text-white/30 mt-2 font-mono truncate">{c.args}</p>
                )}
                <div className="flex items-center gap-2 mt-auto pt-3">
                  <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${
                    c.enabled ? "bg-green-500/10 text-green-400" : "bg-white/5 text-white/30"
                  }`}>
                    {c.enabled ? "Active" : "Inactive"}
                  </span>
                  <span className="text-[11px] text-white/30">{formatDate(c.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md bg-neutral-900 border-white/10">
          <DialogHeader>
            <DialogTitle className="text-white/90">Add MCP Connector</DialogTitle>
          </DialogHeader>
          <div className="py-2">
            <div className="mb-4">
              <label className="text-sm text-white/60 mb-2 block">Name</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Win Control"
                className="bg-white/5 border-white/10 text-white/80 placeholder:text-white/30 h-9 px-3"
              />
            </div>
            <div>
              <label className="text-sm text-white/60 mb-2 block">Command</label>
              <Input
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder='python "C:\path\to\server.py"'
                className="bg-white/5 border-white/10 text-white/80 placeholder:text-white/30 h-9 px-3 font-mono"
              />
            </div>
          </div>
          <DialogFooter className="gap-2">
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate}>
              Add connector
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
