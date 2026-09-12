import { createContext, useContext, useState, useCallback, useEffect, useMemo, type ReactNode } from "react"
import { type Chat, type Message, type MessagePart, type ArtifactPart, type TurnSegment, generateId, truncateTitle } from "@/types/message"

const STORAGE_KEY = "nightcode-chats"

function loadChats(): Chat[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Chat[]
    return parsed.map((c) => ({ ...c, artifacts: c.artifacts ?? [] }))
  } catch {
    return []
  }
}

function saveChats(chats: Chat[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(chats))
  } catch {}
}

interface ChatContextType {
  chats: Chat[]
  getChat: (id: string) => Chat | undefined
  createChat: (firstMessage: string, workspaceId?: string) => string
  addMessage: (chatId: string, role: "user" | "assistant", parts: MessagePart[], segments?: TurnSegment[]) => void
  addArtifact: (chatId: string, artifact: ArtifactPart) => void
  deleteChat: (chatId: string) => void
  togglePinChat: (chatId: string) => void
  isArtifactPanelOpen: boolean
  activeArtifactId: string | null
  openArtifactPanel: () => void
  openArtifact: (id: string) => void
  closeArtifactPanel: () => void
  getArtifactsForChat: (chatId: string) => ArtifactPart[]
}

const ChatContext = createContext<ChatContextType | null>(null)

export function ChatProvider({ children }: { children: ReactNode }) {
  const [chats, setChats] = useState<Chat[]>(loadChats)
  const [isArtifactPanelOpen, setIsArtifactPanelOpen] = useState(false)
  const [activeArtifactId, setActiveArtifactId] = useState<string | null>(null)

  useEffect(() => {
    saveChats(chats)
  }, [chats])

  const getChat = useCallback((id: string) => {
    return chats.find((c) => c.id === id)
  }, [chats])

  const createChat = useCallback((firstMessage: string, workspaceId?: string) => {
    const id = generateId()
    const newChat: Chat = {
      id,
      title: truncateTitle(firstMessage),
      messages: [],
      artifacts: [],
      createdAt: Date.now(),
      workspaceId,
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
      prev.map((chat) => {
        if (chat.id !== chatId) return chat
        const updatedChat = { ...chat, messages: [...chat.messages, newMessage] }
        const newArtifacts: ArtifactPart[] = []
        for (const part of parts) {
          if (part.type === "attachment" && part.content) {
            const ext = part.name.split(".").pop()?.toLowerCase() || ""
            const isMd = ext === "md"
            const languageMap: Record<string, string> = { tsx: "tsx", ts: "typescript", jsx: "jsx", js: "javascript", py: "python", md: "md", json: "json", css: "css", html: "html" }
            newArtifacts.push({
              type: "artifact",
              id: `att-${part.name}`,
              name: part.name,
              content: part.content,
              artifactType: isMd ? "document" : "code",
              language: languageMap[ext] || ext,
              createdAt: Date.now(),
            })
          }
        }
        if (newArtifacts.length > 0) {
          const merged = new Map<string, ArtifactPart>()
          for (const a of updatedChat.artifacts) merged.set(a.id, a)
          for (const a of newArtifacts) merged.set(a.id, a)
          updatedChat.artifacts = Array.from(merged.values())
        }
        return updatedChat
      })
    )
  }, [])

  const addArtifact = useCallback((chatId: string, artifact: ArtifactPart) => {
    setChats((prev) =>
      prev.map((chat) => {
        if (chat.id !== chatId) return chat
        const exists = chat.artifacts.some((a) => a.id === artifact.id)
        if (exists) {
          return {
            ...chat,
            artifacts: chat.artifacts.map((a) => (a.id === artifact.id ? artifact : a)),
          }
        }
        return { ...chat, artifacts: [...chat.artifacts, artifact] }
      })
    )
  }, [])

  const deleteChat = useCallback((chatId: string) => {
    setChats((prev) => prev.filter((c) => c.id !== chatId))
  }, [])

  const togglePinChat = useCallback((chatId: string) => {
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === chatId ? { ...chat, pinned: !chat.pinned } : chat
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
    const messageArtifacts: ArtifactPart[] = []
    for (const msg of chat.messages) {
      for (const part of msg.parts) {
        if (part.type === "artifact") {
          messageArtifacts.push(part)
        }
      }
    }
    const merged = new Map<string, ArtifactPart>()
    for (const a of messageArtifacts) merged.set(a.id, a)
    for (const a of chat.artifacts) merged.set(a.id, a)
    return Array.from(merged.values())
  }, [chats])

  const sortedChats = useMemo(() => {
    return [...chats].sort((a, b) => {
      if (a.pinned && !b.pinned) return -1
      if (!a.pinned && b.pinned) return 1
      return b.createdAt - a.createdAt
    })
  }, [chats])

  const value = useMemo(() => ({
    chats: sortedChats,
    getChat,
    createChat,
    addMessage,
    addArtifact,
    deleteChat,
    togglePinChat,
    isArtifactPanelOpen,
    activeArtifactId,
    openArtifactPanel,
    openArtifact,
    closeArtifactPanel,
    getArtifactsForChat,
  }), [sortedChats, getChat, createChat, addMessage, addArtifact, deleteChat, togglePinChat, isArtifactPanelOpen, activeArtifactId, openArtifactPanel, openArtifact, closeArtifactPanel, getArtifactsForChat])

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
