import { useEffect, useId, useMemo, useRef, useState } from "react"
import { AlertCircle, ArrowRight, FileCode2, Loader2 } from "lucide-react"
import { api } from "@/lib/api"
import { extractDiagramRefs } from "@/lib/mermaidRefs"
import type { ResolveMatch } from "@/lib/types"
import { useI18n } from "@/i18n"

type MermaidAPI = {
  initialize: (config: Record<string, unknown>) => void
  render: (id: string, text: string) => Promise<{ svg: string }>
}

let mermaidPromise: Promise<MermaidAPI> | null = null

function loadMermaid(): Promise<MermaidAPI> {
  if (!mermaidPromise) {
    mermaidPromise = import("mermaid").then((mod) => mod.default as unknown as MermaidAPI)
  }
  return mermaidPromise
}

const resolveCache = new Map<string, ResolveMatch[]>()

export function Mermaid({
  chart,
  repoId,
  onOpenFile,
}: {
  chart: string
  repoId?: string
  onOpenFile?: (path: string, lines?: [number, number]) => void
}) {
  const { t } = useI18n()
  const [matches, setMatches] = useState<ResolveMatch[]>([])
  const refs = useMemo(() => extractDiagramRefs(chart), [chart])

  useEffect(() => {
    if (!repoId || refs.length === 0) {
      setMatches([])
      return
    }
    const key = `${repoId}\u0000${chart}`
    const cached = resolveCache.get(key)
    if (cached) {
      setMatches(cached)
      return
    }
    let alive = true
    api
      .resolve(repoId, refs.map((ref) => ref.label))
      .then((res) => {
        resolveCache.set(key, res.matches)
        if (alive) setMatches(res.matches)
      })
      .catch(() => {
        if (alive) setMatches([])
      })
    return () => {
      alive = false
    }
  }, [repoId, chart, refs])
  const id = useId().replace(/[^a-zA-Z0-9]/g, "")
  const container = useRef<HTMLDivElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [dark, setDark] = useState(() =>
    typeof document === "undefined" ? true : document.documentElement.classList.contains("dark"),
  )

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setDark(document.documentElement.classList.contains("dark"))
    })
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] })
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    loadMermaid()
      .then(async (mermaid) => {
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "base",
          fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
          themeVariables: {
            background: "transparent",
            primaryColor: dark ? "#2a2116" : "#f1e4c8",
            primaryTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            primaryBorderColor: dark ? "#d9b45c" : "#9a6b0f",
            lineColor: dark ? "#a89a82" : "#8a7558",
            secondaryColor: dark ? "#3a271b" : "#ead6b3",
            secondaryBorderColor: dark ? "#b4552d" : "#a3542e",
            tertiaryColor: dark ? "#211a13" : "#f7efdf",
            tertiaryBorderColor: dark ? "#5c4b36" : "#cdbb9d",
            noteBkgColor: dark ? "#211a13" : "#f7efdf",
            noteTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            noteBorderColor: dark ? "#5c4b36" : "#cdbb9d",
            actorBkg: dark ? "#2a2116" : "#f1e4c8",
            actorBorder: dark ? "#d9b45c" : "#9a6b0f",
            actorTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            signalColor: dark ? "#a89a82" : "#8a7558",
            signalTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            labelBoxBkgColor: dark ? "#2a2116" : "#f1e4c8",
            labelBoxBorderColor: dark ? "#d9b45c" : "#9a6b0f",
            labelTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            loopTextColor: dark ? "#e8ddc9" : "#3b2e1e",
            fontSize: "13px",
          },
        })
        const { svg } = await mermaid.render(`mermaid-${id}-${dark ? "d" : "l"}`, chart)
        if (cancelled) return
        if (container.current) container.current.innerHTML = svg
        setLoading(false)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(err instanceof Error ? err.message : "Could not render diagram")
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [chart, dark, id])

  if (error) {
    return (
      <div className="my-4 flex items-start gap-2 rounded-sm border border-primary/25 bg-primary/5 p-3 text-xs text-primary">
        <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
        <div>
          <p className="font-medium">Diagram could not be rendered</p>
          <pre className="mt-1 overflow-x-auto whitespace-pre-wrap font-mono opacity-80">{chart}</pre>
        </div>
      </div>
    )
  }

  return (
    <div className="mermaid-block relative min-h-24">
      {loading && (
        <div className="absolute inset-0 grid place-items-center">
          <div className="flex items-center gap-2 font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin text-primary" />
            Rendering diagram
          </div>
        </div>
      )}
      <div ref={container} className={loading ? "opacity-0" : "opacity-100 transition-opacity"} />
      {matches.length > 0 && (
        <div className="mt-2 rounded-sm border border-border/60 bg-card/35 p-2">
          <div className="mb-1.5 flex items-center gap-1.5 font-mono text-[9px] uppercase tracking-[0.14em] text-muted-foreground">
            <FileCode2 className="size-3" />
            {t("wiki.relatedCode")}
            <span className="rounded-full bg-muted px-1.5 py-0.5 tracking-normal">{matches.length}</span>
          </div>
          <ul className="space-y-1">
            {matches.map((match) => (
              <li key={match.query} className="flex flex-wrap items-center gap-1.5 text-[10px]">
                <span
                  className="max-w-[42%] truncate font-mono text-muted-foreground"
                  title={match.query}
                >
                  {match.query}
                </span>
                <ArrowRight className="size-3 shrink-0 text-muted-foreground/50" />
                <span className="flex min-w-0 flex-wrap gap-1">
                  {match.candidates.slice(0, 4).map((candidate) => (
                    <button
                      key={candidate.id || `${candidate.path}:${candidate.line}`}
                      type="button"
                      onClick={() => onOpenFile?.(candidate.path, [candidate.line, candidate.line])}
                      title={`${candidate.path}:${candidate.line}`}
                      className="inline-flex max-w-[220px] items-center gap-1 rounded-sm border border-primary/25 bg-primary/5 px-1.5 py-0.5 font-mono text-primary transition-colors hover:bg-primary/15"
                    >
                      <span className="truncate">{candidate.name}</span>
                      <span className="shrink-0 opacity-60">:{candidate.line}</span>
                    </button>
                  ))}
                  {match.candidates.length > 4 && (
                    <span className="self-center font-mono text-[9px] text-muted-foreground">
                      +{match.candidates.length - 4}
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
