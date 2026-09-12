import { createRouter, createRoute, createRootRoute } from "@tanstack/react-router"
import { ChatView } from "@/components/chat-view"
import { Home } from "@/pages/home"
import { Workspaces } from "@/pages/workspaces"
import { WorkspaceView } from "@/pages/workspace-view"
import { Settings } from "@/pages/settings"
import { App } from "@/App"

const rootRoute = createRootRoute({
  component: App,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
})

const chatRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/chat/$chatId",
  component: () => {
    const { chatId } = chatRoute.useParams()
    return <ChatView chatId={chatId} />
  },
})

const workspacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/workspaces",
  component: Workspaces,
})

const workspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/workspaces/$workspaceId",
  component: () => {
    const { workspaceId } = workspaceRoute.useParams()
    return <WorkspaceView workspaceId={workspaceId} />
  },
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: Settings,
})

const routeTree = rootRoute.addChildren([indexRoute, chatRoute, workspacesRoute, workspaceRoute, settingsRoute])

export const router = createRouter({
  routeTree,
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
