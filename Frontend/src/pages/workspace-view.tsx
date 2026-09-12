import { useMemo } from "react"
import { useNavigate } from "@tanstack/react-router"
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"

interface Workspace {
  id: string
  name: string
  description: string
  createdAt: number
}

function loadWorkspaces(): Workspace[] {
  try {
    const raw = localStorage.getItem("nightcode-workspaces")
    if (!raw) return []
    return JSON.parse(raw) as Workspace[]
  } catch {
    return []
  }
}

export function WorkspaceView({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate()
  const workspaces = useMemo(() => loadWorkspaces(), [])
  const workspace = useMemo(() => workspaces.find((w) => w.id === workspaceId), [workspaces, workspaceId])

  return (
    <div className="flex flex-col h-full pt-8 px-14">
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
  )
}
