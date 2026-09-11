import { useNavigate } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"

export function Home() {
  const navigate = useNavigate()
  const { createChat, addMessage } = useChats()

  const handleSend = (message: string) => {
    const chatId = createChat(message)
    addMessage(chatId, { role: "user", content: message })

    navigate({ to: "/chat/$chatId", params: { chatId } })
  }

  return <PromptInput onSend={handleSend} />
}
