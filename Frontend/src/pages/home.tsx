import { useNavigate } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"
import { PromptInput } from "@/components/prompt-input"
import type { AttachmentPart } from "@/types/message"

export function Home() {
  const navigate = useNavigate()
  const { createChat, addMessage } = useChats()

  const handleSend = (message: string, attachments: AttachmentPart[]) => {
    const chatId = createChat(message)
    const parts = [
      ...(attachments.length > 0 ? attachments : []),
      ...(message ? [{ type: "text" as const, text: message }] : []),
    ]
    addMessage(chatId, "user", parts)

    navigate({ to: "/chat/$chatId", params: { chatId } })
  }

  return <PromptInput onSend={handleSend} />
}
