import {
  BookOpen,
  Boxes,
  ChevronRight,
  FileCode2,
  FileText,
  GitBranch,
  Layers,
  Loader2,
  Network,
  Rocket,
  Search,
} from "lucide-react"
import { useMemo, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn, formatNumber, languageColor } from "@/lib/utils"
import type { IndexData, Page, Repo, TreeNode } from "@/lib/types"

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
  className?: string
}) {
  const [query, setQuery] = useState("")
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

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
            <BookOpen className="size-3.5" /> Wiki
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
            <FileCode2 className="size-3.5" /> Code
          </button>
        </div>
      </div>

      {view === "wiki" ? (
        <>
          <div className="px-4 pb-2 pt-4 font-mono text-[9px] font-medium uppercase tracking-[0.22em] text-muted-foreground/80">
            Documentation
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
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Find a file…"
                className="h-8 w-full rounded-sm border border-border/70 bg-background/60 pl-8 pr-2 text-xs outline-none transition-colors placeholder:text-muted-foreground/60 focus:border-primary/40"
              />
            </div>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="pb-4">
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
                    <p className="px-2 py-3 text-xs text-muted-foreground">No files match.</p>
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
                  depth={0}
                  expanded={expanded}
                  setExpanded={setExpanded}
                  activeFile={activeFile}
                  onSelectFile={onSelectFile}
                />
              )}
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
              {formatNumber(repo.stats.lines)} {repo.stats.lines === 1 ? "line" : "lines"}
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
}: {
  node: TreeNode
  depth: number
  expanded: Record<string, boolean>
  setExpanded: React.Dispatch<React.SetStateAction<Record<string, boolean>>>
  activeFile: string | null
  onSelectFile: (path: string) => void
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
      </button>
    )
  }

  const isRoot = depth === 0
  const isOpen = isRoot || expanded[node.path] === true

  return (
    <div>
      {!isRoot && (
        <button
          type="button"
          onClick={() =>
            setExpanded((prev) => ({ ...prev, [node.path]: !prev[node.path] }))
          }
          className="flex w-full items-center gap-1 rounded-md py-1 pr-2 text-left text-[11px] text-muted-foreground transition-colors hover:bg-muted/70 hover:text-foreground"
          style={{ paddingLeft: `${depth * 12 + 4}px` }}
        >
          <ChevronRight
            className={cn("size-3 shrink-0 transition-transform", isOpen && "rotate-90")}
          />
          <span className="truncate font-medium">{node.name}</span>
        </button>
      )}
      {isOpen &&
        node.children?.map((child) => (
          <Tree
            key={child.path}
            node={child}
            depth={isRoot ? 0 : depth + 1}
            expanded={expanded}
            setExpanded={setExpanded}
            activeFile={activeFile}
            onSelectFile={onSelectFile}
          />
        ))}
    </div>
  )
}
