import { useNavigate } from "react-router-dom"
import {
  ArrowUpRight,
  Clock3,
  ExternalLink,
  FileText,
  GitBranch,
  MoreHorizontal,
  RefreshCw,
  Trash2,
} from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Progress } from "@/components/ui/progress"
import { api } from "@/lib/api"
import { useI18n } from "@/i18n"
import { MazeIcon } from "@/components/icons/MazeIcon"
import { cn, formatNumber, initials, isActive, languageColor, relativeTime, statusMeta } from "@/lib/utils"
import type { Repo } from "@/lib/types"

export function RepoCard({
  repo,
  onDeleted,
  onChanged,
}: {
  repo: Repo
  onDeleted?: (id: string) => void
  onChanged?: () => void
}) {
  const navigate = useNavigate()
  const { t, locale } = useI18n()
  const meta = statusMeta[repo.status] ?? statusMeta.queued
  const statusLabel = t(`status.${repo.status}`)
  const active = isActive(repo.status)
  const languages = (repo.stats.languages ?? []).slice(0, 4)

  const remove = async () => {
    if (!window.confirm(t("card.deleteConfirm", { repo: `${repo.owner}/${repo.name}` }))) return
    try {
      await api.deleteRepo(repo.id)
      toast.success(t("card.deleted"))
      onDeleted?.(repo.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("card.deleteError"))
    }
  }

  const regenerate = async () => {
    try {
      await api.regenerate(repo.id)
      toast.success(t("card.regenerating"))
      onChanged?.()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("card.regenerateError"))
    }
  }

  return (
    <div
      role="link"
      tabIndex={0}
      onClick={() => navigate(`/repo/${repo.id}`)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault()
          navigate(`/repo/${repo.id}`)
        }
      }}
      className="group relative flex cursor-pointer flex-col overflow-hidden rounded-sm border border-border/80 bg-card/50 p-5 transition-all duration-300 hover:-translate-y-0.5 hover:border-primary/30 hover:bg-card/80 hover:shadow-[0_22px_60px_-30px_rgba(217,180,92,0.3)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
    >
      <MazeIcon className="host-watermark size-32" />
      <div className="relative flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid size-10 shrink-0 place-items-center rounded-sm border border-primary/25 bg-gradient-to-br from-primary/15 via-orange-700/10 to-transparent font-serif text-lg font-semibold text-primary">
            {initials(repo.name) || "CI"}
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5">
              <h3 className="truncate font-serif text-lg font-semibold tracking-tight">{repo.name}</h3>
              <ArrowUpRight className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
            </div>
            <p className="truncate text-xs text-muted-foreground">{repo.owner}</p>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Badge className={cn("border", meta.tone)}>
            <span className={cn("size-1.5 rounded-full", meta.dot, active && "animate-pulse")} />
            {statusLabel}
          </Badge>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground"
                onClick={(event) => event.stopPropagation()}
              >
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              onClick={(event) => event.stopPropagation()}
            >
              <DropdownMenuItem onSelect={() => navigate(`/repo/${repo.id}`)}>
                <FileText /> {t("card.open")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => window.open(repo.webUrl, "_blank")}>
                <ExternalLink /> {t("card.github")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={regenerate}>
                <RefreshCw /> {t("card.regenerate")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={remove}>
                <Trash2 /> {t("card.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <p className="mt-3 line-clamp-2 min-h-9 text-[13px] leading-relaxed text-muted-foreground">
        {repo.wiki?.summary || repo.message || t("card.waiting")}
      </p>

      {active && (
        <div className="mt-3 space-y-1.5">
          <Progress value={repo.progress} />
          <p className="truncate text-[11px] text-muted-foreground">{repo.message}</p>
        </div>
      )}

      {repo.status === "error" && (
        <p className="mt-3 line-clamp-2 rounded-lg border border-rose-500/20 bg-rose-500/5 px-2.5 py-2 text-[11px] text-rose-500">
          {repo.error || t("card.failed")}
        </p>
      )}

      <div className="relative mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-border/60 pt-3 font-mono text-[10px] text-muted-foreground">
        <span className="inline-flex items-center gap-1">
          <FileText className="size-3" /> {formatNumber(repo.stats.files)}{" "}
          {t(repo.stats.files === 1 ? "common.file" : "common.files")}
        </span>
        {repo.commit && (
          <span className="inline-flex items-center gap-1 font-mono">
            <GitBranch className="size-3" /> {repo.commit}
          </span>
        )}
        <span className="inline-flex items-center gap-1">
          <Clock3 className="size-3" /> {relativeTime(repo.updatedAt, locale)}
        </span>
        {languages.length > 0 && (
          <span className="ml-auto flex items-center gap-1.5">
            {languages.map((language) => (
              <span
                key={language.name}
                title={`${language.name} · ${language.files} files`}
                className="size-2 rounded-full ring-2 ring-card"
                style={{ backgroundColor: languageColor(language.name) }}
              />
            ))}
          </span>
        )}
      </div>
    </div>
  )
}
