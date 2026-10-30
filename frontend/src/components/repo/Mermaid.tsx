import { useEffect, useId, useRef, useState } from "react"
import { AlertCircle, Loader2 } from "lucide-react"

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

export function Mermaid({ chart }: { chart: string }) {
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
    </div>
  )
}
