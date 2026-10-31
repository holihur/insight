import { useEffect, useState, type ReactNode } from "react"
import { ArrowDownLeft, ArrowUpRight, FileCode2, GitBranch, Loader2, TriangleAlert } from "lucide-react"
import { CallTrace } from "@/components/repo/CallTrace"
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
        <DialogHeader className="gap-1 border-b border-border/60 py-4 pl-5 pr-12">
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
                    {t("callgraph.indexed", { count: data.files ?? data.goFiles })}
                    {(data.reusedFiles ?? 0) > 0 && (
                      <> · {t("callgraph.reused", { count: data.reusedFiles ?? 0 })}</>
                    )}
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
          <Tabs
            key={data.function.id}
            defaultValue={data.callees.length > 0 ? "callees" : "callers"}
            className="min-h-0 flex-1 gap-0"
          >
            <TabsList className="mx-5 mt-3 h-8 w-fit">
              <TabsTrigger value="callees" className="gap-1 text-[11px]">
                <ArrowUpRight className="size-3" /> {t("callgraph.calls")} ({data.callees.length})
              </TabsTrigger>
              <TabsTrigger value="callers" className="gap-1 text-[11px]">
                <ArrowDownLeft className="size-3" /> {t("callgraph.callers")} ({data.callers.length})
              </TabsTrigger>
              <TabsTrigger value="trace" className="gap-1 text-[11px]">
                <GitBranch className="size-3" /> {t("callgraph.trace")}
              </TabsTrigger>
            </TabsList>
            <p className="px-5 pb-1 pt-2 text-[10px] leading-relaxed text-muted-foreground">
              {t("callgraph.legend")}
            </p>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-2">
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
              <TabsContent value="trace" className="mt-0">
                <CallTrace
                  repoId={repoId}
                  functionId={data.function.id}
                  onNavigate={(path, line) => jump(path, line)}
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
    return <p className="py-8 text-center text-xs leading-relaxed text-muted-foreground">{empty}</p>
  }
  return (
    <div className="space-y-3">
      {groupByFile(links).map((group) => (
        <section key={group.path}>
          <div className="sticky top-0 z-[1] flex items-center gap-1.5 bg-background/95 py-1 backdrop-blur">
            <FileCode2 className="size-3 shrink-0 text-muted-foreground/70" />
            <span className="min-w-0 flex-1 truncate font-mono text-[10px] text-muted-foreground">
              {group.path}
            </span>
            <span className="shrink-0 rounded-full bg-muted px-1.5 font-mono text-[9px] text-muted-foreground">
              {group.links.length}
            </span>
          </div>
          <ul className="space-y-0.5">
            {group.links.map((link, index) => (
              <li key={`${link.node.id}-${link.line}-${index}`}>
                <button
                  type="button"
                  onClick={() => onJump(link.path, link.line)}
                  className="group flex w-full items-center gap-2 rounded-sm border border-transparent px-2 py-1.5 text-left transition-colors hover:border-primary/40 hover:bg-primary/5"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-mono text-[11px] group-hover:text-primary">
                      {link.node.name}
                    </span>
                  </span>
                  <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                    :{link.line}
                  </span>
                  {confidenceLabel(link.confidence)}
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

function groupByFile(links: CallLink[]): Array<{ path: string; links: CallLink[] }> {
  const order: string[] = []
  const groups = new Map<string, CallLink[]>()
  for (const link of links) {
    const list = groups.get(link.path)
    if (list) {
      list.push(link)
      continue
    }
    groups.set(link.path, [link])
    order.push(link.path)
  }
  return order.map((path) => ({ path, links: groups.get(path) ?? [] }))
}
