import { useEffect, useState } from "react"
import { ChevronDown, ChevronRight, GitBranch, Loader2, RotateCcw, TriangleAlert } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { api } from "@/lib/api"
import { useI18n } from "@/i18n"
import type { TraceNode, TraceResult } from "@/lib/types"

export function CallTrace({
  repoId,
  functionId,
  onNavigate,
}: {
  repoId: string
  functionId: string
  onNavigate: (path: string, line: number) => void
}) {
  const { t } = useI18n()
  const [direction, setDirection] = useState<"callees" | "callers">("callees")
  const [data, setData] = useState<TraceResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Set<string>>(new Set(["root"]))

  useEffect(() => {
    let alive = true
    setLoading(true)
    setError(null)
    setData(null)
    setExpanded(new Set(["root"]))
    api
      .getTrace(repoId, functionId, direction)
      .then((res) => {
        if (alive) setData(res)
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : t("callgraph.error"))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [repoId, functionId, direction, t])

  const toggle = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
      <div className="mb-2 flex items-center gap-1">
        <span className="mr-1 inline-flex items-center gap-1 font-mono text-[9px] uppercase tracking-[0.14em] text-muted-foreground">
          <GitBranch className="size-3" /> {t("callgraph.trace")}
        </span>
        <DirectionButton active={direction === "callees"} onClick={() => setDirection("callees")}>
          {t("callgraph.outgoing")}
        </DirectionButton>
        <DirectionButton active={direction === "callers"} onClick={() => setDirection("callers")}>
          {t("callgraph.incoming")}
        </DirectionButton>
      </div>

      {loading && (
        <div className="flex items-center gap-2 py-6 text-xs text-muted-foreground">
          <Loader2 className="size-3.5 animate-spin" /> {t("callgraph.loading")}
        </div>
      )}
      {!loading && error && (
        <div className="flex items-start gap-2 py-4 text-xs text-amber-500">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" /> {error}
        </div>
      )}
      {!loading && !error && data?.root && (
        <TraceTree
          node={data.root}
          keyPath="root"
          depth={0}
          expanded={expanded}
          onToggle={toggle}
          onNavigate={onNavigate}
          empty={
            direction === "callees" ? t("callgraph.noCalls") : t("callgraph.noCallers")
          }
        />
      )}
    </div>
  )
}

function DirectionButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        active
          ? "rounded-sm border border-primary/40 bg-primary/10 px-2 py-0.5 font-mono text-[10px] text-primary"
          : "rounded-sm border border-transparent px-2 py-0.5 font-mono text-[10px] text-muted-foreground hover:bg-muted"
      }
    >
      {children}
    </button>
  )
}

function TraceTree({
  node,
  keyPath,
  depth,
  expanded,
  onToggle,
  onNavigate,
  empty,
}: {
  node: TraceNode
  keyPath: string
  depth: number
  expanded: Set<string>
  onToggle: (key: string) => void
  onNavigate: (path: string, line: number) => void
  empty: string
}) {
  const { t } = useI18n()
  const children = node.children ?? []
  const hasChildren = children.length > 0
  const isExpanded = expanded.has(keyPath)

  return (
    <div>
      <div className="flex items-center gap-1 py-0.5" style={{ paddingLeft: depth * 14 }}>
        <button
          type="button"
          onClick={() => onToggle(keyPath)}
          disabled={!hasChildren}
          aria-label={isExpanded ? "collapse" : "expand"}
          className="grid size-5 shrink-0 place-items-center rounded-sm text-muted-foreground/60 hover:bg-muted hover:text-foreground disabled:opacity-30"
        >
          {hasChildren ? (
            isExpanded ? (
              <ChevronDown className="size-3" />
            ) : (
              <ChevronRight className="size-3" />
            )
          ) : (
            <span className="size-3" />
          )}
        </button>
        <button
          type="button"
          onClick={() => onNavigate(node.path, node.line)}
          className="group flex min-w-0 flex-1 items-center gap-2 rounded-sm px-1.5 py-1 text-left hover:bg-primary/5"
          title={`${node.path}:${node.line}`}
        >
          <span className="min-w-0 flex-1 truncate font-mono text-[11px] group-hover:text-primary">
            {node.function.name}
          </span>
          <span className="hidden shrink-0 font-mono text-[10px] text-muted-foreground sm:inline">
            {node.path}:{node.line}
          </span>
        </button>
        {node.cycle && (
          <Badge variant="warning" className="gap-1 text-[9px]" title={t("callgraph.cycle")}>
            <RotateCcw className="size-2.5" /> {t("callgraph.cycle")}
          </Badge>
        )}
        {node.confidence === "multiple" && (
          <Badge variant="outline" className="text-[9px]">
            {t("callgraph.ambiguous")}
          </Badge>
        )}
      </div>

      {node.truncated && (
        <div
          className="py-0.5 font-mono text-[9px] text-muted-foreground"
          style={{ paddingLeft: depth * 14 + 26 }}
        >
          {t("callgraph.more")}
        </div>
      )}

      {isExpanded && hasChildren
        ? children.map((child, index) => (
            <TraceTree
              key={`${keyPath}/${child.function.id}:${child.line}:${index}`}
              node={child}
              keyPath={`${keyPath}/${child.function.id}:${child.line}:${index}`}
              depth={depth + 1}
              expanded={expanded}
              onToggle={onToggle}
              onNavigate={onNavigate}
              empty={empty}
            />
          ))
        : null}
      {!hasChildren && depth === 0 && (
        <p className="py-6 text-center text-xs text-muted-foreground">{empty}</p>
      )}
    </div>
  )
}
