import { Link } from "react-router-dom"
import {
  ArrowLeft,
  ExternalLink,
  Loader2,
  Menu,
  MessageSquare,
  RefreshCw,
  Sparkles,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn, isActive, statusMeta } from "@/lib/utils"
import { useI18n } from "@/i18n"
import type { Repo } from "@/lib/types"

export function RepoHeader({
  repo,
  onAsk,
  onRegenerate,
  onOpenNav,
}: {
  repo: Repo
  onAsk: () => void
  onRegenerate: () => void
  onOpenNav: () => void
}) {
  const { t } = useI18n()
  const meta = statusMeta[repo.status] ?? statusMeta.queued
  const statusLabel = t(`status.${repo.status}`)
  const active = isActive(repo.status)

  return (
    <header className="relative z-30 flex h-14 shrink-0 items-center gap-2 border-b border-border/60 bg-background/80 px-3 backdrop-blur-xl sm:px-4">
      <Button variant="ghost" size="icon-sm" asChild aria-label="Back to home">
        <Link to="/">
          <ArrowLeft />
        </Link>
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="lg:hidden"
        onClick={onOpenNav}
        aria-label="Open navigation"
      >
        <Menu />
      </Button>

      <div className="flex min-w-0 items-center gap-2">
        <div className="grid size-7 shrink-0 place-items-center rounded-sm border border-primary/30 bg-primary/8 font-serif text-[11px] font-semibold text-primary">
          {repo.name.slice(0, 2).toUpperCase()}
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <span className="truncate font-serif text-[15px] font-semibold leading-tight">{repo.name}</span>
            <Badge className={cn("hidden border sm:inline-flex", meta.tone)}>
              <span className={cn("size-1.5 rounded-full", meta.dot, active && "animate-pulse")} />
              {statusLabel}
            </Badge>
          </div>
          <p className="hidden truncate text-[10px] leading-tight text-muted-foreground sm:block">
            {repo.owner}
            {repo.branch ? ` · ${repo.branch}` : ""}
            {repo.commit ? ` · ${repo.commit}` : ""}
          </p>
        </div>
      </div>

      <div className="ml-auto flex items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              className="hidden gap-1.5 md:inline-flex"
              onClick={onRegenerate}
              disabled={active}
            >
              {active ? <Loader2 className="animate-spin" /> : <RefreshCw />}
              <span className="hidden lg:inline">{t("repo.regenerate")}</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("repo.regenerate")}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" asChild>
              <a href={repo.webUrl} target="_blank" rel="noreferrer" aria-label={t("code.github")}>
                <ExternalLink />
              </a>
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("code.github")}</TooltipContent>
        </Tooltip>
        <Button size="sm" onClick={onAsk} className="gap-1.5 xl:hidden">
          <MessageSquare />
          <span className="hidden sm:inline">{t("repo.ask")}</span>
        </Button>
        <Button size="sm" variant="secondary" className="hidden gap-1.5 xl:inline-flex" asChild>
          <a href="#chat">
            <Sparkles /> {t("repo.ask")}
          </a>
        </Button>
      </div>

      {active && (
        <Progress
          value={repo.progress}
          className="absolute inset-x-0 bottom-[-1px] h-0.5 rounded-none bg-transparent"
          indicatorClassName="bg-gradient-to-r from-primary via-amber-400 to-orange-600"
        />
      )}
    </header>
  )
}
