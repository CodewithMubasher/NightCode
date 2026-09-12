import { Search, Plus } from "lucide-react"

export function Workspaces() {
  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-6 py-5 shrink-0">
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
              className="w-56 pl-9 pr-3 py-1.5 rounded-lg bg-white/5 border border-white/10 text-sm text-white/80 placeholder:text-white/30 outline-none focus:border-white/20 transition-colors"
            />
          </div>
          <button className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/10 text-sm text-white/80 hover:bg-white/15 transition-colors cursor-pointer">
            <Plus className="size-4" />
            <span>New workspace</span>
          </button>
        </div>
      </div>
      <div className="flex-1 flex items-center justify-center">
        <div className="text-center">
          <div className="size-12 rounded-xl bg-white/5 flex items-center justify-center mx-auto mb-3">
            <Plus className="size-5 text-white/30" />
          </div>
          <p className="text-sm text-white/40">No workspaces yet</p>
          <p className="text-xs text-white/25 mt-1">Create one to get started</p>
        </div>
      </div>
    </div>
  )
}
