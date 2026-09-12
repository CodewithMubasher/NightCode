import { useState, useRef, useEffect, type ReactNode } from "react"

interface DropdownMenuProps {
  trigger: ReactNode
  children: ReactNode
  align?: "left" | "right"
}

export function DropdownMenu({ trigger, children, align = "right" }: DropdownMenuProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [open])

  return (
    <div ref={ref} className="relative">
      <div onClick={() => setOpen(!open)}>{trigger}</div>
      {open && (
        <div
          className={`absolute top-full mt-1 z-50 w-48 rounded-xl border border-white/10 bg-neutral-900 shadow-lg p-1 ${
            align === "right" ? "right-0" : "left-0"
          }`}
          onClick={() => setOpen(false)}
        >
          {children}
        </div>
      )}
    </div>
  )
}

interface DropdownMenuItemProps {
  children: ReactNode
  onClick?: () => void
  variant?: "default" | "destructive"
}

export function DropdownMenuItem({ children, onClick, variant = "default" }: DropdownMenuItemProps) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-xs cursor-pointer transition-colors ${
        variant === "destructive"
          ? "text-red-400 hover:bg-red-500/10"
          : "text-white hover:bg-white/10"
      }`}
    >
      {children}
    </button>
  )
}
