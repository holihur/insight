import { useEffect, useMemo, useState } from "react"
import { BookOpen, FileText, Search } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import type { IndexData, Page, TreeNode } from "@/lib/types"

function collectFiles(node: TreeNode, out: string[]) {
  if (node.type === "file") out.push(node.path)
  node.children?.forEach((c) => collectFiles(c, out))
}

export function CommandPalette({
  pages,
  index,
  open,
  onOpenChange,
  onSelectFile,
  onSelectSlug,
}: {
  pages: Page[]
  index: IndexData | null
  open: boolean
  onOpenChange: (v: boolean) => void
  onSelectFile: (path: string) => void
  onSelectSlug: (slug: string) => void
}) {
  const [q, setQ] = useState("")

  useEffect(() => {
    if (!open) setQ("")
  }, [open])

  const files = useMemo(() => {
    if (!index) return [] as string[]
    const out: string[] = []
    collectFiles(index.tree, out)
    return out
  }, [index])

  const filteredPages = useMemo(() => {
    if (!q.trim()) return pages.slice(0, 5)
    const low = q.toLowerCase()
    return pages.filter((p) => p.title.toLowerCase().includes(low) || p.slug.toLowerCase().includes(low)).slice(0, 5)
  }, [q, pages])

  const filteredFiles = useMemo(() => {
    if (!q.trim()) return files.slice(0, 8)
    const low = q.toLowerCase()
    return files.filter((f) => f.toLowerCase().includes(low)).slice(0, 10)
  }, [q, files])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="top-[18%] max-w-lg gap-0 p-0">
        <DialogHeader className="sr-only">
          <DialogTitle>Command palette</DialogTitle>
          <DialogDescription>快速跳转文件与文档</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 border-b border-border/60 px-3">
          <Search className="size-3.5 text-muted-foreground" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="搜索文件、Wiki 章节…  (试试 gin、architecture)"
            className="h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
          />
          <span className="hidden rounded border border-border/70 bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground sm:inline">ESC</span>
        </div>
        <div className="max-h-[48dvh] overflow-y-auto p-2">
          {filteredPages.length > 0 && (
            <div className="mb-2">
              <p className="px-2 py-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">Wiki</p>
              {filteredPages.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => {
                    onSelectSlug(p.slug)
                    onOpenChange(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-2.5 py-2 text-left text-sm hover:bg-accent"
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
                  onClick={() => {
                    onSelectFile(f)
                    onOpenChange(false)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-left font-mono text-[12px] hover:bg-accent"
                >
                  <FileText className="size-3 shrink-0 text-muted-foreground/60" />
                  <span className="truncate">{f}</span>
                </button>
              ))}
            </div>
          )}
          {filteredPages.length === 0 && filteredFiles.length === 0 && (
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
