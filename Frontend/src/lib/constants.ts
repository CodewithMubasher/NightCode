export const LANGUAGE_MAP: Record<string, string> = {
  tsx: "tsx",
  ts: "typescript",
  jsx: "jsx",
  js: "javascript",
  py: "python",
  md: "md",
  json: "json",
  css: "css",
  html: "html",
}

export const ACCEPTED_EXTENSIONS = new Set([
  ".ts", ".tsx", ".js", ".jsx", ".py", ".go", ".rs", ".java", ".c", ".cpp",
  ".cs", ".rb", ".php", ".swift", ".kt", ".scala", ".html", ".css", ".scss",
  ".json", ".yaml", ".yml", ".toml", ".xml", ".sql", ".sh", ".bash", ".zsh",
  ".md", ".txt", ".env", ".gitignore", ".dockerfile", ".vue", ".svelte",
])

export function getFileLanguage(filename: string): string {
  const ext = filename.split(".").pop()?.toLowerCase() || ""
  return LANGUAGE_MAP[ext] || ext
}
