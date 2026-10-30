import { Link } from "react-router-dom"
import { MazeIcon } from "@/components/icons/MazeIcon"
import { cn } from "@/lib/utils"

export function Logo({ className, compact = false }: { className?: string; compact?: boolean }) {
  return (
    <Link
      to="/"
      className={cn("group inline-flex items-center gap-2.5", className)}
      aria-label="Code Insight home"
    >
      <span className="relative grid size-8 place-items-center">
        <span className="absolute inset-0 rounded-full bg-primary/15 blur-md transition-opacity group-hover:opacity-100" />
        <MazeIcon className="relative size-8 text-primary transition-transform duration-500 group-hover:rotate-180" />
      </span>
      {!compact && (
        <span className="font-serif text-[19px] font-semibold tracking-[0.01em]">
          Code <span className="italic text-primary">Insight</span>
        </span>
      )}
    </Link>
  )
}
