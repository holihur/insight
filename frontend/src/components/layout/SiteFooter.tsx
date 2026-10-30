import { Logo } from "@/components/layout/Logo"

export function SiteFooter() {
  return (
    <footer className="border-t border-border/60">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-4 py-8 text-sm text-muted-foreground sm:flex-row sm:px-6">
        <div className="flex items-center gap-3">
          <Logo />
          <span className="text-border">/</span>
          <span className="font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/80">
            The maze is a journey inward.
          </span>
        </div>
        <div className="flex items-center gap-4 font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/80">
          <span>Grounded in source</span>
        </div>
      </div>
    </footer>
  )
}
