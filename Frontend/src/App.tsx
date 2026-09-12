import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarFooter,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuAction,
} from "@/components/ui/sidebar"
import { Eclipse, Settings, CirclePlus, FolderOpen, ClockFading, Scroll, Settings2, Ellipsis, ChevronDown, Check, Pin, PinOff, Trash2 } from "lucide-react"
import {
  Avatar,
  AvatarFallback,
} from "@/components/ui/avatar"
import { useState, useRef, useEffect, useMemo, createContext, useContext } from "react"
import { Outlet, useNavigate, useRouterState } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu"

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

const SelectedWorkspaceContext = createContext<string>("")

export function useSelectedWorkspace() {
  return useContext(SelectedWorkspaceContext)
}

export function App() {
  const [workspaceOpen, setWorkspaceOpen] = useState(false)
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string>("")
  const workspaceRef = useRef<HTMLDivElement>(null)
  const workspaces = useMemo(() => loadWorkspaces(), [])
  const selectedWorkspace = workspaces.find((w) => w.id === selectedWorkspaceId)
  const navigate = useNavigate()
  const { chats, deleteChat, togglePinChat, isArtifactPanelOpen, openArtifactPanel, closeArtifactPanel } = useChats()
  const routerState = useRouterState()
  const isHome = routerState.location.pathname === "/"

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (workspaceRef.current && !workspaceRef.current.contains(e.target as Node)) {
        setWorkspaceOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [])

  const handleNewChat = () => {
    navigate({ to: "/" })
  }

  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton size="lg">
                <Eclipse style={{ width: "1.45rem", height: "1.45rem" }} className="text-[#a3004c]" />
                <span className="text-lg font-medium text-white" style={{ fontFamily: "'Google Sans Flex', sans-serif" }}>NightCode</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarHeader>
        <SidebarContent className="pt-0">
          <SidebarGroup className="pt-2">
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="New Chat" onClick={handleNewChat}>
                    <CirclePlus className="size-4" />
                    <span>New Chat</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Workspaces" onClick={() => navigate({ to: "/workspaces" })}>
                    <FolderOpen className="size-4" />
                    <span>Workspaces</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Schedule">
                    <ClockFading className="size-4" />
                    <span>Schedule</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Artifacts" onClick={() => isArtifactPanelOpen ? closeArtifactPanel() : openArtifactPanel()}>
                    <Scroll className="size-4" />
                    <span>Artifacts</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Settings" onClick={() => navigate({ to: "/settings" })}>
                    <Settings className="size-4" />
                    <span>Settings</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
          <SidebarGroup>
            <SidebarGroupLabel>
              <span>Recents</span>
              <Settings2 className="ml-auto !size-3" />
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {chats.map((chat) => (
                  <SidebarMenuItem key={chat.id}>
                    <SidebarMenuButton
                      tooltip={chat.title}
                      onClick={() => navigate({ to: "/chat/$chatId", params: { chatId: chat.id } })}
                    >
                      {chat.pinned ? (
                        <Pin className="size-4 shrink-0" />
                      ) : (
                        <ClockFading className="size-4 shrink-0" />
                      )}
                      <span className="truncate">{chat.title}</span>
                    </SidebarMenuButton>
                    <SidebarMenuAction showOnHover>
                      <DropdownMenu
                        trigger={<Ellipsis className="size-4" />}
                        align="right"
                      >
                        <DropdownMenuItem onClick={() => togglePinChat(chat.id)}>
                          {chat.pinned ? <PinOff className="size-3.5" /> : <Pin className="size-3.5" />}
                          <span>{chat.pinned ? "Unpin chat" : "Pin chat"}</span>
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => {}}>
                          <FolderOpen className="size-3.5" />
                          <span>Add to project</span>
                        </DropdownMenuItem>
                        <div className="my-1 h-px bg-white/10" />
                        <DropdownMenuItem variant="destructive" onClick={() => deleteChat(chat.id)}>
                          <Trash2 className="size-3.5" />
                          <span>Delete chat</span>
                        </DropdownMenuItem>
                      </DropdownMenu>
                    </SidebarMenuAction>
                  </SidebarMenuItem>
                ))}
                {chats.length === 0 && (
                  <div className="px-3 py-2 text-xs text-white/40">
                    No recent chats
                  </div>
                )}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton size="lg">
                <Avatar className="size-8">
                  <AvatarFallback>M</AvatarFallback>
                </Avatar>
                <div className="flex flex-col leading-none">
                  <span className="text-sm font-medium">Mubasher</span>
                  <span className="text-xs text-muted-foreground">mubasher@example.com</span>
                </div>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      </Sidebar>
      <SidebarInset
        className="relative flex flex-col h-screen overflow-hidden bg-cover bg-center bg-no-repeat bg-fixed"
        style={{ backgroundImage: isHome ? "url('/bg.png')" : "url('/bg2.png')" }}
      >
        {isHome && (
          <div className="relative p-4 shrink-0" ref={workspaceRef}>
            <button
              onClick={() => setWorkspaceOpen(!workspaceOpen)}
              className="flex items-center gap-2 rounded-full bg-white/0 px-3 py-1.5 text-sm text-white/70 cursor-pointer hover:bg-white/10"
            >
              <FolderOpen className="size-4" />
              <span>{selectedWorkspace?.name ?? "Workspace"}</span>
              <ChevronDown className={`size-3.5 transition-transform duration-200 ${workspaceOpen ? "rotate-180" : ""}`} />
            </button>
            {workspaceOpen && (
              <div className="absolute top-full left-4 mt-1 w-44 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden p-1">
                <button
                  onClick={() => {
                    setSelectedWorkspaceId("")
                    setWorkspaceOpen(false)
                  }}
                  className={`flex w-full items-center gap-2 rounded-full px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selectedWorkspaceId === "" ? "bg-white/5" : ""}`}
                >
                  <Check className={`size-3.5 ${selectedWorkspaceId === "" ? "opacity-100" : "opacity-0"}`} />
                  <FolderOpen className="size-3.5" />
                  <span>No workspace</span>
                </button>
                {workspaces.map((ws) => (
                  <button
                    key={ws.id}
                    onClick={() => {
                      setSelectedWorkspaceId(ws.id)
                      setWorkspaceOpen(false)
                    }}
                    className={`flex w-full items-center gap-2 rounded-full px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selectedWorkspaceId === ws.id ? "bg-white/5" : ""}`}
                  >
                    <Check className={`size-3.5 ${selectedWorkspaceId === ws.id ? "opacity-100" : "opacity-0"}`} />
                    <FolderOpen className="size-3.5" />
                    <span>{ws.name}</span>
                  </button>
                ))}
                <div className="my-1 h-px bg-white/10" />
                <button
                  onClick={() => {
                    setWorkspaceOpen(false)
                    navigate({ to: "/workspaces" })
                  }}
                  className="flex w-full items-center gap-2 rounded-full px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer"
                >
                  <span className="size-3.5" />
                  <CirclePlus className="size-3.5" />
                  <span>New Workspace</span>
                </button>
              </div>
            )}
          </div>
        )}
        <div className="flex-1 min-h-0">
          <SelectedWorkspaceContext.Provider value={selectedWorkspaceId}>
            <Outlet />
          </SelectedWorkspaceContext.Provider>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

export default App
