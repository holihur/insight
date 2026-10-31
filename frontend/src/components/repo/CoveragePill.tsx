import { cn } from "@/lib/utils"
import type { CoverageNode } from "@/lib/types"

export function CoveragePill({
  node,
  showCounts = false,
  className,
}: {
  node?: CoverageNode | null
  showCounts?: boolean
  className?: string
}) {
  if (!node || node.total === 0) return null
  const tone =
    node.percent >= 80
      ? "text-emerald-500"
      : node.percent >= 40
        ? "text-amber-500"
        : "text-muted-foreground"
  const title = `${node.reviewed}/${node.total} functions reviewed · ${node.percent}%`
  return (
    <span
      title={title}
      className={cn("shrink-0 font-mono text-[9px] tabular-nums", tone, className)}
    >
      {showCounts && (
        <span className="mr-0.5 text-muted-foreground/70">
          {node.reviewed}/{node.total}
        </span>
      )}
      {node.percent}%
    </span>
  )
}
