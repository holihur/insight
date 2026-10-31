import {
  BookOpen,
  Boxes,
  ChevronRight,
  FileCode2,
  FileText,
  Folder,
  FolderOpen,
  GitBranch,
  Layers,
  Loader2,
  Network,
  Rocket,
  Search,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn, formatNumber, languageColor } from "@/lib/utils"
import type { CoverageTree, IndexData, Page, Repo, TreeNode } from "@/lib/types"
import { CoveragePill } from "@/components/repo/CoveragePill"
import { useI18n } from "@/i18n"

export type RepoView = "wiki" | "code"

function pageIcon(slug: string) {
  if (slug.includes("overview")) return BookOpen
  if (slug.includes("getting-started") || slug.includes("start")) return Rocket
  if (slug.includes("architecture")) return Network
  if (slug.startsWith("module-")) return Boxes
  if (slug.includes("key-file")) return FileCode2
  return FileText
}

export function WikiSidebar({
  repo,
  view,
  onViewChange,
  pages,
  activeSlug,
  onSelectSlug,
  index,
  activeFile,
  onSelectFile,
  activeDir,
  onSelectDir,
  coverage,
  className,
}: {
  repo: Repo
  view: RepoView
  onViewChange: (view: RepoView) => void
  pages: Page[]
  activeSlug: string
  onSelectSlug: (slug: string) => void
  index: IndexData | null
  activeFile: string | null
  onSelectFile: (path: string) => void
  activeDir: string | null
  onSelectDir: (path: string) => void
  coverage?: CoverageTree | null
  className?: string
}) {
  const { t } = useI18n()
  const [query, setQuery] = useState("")
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const treeRef = useRef<HTMLDivElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const [recent, setRecent] = useState<string[]>(() => {
    try { const raw = window.localStorage.getItem(`code-insight-recent:${repo.id}`); return raw ? (JSON.parse(raw) as string[]) : [] } catch { return [] }
  })
  useEffect(() => {
    if (!activeFile) return
    setRecent((prev) => {
      const next = [activeFile, ...prev.filter((p) => p !== activeFile)].slice(0, 6)
      try { window.localStorage.setItem(`code-insight-recent:${repo.id}`, JSON.stringify(next)) } catch { /* ignore */ }
      return next
    })
  }, [activeFile, repo.id])
  useEffect(() => {
    if (view !== "code") return
    const handler = (e: KeyboardEvent) => {
      if (e.key === "/" && !e.metaKey && !e.ctrlKey && document.activeElement?.tagName !== "INPUT" && document.activeElement?.tagName !== "TEXTAREA") {
        e.preventDefault()
        searchInputRef.current?.focus()
      }
    }
    window.addEventListener("keydown", handler)
    return () => window.removeEventListener("keydown", handler)
  }, [view])
  const handleTreeKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const root = treeRef.current
    if (!root) return
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>("button"))
    if (buttons.length === 0) return
    const active = document.activeElement as HTMLElement | null
    const idx = active ? buttons.indexOf(active as HTMLButtonElement) : -1
    if (e.key === "ArrowDown") {
      e.preventDefault()
      const nxt = idx >= 0 ? buttons[Math.min(buttons.length - 1, idx + 1)] : buttons[0]
      nxt?.focus()
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      const prev = idx >= 0 ? buttons[Math.max(0, idx - 1)] : buttons[buttons.length - 1]
      prev?.focus()
    } else if (e.key === "Enter" || e.key === " ") {
      if (active && buttons.includes(active as HTMLButtonElement)) {
        e.preventDefault()
        ;(active as HTMLButtonElement).click()
      }
    } else if (e.key === "ArrowRight") {
      // reserved for future: expand dir
    }
  }

  useEffect(() => {
    const target = activeDir ?? activeFile
    if (!target) return
    const parts = target.split("/")
    if (activeFile) parts.pop()
    setExpanded((prev) => {
      const next = { ...prev }
      let path = ""
      for (const part of parts) {
        path = path ? `${path}/${part}` : part
        next[path] = true
      }
      return next
    })
  }, [activeDir, activeFile])

  const languages = (repo.stats.languages ?? []).slice(0, 5)
  const matches = useMemo(() => {
    if (!index || !query.trim()) return null
    const q = query.toLowerCase()
    const out: string[] = []
    const walk = (node: TreeNode) => {
      if (out.length >= 80) return
      if (node.type === "file" && node.path.toLowerCase().includes(q)) out.push(node.path)
      node.children?.forEach(walk)
    }
    walk(index.tree)
    return out
  }, [index, query])

  return (
    <div className={cn("flex h-full flex-col bg-card/20", className)}>
      <div className="flex h-12 shrink-0 items-center gap-1 border-b border-border/60 px-3">
        <div className="grid w-full grid-cols-2 rounded-sm border border-border/60 bg-muted/40 p-0.5">
          <button
            type="button"
            onClick={() => onViewChange("wiki")}
            className={cn(
              "flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition-colors",
              view === "wiki"
                ? "bg-primary/12 text-primary shadow-none"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <BookOpen className="size-3.5" /> {t("sidebar.wiki")}
          </button>
          <button
            type="button"
            onClick={() => onViewChange("code")}
            className={cn(
              "flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition-colors",
              view === "code"
                ? "bg-primary/12 text-primary shadow-none"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <FileCode2 className="size-3.5" /> {t("sidebar.code")}
          </button>
        </div>
      </div>

      {view === "wiki" ? (
        <>
          <div className="px-4 pb-2 pt-4 font-mono text-[9px] font-medium uppercase tracking-[0.22em] text-muted-foreground/80">
            {t("sidebar.documentation")}
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="space-y-0.5 px-2 pb-4">
              {pages.length === 0 && (
                <div className="space-y-2 px-2 py-3">
                  {[0, 1, 2, 3].map((i) => (
                    <div key={i} className="h-8 animate-pulse rounded-sm bg-muted/60" />
                  ))}
                </div>
              )}
              {pages.map((page) => {
                const Icon = pageIcon(page.slug)
                const active = page.slug === activeSlug
                const writing = page.status === "writing" || page.status === "pending"
                return (
                  <button
                    key={page.id}
                    type="button"
                    onClick={() => onSelectSlug(page.slug)}
                    className={cn(
                      "group flex w-full items-center gap-2.5 rounded-sm border-l-2 px-2.5 py-2 text-left text-[13px] transition-colors",
                      active
                        ? "border-primary bg-primary/8 text-foreground"
                        : "border-transparent text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                    )}
                  >
                    <Icon
                      className={cn(
                        "size-3.5 shrink-0",
                        active ? "text-primary" : "text-muted-foreground/60",
                      )}
                    />
                    <span className="min-w-0 flex-1 truncate">{page.title}</span>
                    {writing && repo.status !== "ready" && (
                      <Loader2 className="size-3 animate-spin text-primary/70" />
                    )}
                    {active && !writing && <ChevronRight className="size-3 opacity-50" />}
                  </button>
                )
              })}
            </div>
          </ScrollArea>
        </>
      ) : (
        <>
          <div className="px-3 pb-2 pt-3">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <input
                ref={searchInputRef}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t("sidebar.findFile") + " · / 聚焦"}
                className="h-8 w-full rounded-sm border border-border/70 bg-background/60 pl-8 pr-2 text-xs outline-none transition-colors placeholder:text-muted-foreground/60 focus:border-primary/40"
              />
            </div>
            {recent.length > 0 && !query && (
              <div className="mt-2 flex gap-1 overflow-x-auto pb-1">
                {recent.map((p) => (
                  <button key={p} type="button" onClick={() => onSelectFile(p)} className={cn("shrink-0 rounded-full border px-2 py-0.5 font-mono text-[10px]", activeFile === p ? "border-primary/40 bg-primary/10 text-primary" : "border-border/70 bg-muted/30 text-muted-foreground hover:border-primary/30")} title={p}>
                    {p.split("/").pop()}
                  </button>
                ))}
              </div>
            )}
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div ref={treeRef} onKeyDown={handleTreeKeyDown} tabIndex={-1} className="pb-4 outline-none">
              {!index && (
                <div className="space-y-2 px-3 pt-2">
                  {[0, 1, 2, 3, 4, 5].map((i) => (
                    <div key={i} className="h-5 animate-pulse rounded bg-muted/60" />
                  ))}
                </div>
              )}
              {index && matches && (
                <div className="space-y-0.5 px-2">
                  {matches.length === 0 && (
                    <p className="px-2 py-3 text-xs text-muted-foreground">{t("sidebar.noMatch")}</p>
                  )}
                  {matches.map((path) => (
                    <button
                      key={path}
                      type="button"
                      onClick={() => onSelectFile(path)}
                      className={cn(
                        "block w-full truncate rounded-md px-2 py-1 text-left font-mono text-[11px] transition-colors hover:bg-muted",
                        activeFile === path ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                      )}
                      title={path}
                    >
                      {path}
                    </button>
                  ))}
                </div>
              )}
              {index && !matches && (
                <Tree
                  node={index.tree}
                  depth={-1}
                  expanded={expanded}
                  setExpanded={setExpanded}
                  activeFile={activeFile}
                  onSelectFile={onSelectFile}
                  activeDir={activeDir}
                  onSelectDir={onSelectDir}
                  coverage={coverage}
                />
              )}
              <p className="mt-2 px-2 font-mono text-[9px] text-muted-foreground/60">↑↓ 导航 · Enter 打开 · / 搜索</p>
            </div>
          </ScrollArea>
        </>
      )}

      <div className="shrink-0 border-t border-border/60 p-3">
        <div className="rounded-sm border border-border/60 bg-background/30 p-3">
          <div className="flex items-center justify-between text-[10px] text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Layers className="size-3" /> {formatNumber(repo.stats.files)}{" "}
              {repo.stats.files === 1 ? "file" : "files"}
            </span>
            <span>
              {formatNumber(repo.stats.lines)}{" "}
              {t(repo.stats.lines === 1 ? "common.line" : "common.lines")}
            </span>
          </div>
          <div className="mt-2 flex flex-wrap gap-1">
            {languages.map((language) => (
              <span
                key={language.name}
                className="inline-flex items-center gap-1 rounded-full bg-muted/70 px-1.5 py-0.5 text-[10px] text-muted-foreground"
              >
                <span
                  className="size-1.5 rounded-full"
                  style={{ backgroundColor: languageColor(language.name) }}
                />
                {language.name}
              </span>
            ))}
          </div>
          {repo.branch && (
            <div className="mt-2 flex items-center gap-1 truncate text-[10px] text-muted-foreground">
              <GitBranch className="size-3 shrink-0" />
              <span className="truncate">{repo.branch}</span>
              {repo.commit && (
                <Badge variant="outline" className="ml-auto border-border/60 font-mono text-[9px]">
                  {repo.commit}
                </Badge>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function Tree({
  node,
  depth,
  expanded,
  setExpanded,
  activeFile,
  onSelectFile,
  activeDir,
  onSelectDir,
  coverage,
}: {
  node: TreeNode
  depth: number
  expanded: Record<string, boolean>
  setExpanded: React.Dispatch<React.SetStateAction<Record<string, boolean>>>
  activeFile: string | null
  onSelectFile: (path: string) => void
  activeDir: string | null
  onSelectDir: (path: string) => void
  coverage?: CoverageTree | null
}) {
  if (node.type === "file") {
    return (
      <button
        type="button"
        onClick={() => onSelectFile(node.path)}
        className={cn(
          "flex w-full items-center gap-1.5 rounded-md py-1 pr-2 text-left font-mono text-[11px] transition-colors hover:bg-muted/70",
          activeFile === node.path
            ? "bg-accent text-accent-foreground"
            : "text-muted-foreground hover:text-foreground",
        )}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        title={node.path}
      >
        <FileText className="size-3 shrink-0 opacity-50" />
        <span className="truncate">{node.name}</span>
        <CoveragePill node={coverage?.nodes[node.path]} className="ml-auto pr-1" />
      </button>
    )
  }

  const isRoot = node.path === ""
  const isOpen = isRoot || expanded[node.path] === true

  return (
    <div>
      {!isRoot && (
        <div
          className={cn(
            "flex items-center rounded-sm border-l-2",
            activeDir === node.path
              ? "border-primary bg-primary/8"
              : "border-transparent",
          )}
          style={{ paddingLeft: `${depth * 12 + 2}px` }}
        >
          <button
            type="button"
            onClick={() =>
              setExpanded((prev) => ({ ...prev, [node.path]: !prev[node.path] }))
            }
            className="grid size-6 shrink-0 place-items-center text-muted-foreground/70 transition-colors hover:text-foreground"
            aria-label={isOpen ? "Collapse directory" : "Expand directory"}
          >
            <ChevronRight
              className={cn("size-3 transition-transform", isOpen && "rotate-90")}
            />
          </button>
          <button
            type="button"
            onClick={() => {
              setExpanded((prev) => ({ ...prev, [node.path]: true }))
              onSelectDir(node.path)
            }}
            className={cn(
              "flex min-w-0 flex-1 items-center gap-1.5 py-1 pr-2 text-left text-[11px] transition-colors hover:text-foreground",
              activeDir === node.path ? "text-primary" : "text-muted-foreground",
            )}
            title={node.path}
          >
            {isOpen ? (
              <FolderOpen className="size-3 shrink-0 opacity-70" />
            ) : (
              <Folder className="size-3 shrink-0 opacity-60" />
            )}
            <span className="truncate font-medium">{node.name}</span>
            <CoveragePill node={coverage?.nodes[node.path]} className="ml-auto pr-1" />
          </button>
        </div>
      )}
      {isOpen &&
        node.children?.map((child) => (
          <Tree
            key={child.path}
            node={child}
            depth={depth + 1}
            expanded={expanded}
            setExpanded={setExpanded}
            activeFile={activeFile}
            onSelectFile={onSelectFile}
            activeDir={activeDir}
            onSelectDir={onSelectDir}
            coverage={coverage}
          />
        ))}
    </div>
  )
}
