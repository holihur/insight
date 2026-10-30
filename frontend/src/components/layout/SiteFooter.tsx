import { Logo } from "@/components/layout/Logo"
import { LanguageSwitcher } from "@/components/layout/LanguageSwitcher"
import { ThemeToggle } from "@/components/layout/ThemeToggle"
import { useI18n } from "@/i18n"
import type { Theme } from "@/hooks/useTheme"

export function SiteFooter({
  theme,
  onToggle,
}: {
  theme: Theme
  onToggle: () => void
}) {
  const { t } = useI18n()
  return (
    <footer className="border-t border-border/60">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-4 py-8 text-sm text-muted-foreground sm:flex-row sm:px-6">
        <div className="flex items-center gap-3">
          <Logo />
          <span className="text-border">/</span>
          <span className="font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/80">
            {t("footer.tagline")}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="mr-2 hidden font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/80 sm:inline">
            {t("footer.right")}
          </span>
          <LanguageSwitcher />
          <ThemeToggle theme={theme} onToggle={onToggle} />
        </div>
      </div>
    </footer>
  )
}
