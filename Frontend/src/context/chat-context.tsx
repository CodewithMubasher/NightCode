import { createContext, useContext, useState, useCallback, type ReactNode } from "react"
import { type Chat, type Message, type MessagePart, generateId, truncateTitle } from "@/types/message"

interface ChatContextType {
  chats: Chat[]
  getChat: (id: string) => Chat | undefined
  createChat: (firstMessage: string) => string
  addMessage: (chatId: string, role: "user" | "assistant", parts: MessagePart[]) => void
}

const ChatContext = createContext<ChatContextType | null>(null)

export function ChatProvider({ children }: { children: ReactNode }) {
  const [chats, setChats] = useState<Chat[]>([])

  const getChat = useCallback((id: string) => {
    return chats.find((c) => c.id === id)
  }, [chats])

  const createChat = useCallback((firstMessage: string) => {
    const id = generateId()
    const newChat: Chat = {
      id,
      title: truncateTitle(firstMessage),
      messages: [],
      createdAt: Date.now(),
    }
    setChats((prev) => [newChat, ...prev])
    return id
  }, [])

  const addMessage = useCallback((chatId: string, role: "user" | "assistant", parts: MessagePart[]) => {
    const newMessage: Message = {
      id: generateId(),
      role,
      parts,
      timestamp: Date.now(),
    }
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === chatId
          ? { ...chat, messages: [...chat.messages, newMessage] }
          : chat
      )
    )
  }, [])

  return (
    <ChatContext.Provider value={{ chats, getChat, createChat, addMessage }}>
      {children}
    </ChatContext.Provider>
  )
}

export function useChats() {
  const context = useContext(ChatContext)
  if (!context) {
    throw new Error("useChats must be used within a ChatProvider")
  }
  return context
}
