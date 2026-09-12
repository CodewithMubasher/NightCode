import { createContext, useContext, useState, useCallback, useMemo, type ReactNode } from "react"
import { type Chat, type Message, type MessagePart, type ArtifactPart, type TurnSegment, generateId, truncateTitle } from "@/types/message"

interface ChatContextType {
  chats: Chat[]
  getChat: (id: string) => Chat | undefined
  createChat: (firstMessage: string) => string
  addMessage: (chatId: string, role: "user" | "assistant", parts: MessagePart[], segments?: TurnSegment[]) => void
  isArtifactPanelOpen: boolean
  activeArtifactId: string | null
  openArtifactPanel: () => void
  openArtifact: (id: string) => void
  closeArtifactPanel: () => void
  getArtifactsForChat: (chatId: string) => ArtifactPart[]
}

const ChatContext = createContext<ChatContextType | null>(null)

export function ChatProvider({ children }: { children: ReactNode }) {
  const [chats, setChats] = useState<Chat[]>([])
  const [isArtifactPanelOpen, setIsArtifactPanelOpen] = useState(false)
  const [activeArtifactId, setActiveArtifactId] = useState<string | null>(null)

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

  const addMessage = useCallback((chatId: string, role: "user" | "assistant", parts: MessagePart[], segments?: TurnSegment[]) => {
    const newMessage: Message = {
      id: generateId(),
      role,
      parts,
      segments,
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

  const openArtifactPanel = useCallback(() => {
    setIsArtifactPanelOpen(true)
    setActiveArtifactId(null)
  }, [])

  const openArtifact = useCallback((id: string) => {
    setIsArtifactPanelOpen(true)
    setActiveArtifactId(id)
  }, [])

  const closeArtifactPanel = useCallback(() => {
    setIsArtifactPanelOpen(false)
    setActiveArtifactId(null)
  }, [])

  const getArtifactsForChat = useCallback((chatId: string): ArtifactPart[] => {
    const chat = chats.find((c) => c.id === chatId)
    if (!chat) return []
    const artifacts: ArtifactPart[] = []
    for (const msg of chat.messages) {
      for (const part of msg.parts) {
        if (part.type === "artifact") {
          artifacts.push(part)
        }
      }
    }
    return artifacts
  }, [chats])

  const value = useMemo(() => ({
    chats,
    getChat,
    createChat,
    addMessage,
    isArtifactPanelOpen,
    activeArtifactId,
    openArtifactPanel,
    openArtifact,
    closeArtifactPanel,
    getArtifactsForChat,
  }), [chats, getChat, createChat, addMessage, isArtifactPanelOpen, activeArtifactId, openArtifactPanel, openArtifact, closeArtifactPanel, getArtifactsForChat])

  return (
    <ChatContext.Provider value={value}>
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
