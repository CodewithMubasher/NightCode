/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useCallback, useEffect, useMemo, useRef, type ReactNode } from "react"
import { type Chat, type Message, type MessagePart, type ArtifactPart, type TurnSegment, generateId, truncateTitle } from "@/types/message"
import { fetchChats, fetchMessages, type ChatRow, type MessageRow } from "@/lib/backend-runtime"
import { getFileLanguage } from "@/lib/constants"

const STORAGE_KEY = "nightcode-chats"
const SAVE_DEBOUNCE_MS = 300

function loadChats(): Chat[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Chat[]
    return parsed.map((c) => ({ ...c, artifacts: c.artifacts ?? [] }))
  } catch (e) {
    console.error("Failed to load chats from localStorage:", e)
    return []
  }
}

function saveChats(chats: Chat[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(chats))
  } catch (e) {
    console.error("Failed to save chats to localStorage:", e)
  }
}

function extractTextFromSegments(segments: string): string {
  if (!segments) return ""
  try {
    const segs = JSON.parse(segments) as Array<{ type: string; text?: string }>
    return segs.filter((s) => s.type === "text").map((s) => s.text || "").join("")
  } catch (e) {
    console.warn("Failed to parse message segments:", e)
    return ""
  }
}

function convertMessageRow(row: MessageRow): Message {
  const segments = extractTextFromSegments(row.segments)
  return {
    id: row.id,
    role: row.role as "user" | "assistant",
    parts: segments ? [{ type: "text" as const, text: segments }] : [],
    timestamp: new Date(row.created_at).getTime(),
  }
}

function convertChatRow(row: ChatRow): Chat {
  return {
    id: row.id,
    workspaceId: row.workspace_id,
    title: row.title,
    messages: [],
    artifacts: [],
    createdAt: new Date(row.created_at).getTime(),
  }
}

interface ChatContextType {
  chats: Chat[]
  getChat: (id: string) => Chat | undefined
  createChat: (firstMessage: string, workspaceId?: string) => string
  addMessage: (chatId: string, role: "user" | "assistant", parts: MessagePart[], segments?: TurnSegment[]) => void
  removeMessage: (chatId: string, messageId: string) => void
  addArtifact: (chatId: string, artifact: ArtifactPart) => void
  deleteChat: (chatId: string) => void
  togglePinChat: (chatId: string) => void
  loadChatsForWorkspace: (workspaceId: string) => Promise<void>
  loadMessagesForChat: (chatId: string) => Promise<void>
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

  // Always holds the latest chats so async loaders never read a stale closure.
  const chatsRef = useRef(chats)
  useEffect(() => {
    chatsRef.current = chats
  }, [chats])

  useEffect(() => {
    // Debounce localStorage writes so rapid updates don't hammer storage,
    // but flush on page unload so the final state isn't lost.
    const timer = setTimeout(() => saveChats(chats), SAVE_DEBOUNCE_MS)
    const flush = () => saveChats(chats)
    window.addEventListener("beforeunload", flush)
    return () => {
      clearTimeout(timer)
      window.removeEventListener("beforeunload", flush)
    }
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
            newArtifacts.push({
              type: "artifact",
              id: `att-${part.name}`,
              name: part.name,
              content: part.content,
              artifactType: isMd ? "document" : "code",
              language: getFileLanguage(part.name),
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
        const exists = chat.artifacts.some(
          (a) => a.id === artifact.id && a.content === artifact.content && a.name === artifact.name
        )
        // Skip identical artifacts so repeated fetches don't churn state
        // (which previously caused needless re-render loops in the panel).
        if (exists) return chat
        const found = chat.artifacts.some((a) => a.id === artifact.id)
        if (found) {
          return {
            ...chat,
            artifacts: chat.artifacts.map((a) => (a.id === artifact.id ? artifact : a)),
          }
        }
        return { ...chat, artifacts: [...chat.artifacts, artifact] }
      })
    )
  }, [])

  const removeMessage = useCallback((chatId: string, messageId: string) => {
    setChats((prev) =>
      prev.map((chat) => {
        if (chat.id !== chatId) return chat
        return { ...chat, messages: chat.messages.filter((m) => m.id !== messageId) }
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

  const loadChatsForWorkspace = useCallback(async (workspaceId: string) => {
    const backendChats = await fetchChats(workspaceId)
    const converted = backendChats.map(convertChatRow)
    // Merge inside the updater so concurrent chat changes aren't overwritten
    // by a stale closure snapshot taken before the fetch resolved.
    setChats((prev) => {
      const merged = new Map<string, Chat>()
      for (const c of prev) merged.set(c.id, c)
      for (const c of converted) merged.set(c.id, c)
      return Array.from(merged.values())
    })
  }, [])

  const loadMessagesForChat = useCallback(async (chatId: string) => {
    const workspaceId = chatsRef.current.find((c) => c.id === chatId)?.workspaceId
    const backendMessages = await fetchMessages(chatId, workspaceId)
    const converted = backendMessages.map(convertMessageRow)
    setChats((prev) =>
      prev.map((chat) => {
        if (chat.id !== chatId) return chat
        // Merge messages, preferring backend for same IDs
        const merged = new Map<string, Message>()
        for (const m of chat.messages) merged.set(m.id, m)
        for (const m of converted) merged.set(m.id, m)
        return { ...chat, messages: Array.from(merged.values()) }
      })
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
    removeMessage,
    addArtifact,
    deleteChat,
    togglePinChat,
    loadChatsForWorkspace,
    loadMessagesForChat,
    isArtifactPanelOpen,
    activeArtifactId,
    openArtifactPanel,
    openArtifact,
    closeArtifactPanel,
    getArtifactsForChat,
  }), [sortedChats, getChat, createChat, addMessage, removeMessage, addArtifact, deleteChat, togglePinChat, loadChatsForWorkspace, loadMessagesForChat, isArtifactPanelOpen, activeArtifactId, openArtifactPanel, openArtifact, closeArtifactPanel, getArtifactsForChat])

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
