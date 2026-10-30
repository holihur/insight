import { useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { ArrowRight, CheckCircle2, Loader2, Sparkles } from "lucide-react"
import { GitHubIcon } from "@/components/icons/GitHubIcon"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n"

const examples = [
  "gin-gonic/gin",
  "vercel/next.js",
  "facebook/react",
  "rust-lang/rust",
]

export function normalizeRepoInput(raw: string): {
  normalized: string | null
  display: string | null
  valid: boolean
} {
  const v = raw.trim()
  if (!v) return { normalized: null, display: null, valid: false }
  // git@github.com:owner/repo.git
  if (v.startsWith("git@")) {
    const m = /^git@[^:]+:([^/\s]+\/[^/\s]+)(\.git)?\/?$/.exec(v.trim())
    if (m) {
      const clean = m[1].replace(/\.git$/, "")
      return { normalized: clean, display: clean, valid: true }
    }
    return { normalized: null, display: null, valid: false }
  }
  // owner/repo shorthand
  if (/^[\w.\-]+\/[\w.\-]+(\/)?$/.test(v) && !v.includes(" ") && !v.includes(":")) {
    const clean = v.replace(/\.git\/?$/, "").replace(/\/$/, "")
    return { normalized: clean, display: clean, valid: true }
  }
  // http(s) URL - extract owner/repo
  try {
    const parsed = new URL(v)
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:")
      return { normalized: null, display: null, valid: false }
    let path = parsed.pathname.replace(/^\/+/, "").replace(/\.git\/?$/, "").replace(/\/$/, "")
    // handle github tree/blob etc: owner/repo/tree/branch/... -> owner/repo
    const parts = path.split("/").filter(Boolean)
    if (parts.length >= 2) {
      const display = `${parts[0]}/${parts[1]}`
      // keep full https canonical for backend but preview is owner/repo
      const normalized = `https://${parsed.hostname}/${display}`
      return { normalized, display, valid: true }
    }
    return { normalized: null, display: null, valid: false }
  } catch {
    return { normalized: null, display: null, valid: false }
  }
}

function friendlyError(raw: string, t: (k: string) => string): string {
  const lower = raw.toLowerCase()
  if (lower.includes("not found") || lower.includes("404") || lower.includes("no repository")) {
    return `${raw} — 可能是私有仓库或地址拼写错误。私有仓库需在 .env 配置 GITHUB_TOKEN 后重试。`
  }
  if (lower.includes("already being analyzed") || lower.includes("already running")) {
    return raw
  }
  if (lower.includes("invalid") && lower.includes("url")) return t("form.invalid")
  return raw
}

export function RepoForm({ autoFocus = false }: { autoFocus?: boolean }) {
  const { t } = useI18n()
  const [value, setValue] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const parsed = useMemo(() => normalizeRepoInput(value), [value])
  const showPreview = value.trim().length > 2 && parsed.valid && parsed.display !== value.trim()

  const doSubmit = async (raw: string) => {
    if (busy) return
    const trimmed = raw.trim()
    const norm = normalizeRepoInput(trimmed)
    if (!norm.valid || !norm.normalized) {
      setError(t("form.invalid"))
      return
    }
    setBusy(true)
    setError(null)
    try {
      const repo = await api.createRepo(norm.normalized)
      toast.success(t("form.started"), {
        description: t("form.startedDesc", { repo: `${repo.owner}/${repo.name}` }),
      })
      navigate(`/repo/${repo.id}`)
    } catch (err) {
      const rawMsg = err instanceof Error ? err.message : t("form.error")
      const msg = friendlyError(rawMsg, t)
      setError(msg)
      toast.error(msg)
    } finally {
      setBusy(false)
    }
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    await doSubmit(value)
  }

  const handleExample = (example: string) => {
    setValue(example)
    setError(null)
    // Directly analyze on click — save one tap (P0 #5)
    void doSubmit(example)
  }

  return (
    <div className="w-full">
      <form
        onSubmit={submit}
        className={cn(
          "group flex flex-col gap-1.5 rounded-sm border bg-card/70 p-1.5 shadow-[0_18px_50px_-24px_rgba(0,0,0,0.7)] backdrop-blur transition-shadow focus-within:shadow-[0_18px_60px_-20px_rgba(217,180,92,0.25)] sm:flex-row sm:items-center",
          error ? "border-rose-500/40 focus-within:border-rose-500/50" : parsed.valid ? "border-emerald-500/30 focus-within:border-emerald-500/40" : "border-border/90 focus-within:border-primary/50",
        )}
      >
        <div className="flex flex-1 items-center gap-2 px-3">
          <GitHubIcon className="size-4 shrink-0 text-muted-foreground" />
          <input
            value={value}
            onChange={(event) => {
              setValue(event.target.value)
              setError(null)
            }}
            autoFocus={autoFocus}
            spellCheck={false}
            autoComplete="off"
            placeholder={t("form.placeholder")}
            aria-label="GitHub repository"
            aria-invalid={!!error}
            className="h-11 w-full min-w-0 bg-transparent font-mono text-[12px] tracking-wide outline-none placeholder:text-muted-foreground/50"
          />
          {parsed.valid && !busy && (
            <CheckCircle2 className="size-3.5 shrink-0 text-emerald-500" aria-hidden />
          )}
        </div>
        <Button type="submit" size="lg" disabled={busy || !value.trim()} className="h-11 shrink-0 rounded-sm px-5 font-mono text-[11px] uppercase tracking-[0.12em]">
          {busy ? <Loader2 className="animate-spin" /> : null}
          {busy ? t("home.starting") : t("home.analyze")}
          {!busy && <ArrowRight />}
        </Button>
      </form>
      {showPreview && parsed.display && (
        <p className="mt-1.5 flex items-center gap-1.5 font-mono text-[10px] text-emerald-600 dark:text-emerald-400">
          <Sparkles className="size-3" /> 将分析 {parsed.display}
          <span className="text-muted-foreground/50">·</span>
          <span className="text-muted-foreground">{parsed.normalized}</span>
        </p>
      )}
      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <span className="mr-1 font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/70">Access</span>
        {examples.map((example) => (
          <button
            key={example}
            type="button"
            onClick={() => handleExample(example)}
            disabled={busy}
            title="点击直接分析"
            className="rounded-sm border border-border/70 bg-background/40 px-2 py-0.5 font-mono text-[10px] text-muted-foreground transition-colors hover:border-primary/40 hover:bg-primary/5 hover:text-primary disabled:opacity-50"
          >
            {example}
          </button>
        ))}
      </div>
      {error && <p className="mt-2 text-xs leading-relaxed text-rose-500">{error}</p>}
    </div>
  )
}
