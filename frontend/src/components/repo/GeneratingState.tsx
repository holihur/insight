import {
  BookOpen,
  Check,
  Circle,
  GitBranch,
  Loader2,
  Radar,
  Sparkles,
  TriangleAlert,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import type { Repo } from "@/lib/types"
import { useI18n } from "@/i18n"

const steps = [
  { key: "gen.step1", hint: "gen.step1h" },
  { key: "gen.step2", hint: "gen.step2h" },
  { key: "gen.step3", hint: "gen.step3h" },
  { key: "gen.step4", hint: "gen.step4h" },
]

const rank: Record<string, number> = {
  queued: 0,
  cloning: 0,
  indexing: 1,
  planning: 2,
  generating: 3,
  ready: 4,
  error: 0,
}

export function GeneratingState({
  repo,
  onRetry,
}: {
  repo: Repo
  onRetry: () => void
}) {
  const { t } = useI18n()
  const current = rank[repo.status] ?? 0
  const failed = repo.status === "error"

  return (
    <div className="mx-auto w-full max-w-2xl px-5 py-10 sm:px-8 sm:py-14">
      <div className="mb-8 flex items-start gap-3">
        <div className="relative grid size-11 shrink-0 place-items-center rounded-full border border-primary/25 bg-primary/8">
          {failed ? (
            <TriangleAlert className="size-5 text-rose-500" />
          ) : (
            <Radar className="size-5 text-primary" />
          )}
          {!failed && (
            <span className="absolute inset-0 animate-ping rounded-xl border border-primary/20 [animation-duration:2.5s]" />
          )}
        </div>
        <div>
          <h1 className="font-serif text-2xl font-medium tracking-tight">
            {failed ? t("gen.failed") : t("gen.title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {failed
              ? repo.error || t("gen.failedBody")
              : repo.message || t("gen.working")}
          </p>
        </div>
      </div>

      {!failed && (
        <div className="frame-corners mb-8 rounded-sm border border-border/70 bg-card/50 p-5">
          <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1.5 font-mono">
              <GitBranch className="size-3" />
              {repo.owner}/{repo.name}
            </span>
            <span>{repo.progress}%</span>
          </div>
          <Progress value={repo.progress} className="h-1.5" />
          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            {steps.map((step, index) => {
              const done = current > index || repo.status === "ready"
              const active = current === index && repo.status !== "ready"
              return (
                <div
                  key={step.key}
                  className={cn(
                    "flex gap-3 rounded-lg border p-3 transition-colors",
                    active
                      ? "border-primary/25 bg-primary/5"
                      : "border-transparent bg-muted/30",
                  )}
                >
                  <span
                    className={cn(
                      "mt-0.5 grid size-5 shrink-0 place-items-center rounded-full border text-[10px]",
                      done
                        ? "border-emerald-500/30 bg-emerald-500/15 text-emerald-500"
                        : active
                          ? "border-primary/40 bg-primary/15 text-primary"
                          : "border-border text-muted-foreground",
                    )}
                  >
                    {done ? (
                      <Check className="size-3" />
                    ) : active ? (
                      <Loader2 className="size-3 animate-spin" />
                    ) : (
                      <Circle className="size-2" />
                    )}
                  </span>
                  <div className="min-w-0">
                    <p
                      className={cn(
                        "text-xs font-medium",
                        active || done ? "text-foreground" : "text-muted-foreground",
                      )}
                    >
                      {t(step.key)}
                    </p>
                    <p className="mt-0.5 text-[11px] leading-snug text-muted-foreground">
                      {t(step.hint)}
                    </p>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {failed ? (
        <div className="flex gap-2">
          <Button onClick={onRetry}>
            <Sparkles /> {t("gen.retry")}
          </Button>
        </div>
      ) : (
        <div className="rounded-sm border border-border/60 bg-card/30 p-6">
          <div className="mb-4 flex items-center gap-2 text-xs text-muted-foreground">
            <BookOpen className="size-3.5" />
            <span className="animate-pulse">{t("gen.drafting")}</span>
          </div>
          <Skeleton className="mb-2 h-7 w-3/5" />
          <Skeleton className="mb-6 h-4 w-4/5" />
          <div className="space-y-2.5">
            <Skeleton className="h-3.5 w-full" />
            <Skeleton className="h-3.5 w-[92%]" />
            <Skeleton className="h-3.5 w-[84%]" />
            <Skeleton className="h-3.5 w-[88%]" />
          </div>
          <Skeleton className="mt-6 h-28 w-full rounded-xl" />
          <div className="mt-5 space-y-2.5">
            <Skeleton className="h-3.5 w-full" />
            <Skeleton className="h-3.5 w-[78%]" />
          </div>
        </div>
      )}
    </div>
  )
}
