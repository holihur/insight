import { useMemo, useState } from "react"
import {
  ArrowLeft,
  Check,
  ChevronRight,
  Copy,
  ExternalLink,
  FileText,
  Folder,
  FolderOpen,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn, formatNumber } from "@/lib/utils"
import type { CoverageTree, Repo, TreeNode } from "@/lib/types"
import { CoveragePill } from "@/components/repo/CoveragePill"
import { useI18n } from "@/i18n"

function countFiles(node: TreeNode): number {
  if (node.type === "file") return 1
  return (node.children ?? []).reduce((total, child) => total + countFiles(child), 0)
}

function totalBytes(node: TreeNode): number {
  if (node.type === "file") return node.size ?? 0
  return (node.children ?? []).reduce((total, child) => total + totalBytes(child), 0)
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

export function DirectoryView({
  repo,
  node,
  coverage,
  onOpen,
}: {
  repo: Repo
  node: TreeNode
  coverage?: CoverageTree | null
  onOpen: (path: string, kind: "file" | "dir") => void
}) {
  const { t } = useI18n()
  const [copied, setCopied] = useState(false)
  const children = useMemo(() => node.children ?? [], [node])
  const dirs = children.filter((child) => child.type === "dir")
  const files = children.filter((child) => child.type === "file")
  const fileCount = countFiles(node)
  const bytes = totalBytes(node)
  const branch = repo.branch || "main"
  const pathSegments = node.path ? node.path.split("/") : []
  const githubURL = `${repo.webUrl}/tree/${branch}/${node.path
    .split("/")
    .map(encodeURIComponent)
    .join("/")}`

  const copyPath = async () => {
    try {
      await navigator.clipboard.writeText(node.path || ".")
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      // Clipboard unavailable.
    }
  }

  const openParent = () => {
    if (!node.path) return
    const parent = node.path.includes("/") ? node.path.slice(0, node.path.lastIndexOf("/")) : ""
    onOpen(parent, "dir")
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b border-border/60 px-4">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={openParent}
          disabled={!node.path}
          aria-label={t("dir.up")}
        >
          <ArrowLeft />
        </Button>
        <FolderOpen className="size-3.5 shrink-0 text-primary" />
        <div className="flex min-w-0 items-center gap-1 overflow-x-auto font-mono text-[11px] text-muted-foreground">
          <button
            type="button"
            onClick={() => onOpen("", "dir")}
            className={cn(
              "shrink-0 transition-colors hover:text-foreground",
              node.path === "" && "text-foreground",
            )}
          >
            {repo.name}
          </button>
          {pathSegments.map((segment, index) => {
            const path = pathSegments.slice(0, index + 1).join("/")
            const last = index === pathSegments.length - 1
            return (
              <span key={path} className="inline-flex shrink-0 items-center gap-1">
                <span className="opacity-40">/</span>
                <button
                  type="button"
                  onClick={() => onOpen(path, "dir")}
                  className={cn(
                    "transition-colors hover:text-foreground",
                    last && "text-foreground",
                  )}
                >
                  {segment}
                </button>
              </span>
            )
          })}
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-1">
          <Badge variant="outline" className="hidden text-[10px] sm:inline-flex">
            {t("dir.directory")}
          </Badge>
          <Button variant="ghost" size="icon-sm" onClick={copyPath} aria-label={t("dir.copy")}>
            {copied ? <Check className="text-emerald-500" /> : <Copy />}
          </Button>
          <Button variant="ghost" size="icon-sm" asChild aria-label={t("dir.github")}>
            <a href={githubURL} target="_blank" rel="noreferrer">
              <ExternalLink />
            </a>
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-4xl px-5 py-7 sm:px-8">
          <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
            <div>
              <p className="font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground">
                {t("dir.directory")}
              </p>
              <h1 className="mt-1 font-serif text-3xl font-medium tracking-tight">
                {node.name || repo.name}
              </h1>
              <p className="mt-1 flex items-center gap-2 font-mono text-[11px] text-muted-foreground">
                <span>
                  {formatNumber(dirs.length)} {t("dir.folders")} ·{" "}
                  {formatNumber(files.length)} {t("dir.files")} ·{" "}
                  {formatNumber(fileCount)} {t("common.files")} · {formatBytes(bytes)}
                </span>
                <CoveragePill node={coverage?.nodes[node.path]} showCounts className="text-[10px]" />
              </p>
            </div>
          </div>

          {children.length === 0 ? (
            <div className="grid place-items-center rounded-sm border border-dashed border-border py-20 text-center">
              <div>
                <Folder className="mx-auto mb-3 size-8 text-muted-foreground/50" />
                <p className="font-medium">{t("dir.emptyTitle")}</p>
                <p className="mt-1 text-sm text-muted-foreground">{t("dir.emptyBody")}</p>
              </div>
            </div>
          ) : (
            <div className="overflow-hidden rounded-sm border border-border/70 bg-card/20">
              {dirs.length > 0 && (
                <>
                  <div className="border-b border-border/60 bg-muted/30 px-4 py-2 font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground">
                    {t("dir.folders")}
                  </div>
                  {dirs.map((child) => (
                    <button
                      key={child.path}
                      type="button"
                      onClick={() => onOpen(child.path, "dir")}
                      className="group flex w-full items-center gap-3 border-b border-border/50 px-4 py-2.5 text-left transition-colors last:border-b-0 hover:bg-primary/5"
                    >
                      <Folder className="size-4 shrink-0 text-primary/70" />
                      <span className="min-w-0 flex-1 truncate font-mono text-[12px]">
                        {child.name}
                        <span className="text-muted-foreground/40">/</span>
                      </span>
                      <span className="hidden font-mono text-[10px] text-muted-foreground sm:block">
                        {countFiles(child)} {t("common.files")}
                      </span>
                      <CoveragePill node={coverage?.nodes[child.path]} showCounts />
                      <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/50 transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
                    </button>
                  ))}
                </>
              )}
              {files.length > 0 && (
                <>
                  <div className="border-y border-border/60 bg-muted/30 px-4 py-2 font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground">
                    {t("dir.files")}
                  </div>
                  {files.map((child) => (
                    <button
                      key={child.path}
                      type="button"
                      onClick={() => onOpen(child.path, "file")}
                      className="group flex w-full items-center gap-3 border-b border-border/50 px-4 py-2.5 text-left transition-colors last:border-b-0 hover:bg-primary/5"
                    >
                      <FileText className="size-4 shrink-0 text-muted-foreground/60" />
                      <span className="min-w-0 flex-1 truncate font-mono text-[12px]">
                        {child.name}
                      </span>
                      <CoveragePill node={coverage?.nodes[child.path]} showCounts />
                      <span className="hidden font-mono text-[10px] text-muted-foreground sm:inline">
                        {formatBytes(child.size ?? 0)}
                      </span>
                      <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/50 transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
                    </button>
                  ))}
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
