import { Link } from "react-router-dom"
import { ArrowLeft, Compass } from "lucide-react"
import { Button } from "@/components/ui/button"

export function NotFoundPage() {
  return (
    <div className="grid min-h-dvh place-items-center px-6">
      <div className="max-w-md text-center">
        <div className="mx-auto mb-4 grid size-14 place-items-center rounded-2xl border border-border/70 bg-card">
          <Compass className="size-6 text-muted-foreground/60" />
        </div>
        <p className="font-mono text-[10px] uppercase tracking-[0.24em] text-primary">404 / Off the map</p>
        <h1 className="mt-2 font-serif text-3xl font-medium tracking-tight">This corridor leads nowhere</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          The link may be outdated, or the analysis was deleted.
        </p>
        <Button className="mt-5" asChild>
          <Link to="/">
            <ArrowLeft /> Back to Code Insight
          </Link>
        </Button>
      </div>
    </div>
  )
}
