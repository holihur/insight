import { useEffect, useState, type ReactNode } from "react"
import { ArrowDownLeft, ArrowUpRight, Loader2, TriangleAlert } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { api } from "@/lib/api"
import type { CallLink, CallResult } from "@/lib/types"
import { useI18n } from "@/i18n"

export function CallGraphDialog({
  repoId,
  functionId,
  open,
  onOpenChange,
  onNavigate,
}: {
  repoId: string
  functionId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onNavigate: (path: string, line: number) => void
}) {
  const { t } = useI18n()
  const [data, setData] = useState<CallResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open || !functionId) return
    let alive = true
    setLoading(true)
    setError(null)
    setData(null)
    api
      .getCalls(repoId, functionId)
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
  }, [repoId, functionId, open, t])

  const jump = (path: string, line: number) => {
    onOpenChange(false)
    onNavigate(path, line)
  }

  const confidenceLabel = (value?: string) => {
    if (!value) return null
    const key =
      value === "exact"
        ? "callgraph.exact"
        : value === "multiple"
          ? "callgraph.ambiguous"
          : value === "local" || value === "receiver"
            ? "callgraph.exact"
            : "callgraph.fuzzy"
    return (
      <Badge variant={value === "multiple" ? "warning" : "outline"} className="text-[9px]">
        {t(key)}
      </Badge>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85dvh] flex-col gap-0 p-0 max-sm:bottom-0 max-sm:top-auto max-sm:max-h-[80dvh] max-sm:w-full max-sm:max-w-none max-sm:translate-y-0 max-sm:rounded-b-none sm:max-w-xl">
        <DialogHeader className="gap-1 border-b border-border/60 px-5 py-4">
          <DialogTitle className="flex items-center gap-2 font-mono text-sm">
            {data?.function.name ?? functionId ?? t("callgraph.title")}
            {data?.function && (
              <Badge variant="outline" className="text-[9px]">
                {data.function.kind === "method" ? t("callgraph.method") : t("callgraph.function")}
              </Badge>
            )}
            <span className="ml-auto hidden font-mono text-[9px] font-normal uppercase tracking-[0.14em] text-muted-foreground sm:inline">
              {t("callgraph.title")}
            </span>
          </DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-2 font-mono text-[10px]">
            {data ? (
              <>
                <span className="truncate">
                  {data.function.path}:{data.function.startLine}
                </span>
                <Badge variant="outline" className="text-[9px]">
                  {data.commit || "—"}
                </Badge>
                {data.indexed && (
                  <span className="text-muted-foreground">
                    {t("callgraph.indexed", { count: data.goFiles })}
                  </span>
                )}
              </>
            ) : (
              <span>{t("callgraph.loading")}</span>
            )}
          </DialogDescription>
        </DialogHeader>

        {loading && (
          <div className="flex items-center gap-2 px-5 py-8 text-xs text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" /> {t("callgraph.loading")}
          </div>
        )}

        {!loading && error && (
          <div className="flex items-start gap-2 px-5 py-6 text-xs text-amber-500">
            <TriangleAlert className="mt-0.5 size-3.5 shrink-0" /> {error}
          </div>
        )}

        {!loading && !error && data && !data.indexed && (
          <div className="px-5 py-6 text-xs text-muted-foreground">{t("callgraph.goOnly")}</div>
        )}

        {!loading && !error && data && data.indexed && (
          <Tabs defaultValue="callees" className="min-h-0 flex-1 gap-0">
            <TabsList className="mx-5 mt-3 h-8 w-fit">
              <TabsTrigger value="callees" className="gap-1 text-[11px]">
                <ArrowUpRight className="size-3" /> {t("callgraph.calls")} ({data.callees.length})
              </TabsTrigger>
              <TabsTrigger value="callers" className="gap-1 text-[11px]">
                <ArrowDownLeft className="size-3" /> {t("callgraph.callers")} ({data.callers.length})
              </TabsTrigger>
            </TabsList>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
              <TabsContent value="callees" className="mt-0">
                <LinkList
                  links={data.callees}
                  empty={t("callgraph.noCalls")}
                  onJump={jump}
                  confidenceLabel={confidenceLabel}
                />
              </TabsContent>
              <TabsContent value="callers" className="mt-0">
                <LinkList
                  links={data.callers}
                  empty={t("callgraph.noCallers")}
                  onJump={jump}
                  confidenceLabel={confidenceLabel}
                />
              </TabsContent>
            </div>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}

function LinkList({
  links,
  empty,
  onJump,
  confidenceLabel,
}: {
  links: CallLink[]
  empty: string
  onJump: (path: string, line: number) => void
  confidenceLabel: (value?: string) => ReactNode
}) {
  if (!links.length) {
    return <p className="py-8 text-center text-xs text-muted-foreground">{empty}</p>
  }
  return (
    <ul className="space-y-1">
      {links.map((link, index) => (
        <li key={`${link.node.id}-${link.line}-${index}`}>
          <button
            type="button"
            onClick={() => onJump(link.path, link.line)}
            className="group flex w-full items-center gap-2 rounded-sm border border-border/40 px-2.5 py-2 text-left transition-colors hover:border-primary/40 hover:bg-primary/5"
          >
            <span className="min-w-0 flex-1">
              <span className="block truncate font-mono text-[11px] group-hover:text-primary">
                {link.node.name}
              </span>
              <span className="block truncate font-mono text-[10px] text-muted-foreground">
                {link.path}:{link.line}
              </span>
            </span>
            {confidenceLabel(link.confidence)}
          </button>
        </li>
      ))}
    </ul>
  )
}
