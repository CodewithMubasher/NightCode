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
import { Eclipse, Settings, CirclePlus, FolderOpen, ClockFading, Scroll, Settings2, Ellipsis, ChevronDown, Check } from "lucide-react"
import {
  Avatar,
  AvatarFallback,
} from "@/components/ui/avatar"
import { useState, useRef, useEffect } from "react"
import { Outlet, useNavigate, useRouterState } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"

export function App() {
  const [workspaceOpen, setWorkspaceOpen] = useState(false)
  const [workspace, setWorkspace] = useState("my-workspace")
  const workspaceRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const { chats, isArtifactPanelOpen, openArtifactPanel, closeArtifactPanel } = useChats()
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
                <Eclipse style={{ width: "1.45rem", height: "1.45rem" }} className="text-primary" />
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
                  <SidebarMenuButton tooltip="Projects">
                    <FolderOpen className="size-4" />
                    <span>Projects</span>
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
                  <SidebarMenuButton tooltip="Settings">
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
                      <ClockFading className="size-4 shrink-0" />
                      <span className="truncate">{chat.title}</span>
                    </SidebarMenuButton>
                    <SidebarMenuAction showOnHover>
                      <Ellipsis className="size-4" />
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
              className="flex items-center gap-2 rounded-lg bg-white/0 px-3 py-1.5 text-sm text-white/70 cursor-pointer hover:bg-white/10"
            >
              <FolderOpen className="size-4" />
              <span>Workspace</span>
              <ChevronDown className={`size-3.5 transition-transform duration-200 ${workspaceOpen ? "rotate-180" : ""}`} />
            </button>
            {workspaceOpen && (
              <div className="absolute top-full left-4 mt-1 w-52 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden p-1">
                <button
                  onClick={() => {
                    setWorkspace("my-workspace")
                    setWorkspaceOpen(false)
                  }}
                  className={`flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${workspace === "my-workspace" ? "bg-white/5" : ""}`}
                >
                  <Check className={`size-3.5 ${workspace === "my-workspace" ? "opacity-100" : "opacity-0"}`} />
                  <FolderOpen className="size-3.5" />
                  <span>My Workspace</span>
                </button>
                <div className="my-1 h-px bg-white/10" />
                <button
                  onClick={() => setWorkspaceOpen(false)}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer"
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
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

export default App
