import { GitHubIcon } from "@/components/icons/GitHubIcon"
import { Logo } from "@/components/layout/Logo"
import { ThemeToggle } from "@/components/layout/ThemeToggle"
import { Button } from "@/components/ui/button"
import type { Theme } from "@/hooks/useTheme"

export function SiteHeader({
  theme,
  onToggle,
}: {
  theme: Theme
  onToggle: () => void
}) {
  return (
    <header className="sticky top-0 z-40 border-b border-border/60 bg-background/75 backdrop-blur-xl">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
        <Logo />
        <nav className="flex items-center gap-1">
          <a
            href="#how-it-works"
            className="hidden px-3 py-1.5 font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-primary sm:block"
          >
            The Process
          </a>
          <a
            href="#archive"
            className="hidden px-3 py-1.5 font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-primary sm:block"
          >
            The Archive
          </a>
          <ThemeToggle theme={theme} onToggle={onToggle} />
          <Button variant="outline" size="sm" className="ml-1 gap-1.5 border-primary/25 font-mono text-[10px] uppercase tracking-[0.12em] hover:border-primary/50" asChild>
            <a
              href="https://github.com"
              target="_blank"
              rel="noreferrer"
            >
              <GitHubIcon className="size-4" />
              <span className="hidden sm:inline">GitHub</span>
            </a>
          </Button>
        </nav>
      </div>
    </header>
  )
}
