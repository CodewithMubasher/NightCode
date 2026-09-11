import { createRouter, createRoute, createRootRoute } from "@tanstack/react-router"
import { ChatView } from "@/components/chat-view"
import { Home } from "@/pages/home"
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

const routeTree = rootRoute.addChildren([indexRoute, chatRoute])

export const router = createRouter({
  routeTree,
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
