import { useEffect, useState } from "react"
import { cn } from "@/lib/utils"
import type { CoverageFunction } from "@/lib/types"

export function CodeMinimap({
  totalLines,
  functions,
  scrollRef,
  onJumpLine,
}: {
  totalLines: number
  functions: CoverageFunction[]
  scrollRef: React.RefObject<HTMLDivElement | null>
  onJumpLine: (line: number) => void
}) {
  const [viewport, setViewport] = useState({ top: 0, height: 12 })

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    const update = () => {
      const scrollable = element.scrollHeight
      if (!scrollable) return
      setViewport({
        top: (element.scrollTop / scrollable) * 100,
        height: Math.max(4, (element.clientHeight / scrollable) * 100),
      })
    }
    update()
    element.addEventListener("scroll", update, { passive: true })
    const observer = new ResizeObserver(update)
    observer.observe(element)
    return () => {
      element.removeEventListener("scroll", update)
      observer.disconnect()
    }
  }, [scrollRef, totalLines])

  if (totalLines <= 0) return null
  const safeTotal = Math.max(1, totalLines)

  return (
    <div className="relative hidden w-[74px] shrink-0 overflow-hidden border-l border-border/60 bg-card/30 md:block">
      <div className="pointer-events-none absolute left-1.5 top-2 z-10 font-mono text-[7px] uppercase tracking-[0.16em] text-muted-foreground/50">
        map
      </div>
      <div className="absolute inset-x-2 inset-y-3">
        {functions.map((fn) => {
          if (fn.startLine > safeTotal) return null
          const top = ((fn.startLine - 1) / safeTotal) * 100
          const height = Math.max(
            0.7,
            ((Math.min(fn.endLine, safeTotal) - fn.startLine + 1) / safeTotal) * 100,
          )
          return (
            <button
              key={fn.id}
              type="button"
              onClick={() => onJumpLine(fn.startLine)}
              title={`${fn.name} · L${fn.startLine}–${fn.endLine}${
                fn.reviewed ? " · reviewed" : fn.changed ? " · changed" : ""
              }`}
              className={cn(
                "absolute inset-x-0 rounded-[1px] border-l-2 transition-colors",
                fn.reviewed
                  ? "border-emerald-400/80 bg-emerald-500/45 hover:bg-emerald-500/70"
                  : fn.changed
                    ? "border-primary/70 bg-primary/30 hover:bg-primary/50"
                    : "border-border/70 bg-muted-foreground/15 hover:bg-muted-foreground/30",
              )}
              style={{ top: `${top}%`, height: `${height}%` }}
            />
          )
        })}
        <div
          className="pointer-events-none absolute inset-x-0 rounded-sm border border-primary/25 bg-primary/5"
          style={{ top: `${viewport.top}%`, height: `${viewport.height}%` }}
        />
      </div>
    </div>
  )
}
