import { useEffect, useMemo, useRef, useState } from "react"
import { ArrowLeft, ArrowRight, Clock3, Copy, FileText, Sparkles, TriangleAlert, Type, Maximize2, Printer, Share2, ListTree, Minus, Plus } from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Markdown, slugify } from "@/components/repo/Markdown"
import { GeneratingState } from "@/components/repo/GeneratingState"
import { cn, isActive, relativeTime } from "@/lib/utils"
import type { Page, Repo } from "@/lib/types"
import { useI18n } from "@/i18n"

function stripLeadingHeading(content: string): string {
  const trimmed = content.trimStart()
  if (!trimmed.startsWith("# ")) return content
  const newline = trimmed.indexOf("\n")
  if (newline === -1) return ""
  return trimmed.slice(newline + 1).trimStart()
}

function estimateReadingTime(content: string): number {
  const words = content.trim().split(/\s+/).filter(Boolean).length
  return Math.max(1, Math.round(words / 200))
}

function extractHeadings(content: string): Array<{ level: number; text: string; id: string }> {
  const lines = content.split("\n")
  const out: Array<{ level: number; text: string; id: string }> = []
  for (const line of lines) {
    const m2 = /^##\s+(.+)$/.exec(line)
    if (m2) { const text = m2[1].trim().replace(/\*\*/g, ""); out.push({ level: 2, text, id: slugify(text) }); continue }
    const m3 = /^###\s+(.+)$/.exec(line)
    if (m3) { const text = m3[1].trim().replace(/\*\*/g, ""); out.push({ level: 3, text, id: slugify(text) }) }
  }
  return out
}

export function WikiView({
  repo,
  page,
  onNavigate,
  onRetry,
  onOpenFile,
}: {
  repo: Repo
  page: Page | null
  onNavigate: (slug: string) => void
  onRetry: () => void
  onOpenFile?: (path: string, lines?: [number, number]) => void
}) {
  const { t, locale } = useI18n()
  const pages = repo.wiki?.pages ?? []
  const ready = pages.filter((p) => p.status === "ready" && p.content)
  const noContentYet = ready.length === 0
  const contentRef = useRef<HTMLDivElement>(null)
  const [progress, setProgress] = useState(0)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [fontScale, setFontScale] = useState<number>(() => {
    try { const v = Number(window.localStorage.getItem("wiki-font-scale")); return Number.isFinite(v) && v >= 14 && v <= 20 ? v : 15 } catch { return 15 }
  })
  const [wide, setWide] = useState<boolean>(() => {
    try { return window.localStorage.getItem("wiki-wide") === "1" } catch { return false }
  })

  useEffect(() => {
    try { window.localStorage.setItem("wiki-font-scale", String(fontScale)) } catch { /* ignore */ }
  }, [fontScale])
  useEffect(() => {
    try { window.localStorage.setItem("wiki-wide", wide ? "1" : "0") } catch { /* ignore */ }
  }, [wide])

  if (repo.status === "error" || (isActive(repo.status) && noContentYet && !page?.content)) {
    return <GeneratingState repo={repo} onRetry={onRetry} />
  }

  if (!page) {
    return (
      <div className="grid h-full place-items-center p-8 text-center">
        <div>
          <FileText className="mx-auto mb-3 size-8 text-muted-foreground/50" />
          <p className="font-medium">{t("wiki.noPage")}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t("wiki.noPageBody")}</p>
        </div>
      </div>
    )
  }

  const index = pages.findIndex((p) => p.id === page.id)
  const prev = index > 0 ? pages[index - 1] : null
  const next = index >= 0 && index < pages.length - 1 ? pages[index + 1] : null
  const writing = page.status === "writing" || page.status === "pending"
  const demo = repo.wiki?.demo
  const localOnly = demo && repo.wiki?.provider === "local"
  const rawContent = stripLeadingHeading(page.content)
  const headings = useMemo(() => extractHeadings(rawContent), [rawContent])
  const readingTime = useMemo(() => estimateReadingTime(rawContent), [rawContent])

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      toast.success(t("wiki.copied"))
    } catch {
      toast.error(t("wiki.copyError"))
    }
  }
  const copyMarkdown = async () => {
    try {
      await navigator.clipboard.writeText(page.content)
      toast.success("Markdown 已复制")
    } catch {
      toast.error("复制失败")
    }
  }

  // reading progress + active heading
  useEffect(() => {
    const scroller = document.querySelector("main") as HTMLElement | null
    const onScroll = () => {
      const el = scroller
      if (el) {
        const pct = el.scrollTop / Math.max(1, el.scrollHeight - el.clientHeight) * 100
        setProgress(pct)
      } else {
        const pct = window.scrollY / Math.max(1, document.documentElement.scrollHeight - window.innerHeight) * 100
        setProgress(pct)
      }
    }
    const el = scroller
    if (el) el.addEventListener("scroll", onScroll, { passive: true })
    else window.addEventListener("scroll", onScroll, { passive: true })
    onScroll()
    return () => {
      if (el) el.removeEventListener("scroll", onScroll)
      else window.removeEventListener("scroll", onScroll)
    }
  }, [page.id])

  useEffect(() => {
    if (headings.length === 0) return
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
        if (visible[0]) setActiveId(visible[0].target.id)
      },
      { rootMargin: "-20% 0px -70% 0px", threshold: 0.1 }
    )
    // delay to ensure headings rendered
    const t = setTimeout(() => {
      headings.forEach((h) => {
        const el = document.getElementById(h.id)
        if (el) observer.observe(el)
      })
    }, 300)
    return () => { clearTimeout(t); observer.disconnect() }
  }, [headings])

  const scrollToId = (id: string) => {
    const el = document.getElementById(id)
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  return (
    <div className="relative flex w-full flex-1 justify-center">
      {/* progress bar */}
      <div className="pointer-events-none absolute inset-x-0 top-0 z-10 h-0.5 bg-transparent">
        <div className="h-full bg-gradient-to-r from-primary via-amber-400 to-orange-600 transition-all duration-150" style={{ width: `${progress}%` }} />
      </div>

      {/* main article container with TOC layout */}
      <div className={cn("flex w-full gap-8 px-5 py-8 sm:px-8 sm:py-10", wide ? "max-w-6xl" : "max-w-3xl flex-col xl:max-w-6xl xl:flex-row xl:items-start")}>
        <article className={cn("min-w-0 flex-1", wide ? "max-w-none" : "mx-auto w-full max-w-3xl") } style={{ fontSize: `${fontScale}px` }} ref={contentRef}>
          <div className="mb-5 flex flex-wrap items-center gap-2 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
            <span className="inline-flex items-center gap-1 font-medium">
              <FileText className="size-3" /> {t("wiki.breadcrumb")}
            </span>
            <span className="text-border">/</span>
            <span className="truncate">{page.title}</span>
            <div className="ml-auto flex items-center gap-1.5">
              {repo.wiki?.generatedAt && (
                <span className="inline-flex items-center gap-1">
                  <Clock3 className="size-3" /> {relativeTime(repo.wiki.generatedAt, locale)}
                </span>
              )}
              <Button variant="ghost" size="icon-sm" onClick={copyLink} aria-label={t("wiki.copyLink")}>
                <Copy />
              </Button>
            </div>
          </div>

          <header className="mb-7">
            <h1 className="font-serif text-[2.3rem] font-medium leading-[1.05] tracking-[-0.02em] sm:text-[2.8rem]">
              {page.title}
            </h1>
            {page.description && (
              <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-muted-foreground">{page.description}</p>
            )}
            <div className="mt-3 flex flex-wrap items-center gap-1.5">
              <Badge variant="secondary" className="gap-1 font-mono text-[10px]">
                <Sparkles className="size-2.5" /> {t("wiki.grounded")}
              </Badge>
              <Badge variant="outline" className="text-[10px]">
                {t("common.sections", { count: pages.length })}
              </Badge>
              <Badge variant="outline" className="gap-1 text-[10px]">
                <Clock3 className="size-3" /> {readingTime} 分钟
              </Badge>
              {repo.commit && (
                <Badge variant="outline" className="font-mono text-[10px]">
                  {repo.commit}
                </Badge>
              )}
            </div>
            {/* toolbar */}
            <div className="mt-4 flex flex-wrap items-center gap-1.5 rounded-sm border border-border/60 bg-card/30 p-1.5">
              <span className="mr-1 inline-flex items-center gap-1 font-mono text-[10px] text-muted-foreground"><Type className="size-3" /> 排版</span>
              <Button variant="outline" size="icon-sm" className="size-7" onClick={() => setFontScale((v) => Math.max(13, v - 1))} aria-label="减小字号"><Minus className="size-3" /></Button>
              <span className="font-mono text-[10px] text-muted-foreground w-8 text-center">{fontScale}</span>
              <Button variant="outline" size="icon-sm" className="size-7" onClick={() => setFontScale((v) => Math.min(20, v + 1))} aria-label="增大字号"><Plus className="size-3" /></Button>
              <div className="mx-1 h-4 w-px bg-border/60" />
              <Button variant={wide ? "secondary" : "ghost"} size="sm" className="h-7 text-[11px]" onClick={() => setWide((v) => !v)}><Maximize2 className="size-3" /> {wide ? "标准宽度" : "宽屏"}</Button>
              <div className="ml-auto flex gap-1">
                <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={copyMarkdown}><Copy className="size-3" /> 复制 Markdown</Button>
                <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={() => window.print()}><Printer className="size-3" /> 打印</Button>
                <Button variant="ghost" size="sm" className="h-7 text-[11px]" onClick={copyLink}><Share2 className="size-3" /> 分享</Button>
              </div>
            </div>
          </header>

          {demo && (
            <div
              className={cn(
                "mb-6 flex items-start gap-2.5 rounded-xl border p-3.5 text-xs",
                localOnly
                  ? "border-primary/25 bg-primary/5 text-primary"
                  : "border-amber-500/20 bg-amber-500/5 text-amber-600 dark:text-amber-400",
              )}
            >
              <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
              <div className="leading-relaxed">
                {localOnly ? (
                  <>
                    <span className="font-medium">{t("wiki.quickTitle")}</span> {t("wiki.quickBody")}
                  </>
                ) : (
                  <>
                    <span className="font-medium">Partially local.</span> One or more sections were
                    generated from static analysis because the agent call failed. Regenerate to retry.
                  </>
                )}
              </div>
            </div>
          )}

          {writing && !page.content ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/5 px-3 py-2 text-xs text-primary">
                <Sparkles className="size-3.5 animate-pulse" />
                {t("wiki.writing")}
              </div>
              <Skeleton className="h-3.5 w-full" />
              <Skeleton className="h-3.5 w-[94%]" />
              <Skeleton className="h-3.5 w-[88%]" />
              <Skeleton className="h-40 w-full rounded-xl" />
            </div>
          ) : (
            <Markdown content={rawContent} onFileOpen={onOpenFile} repoId={repo.id} />
          )}

          {(prev || next) && (
            <nav className="mt-12 grid gap-3 border-t border-border/60 pt-6 sm:grid-cols-2">
              {prev ? (
                <button
                  type="button"
                  onClick={() => onNavigate(prev.slug)}
                  className="group rounded-sm border border-border/70 p-3.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
                >
                  <span className="flex items-center gap-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">
                    <ArrowLeft className="size-3" /> {t("wiki.prev")}
                  </span>
                  <span className="mt-1 block truncate text-sm font-medium group-hover:text-primary">
                    {prev.title}
                  </span>
                </button>
              ) : (
                <div className="hidden sm:block" />
              )}
              {next && (
                <button
                  type="button"
                  onClick={() => onNavigate(next.slug)}
                  className="group rounded-sm border border-border/70 p-3.5 text-right transition-colors hover:border-primary/30 hover:bg-primary/5"
                >
                  <span className="flex items-center justify-end gap-1 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">
                    {t("wiki.next")} <ArrowRight className="size-3" />
                  </span>
                  <span className="mt-1 block truncate text-sm font-medium group-hover:text-primary">
                    {next.title}
                  </span>
                </button>
              )}
            </nav>
          )}
        </article>

        {/* TOC aside */}
        {headings.length > 0 && (
          <aside className="hidden w-64 shrink-0 xl:block">
            <div className="sticky top-6 rounded-sm border border-border/60 bg-card/30 p-3">
              <p className="mb-2 flex items-center gap-1.5 font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">
                <ListTree className="size-3" /> 大纲
              </p>
              <div className="space-y-0.5">
                {headings.map((h) => (
                  <button
                    key={h.id}
                    type="button"
                    onClick={() => scrollToId(h.id)}
                    className={cn(
                      "block w-full truncate rounded-sm px-2 py-1 text-left text-[12px] leading-snug transition-colors hover:bg-muted",
                      h.level === 3 && "ml-3 text-[11px]",
                      activeId === h.id ? "bg-primary/8 text-primary font-medium" : "text-muted-foreground hover:text-foreground",
                    )}
                    title={h.text}
                  >
                    {h.text}
                  </button>
                ))}
              </div>
              <div className="mt-3 flex items-center gap-1 border-t border-border/60 pt-2 text-[10px] text-muted-foreground">
                <Clock3 className="size-3" /> 约 {readingTime} 分钟
              </div>
            </div>
          </aside>
        )}
      </div>
    </div>
  )
}
