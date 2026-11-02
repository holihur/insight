import { useEffect, useMemo, useState, type KeyboardEvent } from "react"
import { BookOpen, Box, Braces, FileText, Hash, Loader2, Search } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api } from "@/lib/api"
import type { IndexData, Page, SymbolHit, TreeNode } from "@/lib/types"
import { useI18n } from "@/i18n"

function collectFiles(node: TreeNode, out: string[]) {
  if (node.type === "file") out.push(node.path)
  node.children?.forEach((c) => collectFiles(c, out))
}

function SymbolIcon({ kind }: { kind: string }) {
  if (kind === "class") return <Box className="size-3.5 shrink-0 text-primary/70" />
  if (kind === "variable") return <Braces className="size-3.5 shrink-0 text-muted-foreground/70" />
  return <Hash className="size-3.5 shrink-0 text-muted-foreground/60" />
}

export function CommandPalette({
  repoId,
  pages,
  index,
  open,
  onOpenChange,
  onSelectFile,
  onSelectSymbol,
  onSelectSlug,
}: {
  repoId: string
  pages: Page[]
  index: IndexData | null
  open: boolean
  onOpenChange: (v: boolean) => void
  onSelectFile: (path: string) => void
  onSelectSymbol: (path: string, line: number) => void
  onSelectSlug: (slug: string) => void
}) {
  const { t } = useI18n()
  const [q, setQ] = useState("")
  const [symbols, setSymbols] = useState<SymbolHit[]>([])
  const [symbolsLoading, setSymbolsLoading] = useState(false)

  useEffect(() => {
    if (!open) {
      setQ("")
      setSymbols([])
    }
  }, [open])

  useEffect(() => {
    const query = q.trim()
    if (!open || query.length < 2) {
      setSymbols([])
      setSymbolsLoading(false)
      return
    }
    let alive = true
    const timer = window.setTimeout(() => {
      setSymbolsLoading(true)
      api
        .searchSymbols(repoId, query)
        .then((res) => {
          if (alive) setSymbols(res.symbols ?? [])
        })
        .catch(() => {
          if (alive) setSymbols([])
        })
        .finally(() => {
          if (alive) setSymbolsLoading(false)
        })
    }, 180)
    return () => {
      alive = false
      setSymbolsLoading(false)
      window.clearTimeout(timer)
    }
  }, [open, repoId, q])

  const files = useMemo(() => {
    if (!index) return [] as string[]
    const out: string[] = []
    collectFiles(index.tree, out)
    return out
  }, [index])

  const filteredPages = useMemo(() => {
    if (!q.trim()) return pages.slice(0, 4)
    const low = q.toLowerCase()
    return pages.filter((p) => p.title.toLowerCase().includes(low) || p.slug.toLowerCase().includes(low)).slice(0, 4)
  }, [q, pages])

  const filteredFiles = useMemo(() => {
    if (!q.trim()) return files.slice(0, 6)
    const low = q.toLowerCase()
    return files.filter((f) => f.toLowerCase().includes(low)).slice(0, 8)
  }, [q, files])

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return
    event.preventDefault()
    const items = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-palette-item]"))
    if (!items.length) return
    const index = items.indexOf(document.activeElement as HTMLButtonElement)
    const step = event.key === "ArrowDown" ? 1 : -1
    const next = index < 0 ? (step > 0 ? 0 : items.length - 1) : (index + step + items.length) % items.length
    items[next]?.focus()
  }

  const empty = !symbolsLoading && symbols.length === 0 && filteredPages.length === 0 && filteredFiles.length === 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="top-[18%] max-w-lg gap-0 p-0" onKeyDown={handleKeyDown}>
        <DialogHeader className="sr-only">
          <DialogTitle>Command palette</DialogTitle>
          <DialogDescription>快速跳转文件、符号与文档</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 border-b border-border/60 px-3">
          <Search className="size-3.5 text-muted-foreground" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="搜索文件、函数、类、Wiki 章节…  (试试 handler、coverage)"
            className="h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
          />
          <span className="hidden rounded border border-border/70 bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground sm:inline">ESC</span>
        </div>
        <div className="max-h-[48dvh] overflow-y-auto p-2">
          {symbolsLoading && symbols.length === 0 && (
            <div className="flex items-center gap-2 px-3 py-2 font-mono text-[10px] text-muted-foreground">
              <Loader2 className="size-3 animate-spin" /> 搜索符号…
            </div>
          )}
          {symbols.length > 0 && (
            <div className="mb-2">
              <p className="px-2 py-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">
                {t("palette.symbols")} · {symbols.length}
              </p>
              {symbols.map((symbol) => (
                <button
                  key={symbol.id}
                  type="button"
                  data-palette-item
                  onClick={() => {
                    onSelectSymbol(symbol.path, symbol.line)
                    onOpenChange(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-left hover:bg-accent focus:bg-accent focus:outline-none"
                >
                  <SymbolIcon kind={symbol.kind} />
                  <span className="min-w-0 flex-1 truncate font-mono text-[12px]">{symbol.name}</span>
                  <span className="shrink-0 font-mono text-[9px] uppercase tracking-[0.1em] text-muted-foreground/60">
                    {symbol.kind}
                  </span>
                  <span className="max-w-[45%] shrink-0 truncate font-mono text-[10px] text-muted-foreground">
                    {symbol.path}:{symbol.line}
                  </span>
                </button>
              ))}
            </div>
          )}
          {filteredPages.length > 0 && (
            <div className="mb-2">
              <p className="px-2 py-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">Wiki</p>
              {filteredPages.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  data-palette-item
                  onClick={() => {
                    onSelectSlug(p.slug)
                    onOpenChange(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-2.5 py-2 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none"
                >
                  <BookOpen className="size-3.5 text-primary/70" />
                  <span className="flex-1 truncate">{p.title}</span>
                  <span className="font-mono text-[10px] text-muted-foreground">{p.slug}</span>
                </button>
              ))}
            </div>
          )}
          {filteredFiles.length > 0 && (
            <div className="mb-1">
              <p className="px-2 py-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">Files</p>
              {filteredFiles.map((f) => (
                <button
                  key={f}
                  type="button"
                  data-palette-item
                  onClick={() => {
                    onSelectFile(f)
                    onOpenChange(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-left font-mono text-[12px] hover:bg-accent focus:bg-accent focus:outline-none"
                >
                  <FileText className="size-3 shrink-0 text-muted-foreground/60" />
                  <span className="truncate">{f}</span>
                </button>
              ))}
            </div>
          )}
          {empty && (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">没有匹配。</p>
          )}
        </div>
        <div className="flex items-center justify-between border-t border-border/60 px-3 py-1.5 text-[10px] text-muted-foreground">
          <span className="inline-flex items-center gap-1"><kbd className="rounded border border-border px-1">↑↓</kbd> 导航</span>
          <span className="inline-flex items-center gap-1"><kbd className="rounded border border-border px-1">↵</kbd> 打开</span>
          <span className="inline-flex items-center gap-1"><kbd className="rounded border border-border px-1">⌘K</kbd> 开关</span>
        </div>
      </DialogContent>
    </Dialog>
  )
}
