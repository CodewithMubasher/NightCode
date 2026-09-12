import { XIcon } from "lucide-react"
import type { AttachmentPart } from "@/types/message"
import {
  Attachment,
  AttachmentMedia,
  AttachmentContent,
  AttachmentTitle,
  AttachmentDescription,
  AttachmentActions,
  AttachmentAction,
  AttachmentBadge,
} from "@/components/ui/attachment"

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function isCodeFile(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() || ""
  const codeExts = ["tsx", "ts", "jsx", "js", "py", "json", "css", "html", "rb", "go", "rs", "java", "c", "cpp", "cs", "sh", "md", "yaml", "yml", "xml", "sql", "vue", "svelte"]
  return codeExts.includes(ext)
}

export function isMarkdownFile(name: string): boolean {
  return name.split(".").pop()?.toLowerCase() === "md"
}

function getFileExtensionBadge(name: string): string {
  return name.split(".").pop()?.toUpperCase() ?? "FILE"
}

interface AttachmentCardProps {
  attachment: AttachmentPart
  onRemove?: (id: string) => void
  onClick?: () => void
}

export function AttachmentCard({ attachment, onRemove, onClick }: AttachmentCardProps) {
  const att = attachment
  const isImage = !!att.data
  const isCode = isCodeFile(att.name)
  const lineCount = att.content ? att.content.split("\n").length : null
  const badge = getFileExtensionBadge(att.name)

  return (
    <Attachment
      orientation="vertical"
      className={`w-[140px] flex-none cursor-pointer hover:bg-white/5`}
      onClick={onClick}
    >
      {isImage ? (
        <AttachmentMedia variant="image" className="aspect-[4/3] w-full rounded-lg">
          <img src={att.data} alt={att.name} className="size-full object-cover rounded-lg" />
        </AttachmentMedia>
      ) : null}
      <AttachmentContent className="px-2 py-1.5">
        <AttachmentTitle className="whitespace-normal line-clamp-2 text-white/90">
          {att.name}
        </AttachmentTitle>
        {lineCount !== null ? (
          <AttachmentDescription className="text-white/50">
            {lineCount} lines
          </AttachmentDescription>
        ) : (
          <AttachmentDescription className="text-white/50">
            {formatFileSize(att.size)}
          </AttachmentDescription>
        )}
        {isCode && <AttachmentBadge>{badge}</AttachmentBadge>}
      </AttachmentContent>
      <AttachmentActions>
        {onRemove && (
          <AttachmentAction onClick={() => onRemove(att.id)}>
            <XIcon />
          </AttachmentAction>
        )}
      </AttachmentActions>
    </Attachment>
  )
}
