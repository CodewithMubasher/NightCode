import { useNavigate } from "@tanstack/react-router"
import { useChats } from "@/context/chat-context"
import { useSelectedWorkspace } from "@/App"
import { PromptInput } from "@/components/prompt-input"
import type { AttachmentPart } from "@/types/message"
import type { ModelOption } from "@/lib/backend-runtime"
import { setPendingModel } from "@/lib/pending-model"

export function Home() {
  const navigate = useNavigate()
  const { createChat, addMessage } = useChats()
  const selectedWorkspaceId = useSelectedWorkspace()

  const handleSend = (message: string, attachments: AttachmentPart[], model?: ModelOption | null) => {
    const chatId = createChat(message, selectedWorkspaceId || undefined)
    const parts = [
      ...(attachments.length > 0 ? attachments : []),
      ...(message ? [{ type: "text" as const, text: message }] : []),
    ]
    addMessage(chatId, "user", parts)
    setPendingModel(chatId, model)
    navigate({ to: "/chat/$chatId", params: { chatId } })
  }

  return <PromptInput onSend={handleSend} />
}
