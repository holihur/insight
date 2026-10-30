import { useEffect, useState } from "react"
import {
  ArrowRight,
  ArrowUpRight,
  CircleAlert,
  GitBranch,
  MessageSquare,
  Radar,
  ShieldCheck,
  Zap,
} from "lucide-react"
import { SiteHeader } from "@/components/layout/SiteHeader"
import { SiteFooter } from "@/components/layout/SiteFooter"
import { RepoForm } from "@/components/home/RepoForm"
import { RepoCard } from "@/components/home/RepoCard"
import { HeroPreview } from "@/components/home/HeroPreview"
import { MazeIcon } from "@/components/icons/MazeIcon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAppTheme } from "@/hooks/ThemeProvider"
import { api } from "@/lib/api"
import type { Repo } from "@/lib/types"
import { useI18n } from "@/i18n"

const features = [
  { icon: GitBranch, key: "feature1", step: "I" },
  { icon: Radar, key: "feature2", step: "II" },
  { icon: MessageSquare, key: "feature3", step: "III" },
]

export function HomePage() {
  const { theme, toggle } = useAppTheme()
  const { t } = useI18n()
  const [repos, setRepos] = useState<Repo[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    api
      .listRepos()
      .then((data) => {
        if (alive) setRepos(data)
      })
      .catch((err: unknown) => {
        if (!alive) return
        setError(err instanceof Error ? err.message : "Could not reach the API")
        setRepos([])
      })
    return () => {
      alive = false
    }
  }, [])

  return (
    <div className="min-h-dvh">
      <SiteHeader />

      <main>
        <section className="relative overflow-hidden border-b border-border/60">
          <div className="pointer-events-none absolute inset-0 bg-contour opacity-70 [mask-image:radial-gradient(ellipse_75%_65%_at_60%_0%,black,transparent)]" />
          <div className="pointer-events-none absolute -left-40 top-[-12rem] size-[34rem] rounded-full bg-amber-500/10 blur-[120px]" />
          <div className="pointer-events-none absolute -right-32 top-10 size-[28rem] rounded-full bg-orange-700/10 blur-[110px]" />
          <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/35 to-transparent" />

          <div className="relative mx-auto grid max-w-6xl items-center gap-14 px-4 pb-20 pt-16 sm:px-6 sm:pt-24 lg:grid-cols-[1.08fr_0.92fr] lg:gap-10 lg:pb-24">
            <div className="animate-fade-up">
              <div className="mb-5 flex flex-wrap items-center gap-2">
                <Badge variant="outline" className="gap-1.5 border-primary/30 bg-primary/5 py-1 font-mono text-[9px] uppercase tracking-[0.18em] text-primary">
                  <span className="size-1.5 rounded-full bg-primary" />
                  {t("home.badge")}
                </Badge>
              </div>

              <h1 className="font-serif text-[2.9rem] font-medium leading-[0.98] tracking-[-0.02em] sm:text-[4.4rem]">
                {t("home.titleA")}
                <br />
                {t("home.titleB")} <span className="italic text-gradient">{t("home.titleC")}</span>
              </h1>
              <p className="mt-6 max-w-xl text-[15px] leading-relaxed text-muted-foreground sm:text-base">
                {t("home.subtitle")}
              </p>

              <div className="mt-8 max-w-xl">
                <div className="mb-2 flex items-center gap-3">
                  <span className="section-label">{t("home.intake")}</span>
                  <span className="gold-hairline flex-1" />
                </div>
                <div id="analyze" className="scroll-mt-24">
                  <RepoForm autoFocus />
                </div>
              </div>

              <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-2 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                <span className="inline-flex items-center gap-1.5">
                  <ShieldCheck className="size-3.5 text-primary" /> {t("home.trust1")}
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <Zap className="size-3.5 text-primary" /> {t("home.trust2")}
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <GitBranch className="size-3.5 text-primary" /> {t("home.trust3")}
                </span>
              </div>
            </div>

            <HeroPreview />
          </div>
        </section>

        <section id="how-it-works" className="mx-auto max-w-6xl scroll-mt-20 px-4 py-20 sm:px-6">
          <div className="mx-auto mb-10 max-w-2xl text-center">
            <p className="section-label">{t("home.process.label")}</p>
            <h2 className="mt-3 font-serif text-3xl font-medium tracking-tight sm:text-4xl">
              {t("home.process.title")}
            </h2>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
              {t("home.process.body")}
            </p>
          </div>

          <div className="grid gap-4 md:grid-cols-3">
            {features.map((feature) => (
              <div
                key={feature.key}
                className="frame-corners group relative overflow-hidden rounded-sm border border-border/70 bg-card/40 p-6 transition-all hover:border-primary/25 hover:bg-card/70"
              >
                <span className="absolute right-5 top-4 font-serif text-2xl italic text-primary/30">
                  {feature.step}
                </span>
                <div className="mb-5 grid size-10 place-items-center rounded-sm border border-primary/25 bg-primary/5 text-primary">
                  <feature.icon className="size-[18px]" />
                </div>
                <h3 className="font-serif text-xl font-medium tracking-tight">
                  {t(`${feature.key}.title`)}
                </h3>
                <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">
                  {t(`${feature.key}.body`)}
                </p>
              </div>
            ))}
          </div>
        </section>

        <section id="archive" className="scroll-mt-20 border-t border-border/60 bg-muted/10">
          <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
            <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
              <div>
                <p className="section-label mb-2">{t("archive.label")}</p>
                <h2 className="font-serif text-2xl font-medium tracking-tight">
                  {t("archive.title")}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {repos === null
                    ? t("archive.loading")
                    : repos.length === 0
                      ? t("archive.empty")
                      : t("archive.count", { count: repos.length })}
                </p>
              </div>
              <Button variant="outline" size="sm" asChild>
                <a href="#analyze">
                  {t("archive.new")} <ArrowUpRight />
                </a>
              </Button>
            </div>

            {error && (
              <div className="mb-4 flex items-start gap-2 rounded-xl border border-rose-500/20 bg-rose-500/5 p-3 text-xs text-rose-500">
                <CircleAlert className="mt-0.5 size-4 shrink-0" />
                <div>
                  <p className="font-medium">{t("archive.apiError")}</p>
                  <p className="mt-0.5 text-rose-500/80">
                    {t("archive.apiHint", { error })}
                  </p>
                </div>
              </div>
            )}

            {repos === null ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} className="h-48 rounded-2xl" />
                ))}
              </div>
            ) : repos.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-border bg-card/20 px-6 py-16 text-center">
                <div className="mx-auto mb-4 grid size-12 place-items-center rounded-2xl border border-border/70 bg-card">
                  <GitBranch className="size-5 text-muted-foreground/60" />
                </div>
                <p className="font-medium">{t("archive.emptyTitle")}</p>
                <p className="mx-auto mt-1 max-w-sm text-sm leading-relaxed text-muted-foreground">
                  {t("archive.emptyBody")}
                </p>
                <Button variant="outline" size="sm" className="mt-4" asChild>
                  <a href="#analyze">
                    {t("archive.emptyAction")} <ArrowRight />
                  </a>
                </Button>
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {repos.map((repo) => (
                  <RepoCard
                    key={repo.id}
                    repo={repo}
                    onDeleted={(id) => setRepos((prev) => prev?.filter((r) => r.id !== id) ?? [])}
                    onChanged={() => {
                      void api.listRepos().then(setRepos).catch(() => undefined)
                    }}
                  />
                ))}
              </div>
            )}
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
          <div className="frame-corners relative overflow-hidden rounded-sm border border-primary/20 bg-gradient-to-br from-amber-500/10 via-orange-800/5 to-background px-6 py-14 text-center sm:px-12">
            <div className="pointer-events-none absolute inset-0 bg-contour opacity-50 [mask-image:radial-gradient(ellipse_at_center,black,transparent_75%)]" />
            <div className="relative">
              <div className="mx-auto mb-4 grid size-12 place-items-center rounded-full border border-primary/25 bg-primary/5">
                <MazeIcon className="size-7 text-primary" />
              </div>
              <p className="section-label mb-2">{t("cta.label")}</p>
              <h2 className="font-serif text-3xl font-medium tracking-tight sm:text-4xl">
                {t("cta.title")}
              </h2>
              <p className="mx-auto mt-3 max-w-md text-sm leading-relaxed text-muted-foreground">
                {t("cta.body")}
              </p>
              <Button
                size="lg"
                className="mt-7"
                onClick={() => document.getElementById("analyze")?.scrollIntoView({ behavior: "smooth" })}
              >
                {t("cta.action")} <ArrowRight />
              </Button>
            </div>
          </div>
        </section>
      </main>

      <SiteFooter theme={theme} onToggle={toggle} />
    </div>
  )
}
