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
import { Eclipse, Settings, CirclePlus, FolderOpen, ClockFading, Scroll, Settings2, Ellipsis } from "lucide-react"
import {
  Avatar,
  AvatarFallback,
} from "@/components/ui/avatar"
import { PromptInput } from "@/components/prompt-input"

export function App() {
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
                  <SidebarMenuButton tooltip="New Chat">
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
                  <SidebarMenuButton tooltip="Artifacts">
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
                {[
                  "Full stack roadmap for beginners",
                  "React performance optimization tips",
                  "Building a SaaS dashboard with shadcn",
                  "Go microservices architecture guide",
                  "AI agent harness design patterns",
                ].map((chat) => (
                  <SidebarMenuItem key={chat}>
                    <SidebarMenuButton tooltip={chat}>
                      <ClockFading className="size-4 shrink-0" />
                      <span className="truncate">{chat}</span>
                    </SidebarMenuButton>
                    <SidebarMenuAction showOnHover>
                      <Ellipsis className="size-4" />
                    </SidebarMenuAction>
                  </SidebarMenuItem>
                ))}
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
        className="bg-cover bg-center bg-no-repeat"
        style={{ backgroundImage: "url('/bg.png')" }}
      >
        <PromptInput />
      </SidebarInset>
    </SidebarProvider>
  )
}

export default App
