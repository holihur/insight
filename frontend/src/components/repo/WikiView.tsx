import { ArrowLeft, ArrowRight, Clock3, Copy, FileText, Sparkles, TriangleAlert } from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Markdown } from "@/components/repo/Markdown"
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

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      toast.success(t("wiki.copied"))
    } catch {
      toast.error(t("wiki.copyError"))
    }
  }

  return (
    <article className="mx-auto w-full max-w-3xl px-5 py-8 sm:px-8 sm:py-10">
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
          {repo.commit && (
            <Badge variant="outline" className="font-mono text-[10px]">
              {repo.commit}
            </Badge>
          )}
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
        <Markdown content={stripLeadingHeading(page.content)} onFileOpen={onOpenFile} />
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
  )
}
