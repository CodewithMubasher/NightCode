import { useState, useEffect, useCallback, useMemo } from "react"
import { ArrowLeft, Copy, Check, Download, X, FileText, Code, Maximize2 } from "lucide-react"
import { useChats } from "@/context/chat-context"
import { MarkdownRenderer } from "@/components/markdown-renderer"
import type { ArtifactPart } from "@/types/message"
import { codeToHtml } from "shiki"

interface ArtifactPanelProps {
  chatId: string
  extraArtifacts?: ArtifactPart[]
}

function getLanguageLabel(language?: string): string {
  if (!language) return "Code"
  return language.toUpperCase()
}

function getArtifactIcon(artifact: ArtifactPart) {
  if (artifact.artifactType === "document") return FileText
  return Code
}

function downloadFile(name: string, content: string) {
  const blob = new Blob([content], { type: "text/plain" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

function CodeHighlighter({ code, language }: { code: string; language?: string }) {
  const [html, setHtml] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    codeToHtml(code, {
      lang: language || "text",
      theme: "github-dark",
    }).then((result) => {
      if (!cancelled) setHtml(result)
    }).catch(() => {
      if (!cancelled) setHtml(null)
    })
    return () => { cancelled = true }
  }, [code, language])

  if (html) {
    return (
      <div
        className="rounded-lg overflow-auto text-[13px] leading-5 font-mono whitespace-pre-wrap break-all scrollbar-hide [&_pre]:p-4 [&_pre]:whitespace-pre-wrap [&_pre]:break-all [&_pre]:!bg-transparent [&_pre]:border [&_pre]:border-white/10 [&_pre]:rounded-lg"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    )
  }

  return (
    <pre className="p-4 whitespace-pre-wrap break-all text-[13px] leading-5 font-mono text-white/80 bg-white/5 rounded-lg border border-white/10 scrollbar-hide overflow-auto">
      <code>{code}</code>
    </pre>
  )
}

function ArtifactCard({ artifact, onClick }: { artifact: ArtifactPart; onClick: () => void }) {
  const Icon = getArtifactIcon(artifact)
  const typeLabel = artifact.artifactType === "document" ? "Document" : "Code"
  const subtitle = artifact.language
    ? `${typeLabel} · ${getLanguageLabel(artifact.language)}`
    : typeLabel

  return (
    <button
      onClick={onClick}
      className="w-full flex items-center gap-3 p-3 rounded-lg bg-white/5 border border-white/10 hover:border-white/20 transition-all cursor-pointer text-left group"
    >
      <div className="flex-shrink-0 size-8 rounded-md bg-white/5 flex items-center justify-center group-hover:rotate-12 transition-transform duration-200">
        <Icon className="size-4 text-white/60" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm text-white/90 truncate">{artifact.name}</div>
        <div className="text-[11px] text-white/50">{subtitle}</div>
      </div>
      <button
        onClick={(e) => { e.stopPropagation(); downloadFile(artifact.name, artifact.content) }}
        className="flex-shrink-0 opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-white/10 transition-all cursor-pointer"
      >
        <Download className="size-3.5 text-white/50" />
      </button>
    </button>
  )
}

function DetailView({ artifact, onBack, onClose }: { artifact: ArtifactPart; onBack: () => void; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(artifact.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [artifact.content])

  const handleDownload = useCallback(() => {
    downloadFile(artifact.name, artifact.content)
  }, [artifact.name, artifact.content])

  const typeLabel = artifact.artifactType === "document" ? "Document" : "Code"
  const subtitle = artifact.language
    ? `${typeLabel} · ${getLanguageLabel(artifact.language)}`
    : typeLabel

  return (
    <div className={`flex flex-col h-full ${isFullscreen ? "fixed inset-0 z-50 bg-neutral-950" : ""}`}>
      <div className="flex items-center gap-2 px-4 py-3 border-b border-white/10 shrink-0">
        <button onClick={onBack} className="p-1 rounded hover:bg-white/10 transition-colors cursor-pointer">
          <ArrowLeft className="size-4 text-white/60" />
        </button>
        <div className="flex-1 min-w-0 flex items-center gap-2">
          <span className="text-sm text-white/90 truncate">{artifact.name}</span>
          <span className="flex-shrink-0 text-[10px] text-white/50 bg-white/5 border border-white/10 rounded px-1.5 py-0.5">{subtitle}</span>
        </div>
        <button onClick={handleCopy} className="p-1.5 rounded hover:bg-white/10 transition-colors cursor-pointer" title="Copy">
          {copied ? <Check className="size-3.5 text-green-400" /> : <Copy className="size-3.5 text-white/60" />}
        </button>
        <button onClick={handleDownload} className="p-1.5 rounded hover:bg-white/10 transition-colors cursor-pointer" title="Download">
          <Download className="size-3.5 text-white/60" />
        </button>
        <button onClick={() => setIsFullscreen(!isFullscreen)} className="p-1.5 rounded hover:bg-white/10 transition-colors cursor-pointer" title="Expand">
          <Maximize2 className="size-3.5 text-white/60" />
        </button>
        <button onClick={onClose} className="p-1.5 rounded hover:bg-white/10 transition-colors cursor-pointer" title="Close">
          <X className="size-3.5 text-white/60" />
        </button>
      </div>
      <div className="flex-1 min-h-0 overflow-auto p-4 scrollbar-hide">
        {artifact.artifactType === "document" ? (
          <div className="text-sm text-white/90 leading-6">
            <MarkdownRenderer content={artifact.content} />
          </div>
        ) : (
          <CodeHighlighter code={artifact.content} language={artifact.language} />
        )}
      </div>
    </div>
  )
}

export function ArtifactPanel({ chatId, extraArtifacts = [] }: ArtifactPanelProps) {
  const { activeArtifactId, openArtifact, closeArtifactPanel, getArtifactsForChat } = useChats()
  const contextArtifacts = useMemo(() => getArtifactsForChat(chatId), [getArtifactsForChat, chatId])
  const artifacts = useMemo(() => {
    const merged = [...contextArtifacts]
    for (const extra of extraArtifacts) {
      if (!merged.some((a) => a.id === extra.id)) {
        merged.push(extra)
      }
    }
    return merged
  }, [contextArtifacts, extraArtifacts])
  const activeArtifact = useMemo(
    () => artifacts.find((a) => a.id === activeArtifactId) ?? null,
    [artifacts, activeArtifactId]
  )

  const handleDownloadAll = useCallback(() => {
    for (const artifact of artifacts) {
      downloadFile(artifact.name, artifact.content)
    }
  }, [artifacts])

  return (
    <div className="flex flex-col h-full border-l border-white/10 bg-neutral-950/80">
      {activeArtifact ? (
        <DetailView
          artifact={activeArtifact}
          onBack={() => openArtifact("")}
          onClose={closeArtifactPanel}
        />
      ) : (
        <>
          <div className="flex items-center justify-between px-4 py-3 border-b border-white/10 shrink-0">
            <div className="text-sm font-medium text-white/90">Artifacts</div>
            {artifacts.length > 0 && (
              <button
                onClick={handleDownloadAll}
                className="flex items-center gap-1.5 text-[11px] text-white/50 hover:text-white/80 transition-colors cursor-pointer px-2 py-1 rounded hover:bg-white/5"
              >
                <Download className="size-3" />
                <span>Download all</span>
              </button>
            )}
          </div>
          <div className="flex-1 min-h-0 overflow-auto p-2">
            {artifacts.length === 0 ? (
              <div className="flex items-center justify-center h-full text-white/40 text-sm">
                No artifacts yet
              </div>
            ) : (
              <div className="space-y-1.5">
                {artifacts.map((artifact) => (
                  <ArtifactCard
                    key={artifact.id}
                    artifact={artifact}
                    onClick={() => openArtifact(artifact.id)}
                  />
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
