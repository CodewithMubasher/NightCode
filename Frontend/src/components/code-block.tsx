import { useState, useEffect } from "react"
import { Copy, Check } from "lucide-react"
import { codeToHtml } from "shiki"

interface CodeBlockProps {
  language?: string
  children: string
}

export function CodeBlock({ language, children }: CodeBlockProps) {
  const [copied, setCopied] = useState(false)
  const [html, setHtml] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    codeToHtml(children, {
      lang: language || "text",
      theme: "github-dark",
    }).then((result) => {
      if (!cancelled) setHtml(result)
    }).catch(() => {
      if (!cancelled) setHtml(null)
    })
    return () => { cancelled = true }
  }, [children, language])

  const handleCopy = () => {
    navigator.clipboard.writeText(children)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="my-3 rounded-xl border border-white/10 bg-white/5 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-1.5 border-b border-white/10">
        <span className="text-[11px] text-white/40 font-mono">{language || "code"}</span>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1 text-[11px] text-white/40 hover:text-white/70 transition-colors cursor-pointer"
        >
          {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
          <span>{copied ? "Copied" : "Copy"}</span>
        </button>
      </div>
      {html ? (
        <div
          className="text-[13px] leading-5 font-mono scrollbar-hide [&_pre]:p-4 [&_pre]:whitespace-pre-wrap [&_pre]:break-all [&_pre]:!bg-transparent [&_pre]:overflow-hidden"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      ) : (
        <pre className="p-4 whitespace-pre-wrap break-all text-[13px] leading-5 font-mono text-white/80 overflow-hidden scrollbar-hide">
          <code>{children}</code>
        </pre>
      )}
    </div>
  )
}
