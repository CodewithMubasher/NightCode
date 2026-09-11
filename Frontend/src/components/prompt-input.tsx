import { Eclipse, Plus, Paperclip, ArrowUp, ShieldCheck, Zap, ShieldAlert, ChevronDown } from "lucide-react"
import { useState, useRef, useEffect, type KeyboardEvent } from "react"

const options = [
  { value: "readonly", label: "Read Only", icon: ShieldCheck },
  { value: "auto", label: "Auto", icon: Zap },
  { value: "full", label: "Full Access", icon: ShieldAlert },
]

const models = [
  { value: "gemini-2.5-pro", label: "Gemini 2.5 Pro" },
  { value: "gemini-2.5-flash", label: "Gemini 2.5 Flash" },
  { value: "gemini-2.0-flash", label: "Gemini 2.0 Flash" },
  { value: "gemini-2.0-flash-lite", label: "Gemini 2.0 Flash-Lite" },
  { value: "gemini-1.5-pro", label: "Gemini 1.5 Pro" },
]

interface PromptInputProps {
  onSend?: (message: string) => void
  isInChat?: boolean
}

export function PromptInput({ onSend, isInChat = false }: PromptInputProps) {
  const [value, setValue] = useState("")
  const [selected, setSelected] = useState("readonly")
  const [open, setOpen] = useState(false)
  const [selectedModel, setSelectedModel] = useState("")
  const [modelOpen, setModelOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const modelRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const current = options.find((o) => o.value === selected)

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
      if (modelRef.current && !modelRef.current.contains(e.target as Node)) {
        setModelOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [])

  const handleSend = () => {
    if (!value.trim()) return
    onSend?.(value.trim())
    setValue("")
    textareaRef.current?.focus()
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  if (isInChat) {
    return (
      <div className="w-full p-4">
        <div className="mx-auto max-w-3xl">
          <div className="rounded-2xl border border-white/10 border-l-2 border-l-primary bg-white/10 backdrop-blur-[1px] px-4 pt-4 pb-2 shadow-sm">
            <textarea
              ref={textareaRef}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a message..."
              rows={2}
              className="px-1 w-full resize-none bg-transparent text-sm text-white placeholder:text-white/50 outline-none"
            />
            <div className="flex items-center justify-between mt-3">
              <div className="flex items-center gap-2.5">
                <button className="flex size-7 items-center justify-center rounded-full bg-white/5 text-white cursor-pointer">
                  <Plus className="size-4" />
                </button>
                <button className="flex size-7 items-center justify-center rounded-full bg-white/5 text-white cursor-pointer">
                  <Paperclip className="size-4" />
                </button>
                <div ref={ref} className="relative">
                  <button
                    onClick={() => setOpen(!open)}
                    className="flex h-7 items-center gap-1.5 rounded-full border-transparent bg-white/0 px-2.5 text-white/70 text-xs cursor-pointer hover:bg-white/10"
                  >
                    {current && <current.icon className="size-3.5" />}
                    <span>{current?.label}</span>
                    <ChevronDown className={`size-3.5 transition-transform duration-200 ${open ? "rotate-180" : ""}`} />
                  </button>
                  {open && (
                    <div className="absolute bottom-full left-0 mb-1 w-36 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden">
                      {options.map((opt) => (
                        <button
                          key={opt.value}
                          onClick={() => {
                            setSelected(opt.value)
                            setOpen(false)
                          }}
                          className={`flex w-full items-center gap-2 px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selected === opt.value ? "bg-white/5" : ""}`}
                        >
                          <opt.icon className="size-3.5" />
                          <span>{opt.label}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <div ref={modelRef} className="relative">
                  <button
                    onClick={() => setModelOpen(!modelOpen)}
                    className="flex h-7 items-center gap-1.5 rounded-full bg-white/0 px-2.5 text-white/70 text-xs cursor-pointer hover:bg-white/10"
                  >
                    <span>{selectedModel || "Select model"}</span>
                    <ChevronDown className={`size-3.5 transition-transform duration-200 ${modelOpen ? "rotate-180" : ""}`} />
                  </button>
                  {modelOpen && (
                    <div className="absolute bottom-full right-0 mb-1 w-52 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden">
                      <div className="px-3 py-1.5 text-[10px] font-medium text-white/40 uppercase tracking-wider">Google</div>
                      {models.map((model) => (
                        <button
                          key={model.value}
                          onClick={() => {
                            setSelectedModel(model.label)
                            setModelOpen(false)
                          }}
                          className={`flex w-full items-center gap-2 px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selectedModel === model.label ? "bg-white/5" : ""}`}
                        >
                          <span>{model.label}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
                <button
                  onClick={handleSend}
                  disabled={!value.trim()}
                  className="flex size-9 items-center justify-center rounded-full bg-primary text-primary-foreground transition-opacity disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                  <ArrowUp className="size-5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full items-center justify-center p-4 -mt-16">
      <div className="w-full max-w-[718px] flex flex-col items-center gap-5">
        <div className="flex items-center gap-2.5">
          <Eclipse className="size-7 text-primary" />
          <h1 className="text-2xl font-medium">What can I do for you?</h1>
        </div>
        <div className="w-full rounded-2xl border border-white/10 border-l-2 border-l-primary bg-white/5 backdrop-blur-[2px] px-4 pt-4 pb-2 shadow-sm">
          <textarea
            ref={textareaRef}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Describe what you want to build, / Commands @ files or sessions"
            rows={2}
            className="px-1 w-full resize-none bg-transparent text-sm text-white placeholder:text-white/50 outline-none"
          />
          <div className="flex items-center justify-between mt-3">
            <div className="flex items-center gap-2.5">
              <button className="flex size-7 items-center justify-center rounded-full bg-white/5 text-white cursor-pointer">
                <Plus className="size-4" />
              </button>
              <button className="flex size-7 items-center justify-center rounded-full bg-white/5 text-white cursor-pointer">
                <Paperclip className="size-4" />
              </button>
              <div ref={ref} className="relative">
                <button
                  onClick={() => setOpen(!open)}
                  className="flex h-7 items-center gap-1.5 rounded-full border-transparent bg-white/0 px-2.5 text-white/70 text-xs cursor-pointer hover:bg-white/10"
                >
                  {current && <current.icon className="size-3.5" />}
                  <span>{current?.label}</span>
                  <ChevronDown className={`size-3.5 transition-transform duration-200 ${open ? "rotate-180" : ""}`} />
                </button>
                {open && (
                  <div className="absolute bottom-full left-0 mb-1 w-36 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden">
                    {options.map((opt) => (
                      <button
                        key={opt.value}
                        onClick={() => {
                          setSelected(opt.value)
                          setOpen(false)
                        }}
                        className={`flex w-full items-center gap-2 px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selected === opt.value ? "bg-white/5" : ""}`}
                      >
                        <opt.icon className="size-3.5" />
                        <span>{opt.label}</span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <div ref={modelRef} className="relative">
                <button
                  onClick={() => setModelOpen(!modelOpen)}
                  className="flex h-7 items-center gap-1.5 rounded-full bg-white/0 px-2.5 text-white/70 text-xs cursor-pointer hover:bg-white/10"
                >
                  <span>{selectedModel || "Select model"}</span>
                  <ChevronDown className={`size-3.5 transition-transform duration-200 ${modelOpen ? "rotate-180" : ""}`} />
                </button>
                {modelOpen && (
                  <div className="absolute bottom-full right-0 mb-1 w-52 rounded-xl border border-white/10 bg-neutral-900 shadow-lg overflow-hidden">
                    <div className="px-3 py-1.5 text-[10px] font-medium text-white/40 uppercase tracking-wider">Google</div>
                    {models.map((model) => (
                      <button
                        key={model.value}
                        onClick={() => {
                          setSelectedModel(model.label)
                          setModelOpen(false)
                        }}
                        className={`flex w-full items-center gap-2 px-3 py-1.5 text-xs text-white hover:bg-white/10 cursor-pointer ${selectedModel === model.label ? "bg-white/5" : ""}`}
                      >
                        <span>{model.label}</span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <button
                onClick={handleSend}
                disabled={!value.trim()}
                className="flex size-9 items-center justify-center rounded-full bg-primary text-primary-foreground transition-opacity disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
              >
                <ArrowUp className="size-5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
