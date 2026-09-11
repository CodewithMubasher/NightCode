import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { RouterProvider } from "@tanstack/react-router"

import "./index.css"
import { router } from "./router"
import { ThemeProvider } from "@/components/theme-provider.tsx"
import { ChatProvider } from "@/context/chat-context"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <ChatProvider>
        <RouterProvider router={router} />
      </ChatProvider>
    </ThemeProvider>
  </StrictMode>
)
