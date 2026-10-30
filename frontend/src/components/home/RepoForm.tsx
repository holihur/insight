import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { ArrowRight, Loader2 } from "lucide-react"
import { GitHubIcon } from "@/components/icons/GitHubIcon"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const examples = [
  "gin-gonic/gin",
  "vercel/next.js",
  "facebook/react",
  "rust-lang/rust",
]

function looksValid(value: string): boolean {
  const v = value.trim()
  if (!v) return false
  if (/^[\w.-]+\/[\w.-]+$/.test(v)) return true
  try {
    const parsed = new URL(v)
    return (parsed.protocol === "http:" || parsed.protocol === "https:") && parsed.pathname.length > 1
  } catch {
    return v.startsWith("git@")
  }
}

export function RepoForm({ autoFocus = false }: { autoFocus?: boolean }) {
  const [value, setValue] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    const url = value.trim()
    if (!looksValid(url)) {
      setError("Enter a GitHub URL or an owner/repo pair, e.g. gin-gonic/gin")
      return
    }
    setBusy(true)
    setError(null)
    try {
      const repo = await api.createRepo(url)
      toast.success("Analysis started", {
        description: `${repo.owner}/${repo.name} is being cloned and indexed.`,
      })
      navigate(`/repo/${repo.id}`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "Could not start the analysis"
      setError(message)
      toast.error(message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="w-full">
      <form
        onSubmit={submit}
        className={cn(
          "group flex flex-col gap-1.5 rounded-sm border border-border/90 bg-card/70 p-1.5 shadow-[0_18px_50px_-24px_rgba(0,0,0,0.7)] backdrop-blur transition-shadow focus-within:border-primary/50 focus-within:shadow-[0_18px_60px_-20px_rgba(217,180,92,0.25)] sm:flex-row sm:items-center",
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
            placeholder="https://github.com/owner/repo"
            aria-label="GitHub repository"
            className="h-11 w-full min-w-0 bg-transparent font-mono text-[12px] tracking-wide outline-none placeholder:text-muted-foreground/50"
          />
        </div>
        <Button type="submit" size="lg" disabled={busy} className="h-11 shrink-0 rounded-sm px-5 font-mono text-[11px] uppercase tracking-[0.12em]">
          {busy ? <Loader2 className="animate-spin" /> : null}
          {busy ? "Starting…" : "Analyze repo"}
          {!busy && <ArrowRight />}
        </Button>
      </form>

      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <span className="mr-1 font-mono text-[9px] uppercase tracking-[0.18em] text-muted-foreground/70">Access</span>
        {examples.map((example) => (
          <button
            key={example}
            type="button"
            onClick={() => {
              setValue(example)
              setError(null)
            }}
            className="rounded-sm border border-border/70 bg-background/40 px-2 py-0.5 font-mono text-[10px] text-muted-foreground transition-colors hover:border-primary/40 hover:bg-primary/5 hover:text-primary"
          >
            {example}
          </button>
        ))}
      </div>
      {error && <p className="mt-2 text-xs text-rose-500">{error}</p>}
    </div>
  )
}
