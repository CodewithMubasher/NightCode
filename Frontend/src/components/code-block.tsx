import { useState } from "react"
import { Copy, Check } from "lucide-react"

interface CodeBlockProps {
  language?: string
  children: string
}

export function CodeBlock({ language, children }: CodeBlockProps) {
  const [copied, setCopied] = useState(false)

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
      <pre className="p-4 overflow-x-auto text-[13px] leading-5 font-mono text-white/80">
        <code>{children}</code>
      </pre>
    </div>
  )
}
