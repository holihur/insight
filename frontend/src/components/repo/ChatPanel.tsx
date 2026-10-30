import { useEffect, useRef, useState } from "react"
import {
  ArrowUp,
  BarChart3,
  Check,
  ChevronDown,
  FileCode2,
  FileText,
  FolderTree,
  Loader2,
  Maximize2,
  MessageSquare,
  PanelRight,
  RotateCcw,
  Search,
  Square,
  Wrench,
  Copy,
  X,
  Pencil,
  RefreshCw,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Markdown } from "@/components/repo/Markdown"
import { MazeIcon } from "@/components/icons/MazeIcon"
import { cn } from "@/lib/utils"
import type { ChatMessage, ChatPart, Repo } from "@/lib/types"
import { useI18n } from "@/i18n"

const suggestionKeys = ["chat.s1", "chat.s2", "chat.s3", "chat.s4"]

function toolLabel(
  part: Extract<ChatPart, { type: "tool" }>,
  t: (key: string, vars?: Record<string, string | number>) => string,
): string {
  const args = part.args ?? {}
  const pick = (...keys: string[]) => {
    for (const key of keys) {
      const value = args[key]
      if (typeof value === "string" && value.trim()) return value
    }
    return ""
  }
  switch (part.name) {
    case "read_file":
      return t("chat.reading", { path: pick("path") || "…" })
    case "list_files":
      return t("chat.listing", { dir: pick("dir") || "." })
    case "search_code":
      return t("chat.searching", { query: pick("query") })
    case "repo_stats":
      return t("chat.stats")
    default:
      return part.name.replaceAll("_", " ")
  }
}

function toolIcon(name: string) {
  switch (name) {
    case "read_file":
      return FileText
    case "list_files":
      return FolderTree
    case "search_code":
      return Search
    case "repo_stats":
      return BarChart3
    default:
      return Wrench
  }
}

function ToolChip({ part }: { part: Extract<ChatPart, { type: "tool" }> }) {
  const { t } = useI18n()
  const Icon = toolIcon(part.name)
  const running = part.status === "running"
  return (
    <details className="group/tool overflow-hidden rounded-sm border border-border/70 bg-background/50">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-2.5 py-1.5 text-[11px] text-muted-foreground transition-colors hover:text-foreground">
        <Icon className="size-3 shrink-0" />
        <span className="min-w-0 flex-1 truncate font-mono">{toolLabel(part, t)}</span>
        {running ? (
          <Loader2 className="size-3 shrink-0 animate-spin text-primary" />
        ) : (
          <Check className="size-3 shrink-0 text-emerald-500" />
        )}
        <ChevronDown className="size-3 shrink-0 opacity-40 transition-transform group-open/tool:rotate-180" />
      </summary>
      {part.result && (
        <pre className="max-h-40 overflow-auto border-t border-border/60 bg-code-bg px-2.5 py-2 font-mono text-[10px] leading-relaxed text-muted-foreground whitespace-pre-wrap">
          {part.result}
        </pre>
      )}
    </details>
  )
}

function AssistantMessage({ message, onOpenFile }: { message: ChatMessage; onOpenFile?: (path: string, lines?: [number, number]) => void }) {
  const { t } = useI18n()
  const textParts = message.parts.filter((part) => part.type === "text") as Array<{
    type: "text"
    text: string
  }>
  const toolParts = message.parts.filter((part) => part.type === "tool") as Array<
    Extract<ChatPart, { type: "tool" }>
  >
  const hasText = textParts.some((part) => part.text.trim())

  return (
    <div className="flex gap-2.5">
      <div className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-full border border-primary/30 bg-primary/10">
        <MazeIcon className="size-4 text-primary" />
      </div>
      <div className="min-w-0 flex-1 space-y-2">
        {toolParts.length > 0 && (
          <div className="space-y-1.5">
            {toolParts.map((part) => (
              <ToolChip key={part.id} part={part} />
            ))}
          </div>
        )}
        {textParts.map((part, index) => (
          <div
            key={index}
            className={cn(
              "rounded-xl text-[13px]",
              index === 0 ? "" : "border-t border-border/40 pt-2",
            )}
          >
            <Markdown content={part.text} className="markdown-chat" onFileOpen={onOpenFile} />
          </div>
        ))}
        {message.streaming && !hasText && (
          <div className="flex items-center gap-1.5 py-1.5 text-xs text-muted-foreground">
            <span className="flex gap-1">
              <span className="size-1.5 animate-bounce rounded-full bg-primary [animation-delay:-0.2s]" />
              <span className="size-1.5 animate-bounce rounded-full bg-primary [animation-delay:-0.1s]" />
              <span className="size-1.5 animate-bounce rounded-full bg-primary" />
            </span>
            {t("chat.thinking")}
          </div>
        )}
        {message.error && (
          <div className="rounded-lg border border-rose-500/20 bg-rose-500/5 px-2.5 py-2 text-[11px] text-rose-500">
            {message.error}
          </div>
        )}
        {!message.streaming && !message.error && !hasText && toolParts.length === 0 && (
          <p className="text-xs text-muted-foreground">{t("chat.noResponse")}</p>
        )}
      </div>
    </div>
  )
}

export function ChatPanel({
  repo,
  messages,
  onSend,
  onStop,
  onReset,
  streaming,
  className,
  hideHeader = false,
  inDialog = false,
  onClose,
  context,
  dockMode,
  onToggleDock,
  onOpenFile,
}: {
  repo: Repo
  messages: ChatMessage[]
  onSend: (text: string) => void
  onStop: () => void
  onReset: () => void
  streaming: boolean
  className?: string
  hideHeader?: boolean
  inDialog?: boolean
  onClose?: () => void
  context?: { label: string; detail: string; path: string | null; lines: [number, number] | null } | null
  dockMode?: "floating" | "docked"
  onToggleDock?: () => void
  onOpenFile?: (path: string, lines?: [number, number]) => void
}) {
  const { t } = useI18n()
  const [value, setValue] = useState("")
  const [includeContext, setIncludeContext] = useState(true)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingValue, setEditingValue] = useState("")
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const demo = repo.demo || repo.wiki?.provider === "local"

  useEffect(() => {
    setIncludeContext(true)
  }, [context?.detail])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end", behavior: "smooth" })
  }, [messages])

  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = "auto"
    el.style.height = `${Math.min(el.scrollHeight, 132)}px`
  }, [value])

  const submit = () => {
    const text = value.trim()
    if (!text || streaming) return
    let out = text
    if (includeContext && context?.path) {
      const lineSuffix = context.lines ? `:${context.lines[0]}-${context.lines[1]}` : ""
      out = `[@${context.path}${lineSuffix}] ${text}`
    } else if (includeContext && context && !context.path) {
      out = `[Wiki:${context.detail}] ${text}`
    }
    onSend(out)
    setValue("")
  }
  const copyLast = async (msg: ChatMessage) => {
    const txt = msg.parts.filter((p) => p.type === "text").map((p) => (p.type === "text" ? p.text : "")).join("")
    try { await navigator.clipboard.writeText(txt) } catch { /* ignore */ }
  }

  return (
    <div className={cn("flex h-full min-h-0 flex-col bg-card/20", className)}>
      {!hideHeader && (
        <div
          className={cn(
            "flex h-12 shrink-0 items-center gap-2 border-b border-border/60 px-3.5",
            inDialog && "pr-10",
          )}
        >
          <MessageSquare className="size-3.5 text-primary" />
          <span className="font-mono text-[10px] font-medium uppercase tracking-[0.16em]">
            {t("chat.title")}
          </span>
          <Badge variant={demo ? "warning" : "secondary"} className="ml-1 font-mono text-[9px]">
            {t(demo ? "chat.local" : "chat.grounded")}
          </Badge>
          <div className="ml-auto flex items-center gap-0.5">
            {onToggleDock && (
              <Button variant="ghost" size="icon-sm" className="text-muted-foreground" onClick={onToggleDock} aria-label={dockMode === "docked" ? "浮动" : "侧边固定"} title={dockMode === "docked" ? "切换为浮动" : "固定到侧边"}>
                {dockMode === "docked" ? <Maximize2 className="size-3.5" /> : <PanelRight className="size-3.5" />}
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon-sm"
              className="text-muted-foreground"
              onClick={onReset}
              aria-label={t("chat.reset")}
            >
              <RotateCcw />
            </Button>
            {onClose && (
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground"
                onClick={onClose}
                aria-label="Close"
              >
                <X />
              </Button>
            )}
          </div>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto px-3.5 py-4">
        {messages.length === 0 ? (
          <div className="flex h-full flex-col pt-8">
            <div className="mb-5 flex items-center gap-2.5">
              <div className="grid size-9 place-items-center rounded-full border border-primary/25 bg-primary/10">
                <MazeIcon className="size-5 text-primary" />
              </div>
              <div>
                <p className="font-serif text-lg font-medium">{t("chat.emptyTitle")}</p>
                <p className="text-[11px] text-muted-foreground">
                  {t("chat.emptyBody", { repo: `${repo.owner}/${repo.name}` })}
                </p>
              </div>
            </div>
            {context && (
              <div className="mb-3 flex items-center gap-1.5 rounded-sm border border-primary/20 bg-primary/5 px-2.5 py-1.5 text-[11px]">
                <FileCode2 className="size-3 text-primary" />
                <span className="min-w-0 flex-1 truncate font-mono text-primary">{context.detail}</span>
                <label className="flex items-center gap-1 text-[10px] text-muted-foreground">
                  <input type="checkbox" checked={includeContext} onChange={(e) => setIncludeContext(e.target.checked)} className="size-3 rounded" /> 关联
                </label>
                <span className="text-[10px] text-muted-foreground/60">上下文</span>
              </div>
            )}
            <div className="space-y-1.5">
              {suggestionKeys.map((key) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => {
                    const base = t(key)
                    let out = base
                    if (includeContext && context?.path) out = `[@${context.path}] ${base}`
                    onSend(out)
                  }}
                  className="flex w-full items-start gap-2 rounded-sm border border-border/60 bg-background/30 px-3 py-2.5 text-left text-[12px] leading-snug text-muted-foreground transition-colors hover:border-primary/30 hover:bg-primary/5 hover:text-foreground"
                >
                  <ArrowUp className="mt-0.5 size-3 shrink-0 -rotate-45 text-primary/70" />
                  {t(key)}
                </button>
              ))}
            </div>
            {demo && (
              <p className="mt-4 rounded-lg border border-amber-500/20 bg-amber-500/5 px-2.5 py-2 text-[10px] leading-relaxed text-amber-600 dark:text-amber-400">
                {t("chat.localNote")}
              </p>
            )}
          </div>
        ) : (
          <div className="space-y-5 pb-2">
            {messages.map((message, idx) =>
              message.role === "user" ? (
                <div key={message.id} className="flex flex-col items-end gap-1 group/msg">
                  {editingId === message.id ? (
                    <div className="w-[88%] rounded-sm border border-primary/30 bg-background p-2">
                      <textarea value={editingValue} onChange={(e) => setEditingValue(e.target.value)} rows={2} className="w-full resize-none bg-transparent text-[13px] outline-none" autoFocus />
                      <div className="mt-2 flex justify-end gap-1">
                        <Button variant="ghost" size="sm" className="h-6 text-[11px]" onClick={() => setEditingId(null)}>取消</Button>
                        <Button size="sm" className="h-6 text-[11px]" disabled={!editingValue.trim()} onClick={() => { const v = editingValue.trim(); setEditingId(null); if (v) onSend(v) }}>重发</Button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className="max-w-[88%] whitespace-pre-wrap rounded-sm rounded-br-none bg-primary px-3.5 py-2.5 text-[13px] leading-relaxed text-primary-foreground shadow-sm">
                        {message.parts
                          .filter((part) => part.type === "text")
                          .map((part) => (part.type === "text" ? part.text : ""))
                          .join("")}
                      </div>
                      <div className="flex gap-1 opacity-0 transition-opacity group-hover/msg:opacity-100">
                        <button type="button" onClick={() => void navigator.clipboard.writeText(message.parts.filter((p) => p.type === "text").map((p) => (p.type === "text" ? p.text : "")).join("")).catch(() => {})} className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"><Copy className="size-3" /> 复制</button>
                        <button type="button" onClick={() => { setEditingId(message.id); setEditingValue(message.parts.filter((p) => p.type === "text").map((p) => (p.type === "text" ? p.text : "")).join("")) }} className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"><Pencil className="size-3" /> 编辑</button>
                      </div>
                    </>
                  )}
                </div>
              ) : (
                <div className="group/msg">
                  <AssistantMessage message={message} onOpenFile={onOpenFile} />
                  {!message.streaming && (
                    <div className="ml-9 mt-1 flex gap-1 opacity-0 transition-opacity group-hover/msg:opacity-100">
                      <button type="button" onClick={() => void copyLast(message)} className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"><Copy className="size-3" /> 复制</button>
                      {message.error && idx > 0 && (() => { const prev = [...messages].slice(0, idx).reverse().find((m) => m.role === "user"); const txt = prev ? prev.parts.filter((p) => p.type === "text").map((p) => (p.type === "text" ? p.text : "")).join("") : ""; return txt ? <button type="button" onClick={() => { if (txt) onSend(txt) }} className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-rose-500 hover:bg-rose-500/10"><RefreshCw className="size-3" /> 重试</button> : null })()}
                    </div>
                  )}
                </div>
              ),
            )}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {context && includeContext && context.path && (
        <div className="mx-3 mb-1 flex items-center gap-1.5 text-[10px] text-primary">
          <FileCode2 className="size-3" /> 关联上下文: <span className="font-mono truncate">{context.detail}</span>
        </div>
      )}
      <div className="shrink-0 border-t border-border/60 p-3">
        <div className="rounded-sm border border-border/70 bg-background/60 p-1.5 shadow-sm transition-colors focus-within:border-primary/40">
          <textarea
            ref={textareaRef}
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault()
                submit()
              }
            }}
            rows={1}
            placeholder={includeContext && context?.path ? `基于 ${context.path.split("/").pop()} 提问…` : t("chat.placeholder")}
            className="max-h-32 min-h-[38px] w-full resize-none bg-transparent px-2 py-2 text-[13px] leading-relaxed outline-none placeholder:text-muted-foreground/60"
          />
          <div className="flex items-center justify-between px-1 pb-0.5">
            <span className="text-[10px] text-muted-foreground/70">
              <kbd className="rounded border border-border/70 px-1">Enter</kbd> {t("chat.enter")} ·{" "}
              <kbd className="rounded border border-border/70 px-1">Shift</kbd>+
              <kbd className="rounded border border-border/70 px-1">Enter</kbd> {t("chat.shift")}
            </span>
            {streaming ? (
              <Button
                size="icon-sm"
                variant="secondary"
                onClick={onStop}
                aria-label={t("chat.stop")}
              >
                <Square className="size-3 fill-current" />
              </Button>
            ) : (
              <Button
                size="icon-sm"
                onClick={submit}
                disabled={!value.trim()}
                aria-label={t("chat.send")}
              >
                <ArrowUp />
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
