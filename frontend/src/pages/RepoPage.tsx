import { useCallback, useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, Loader2, RefreshCw } from "lucide-react"
import { toast } from "sonner"
import { RepoHeader } from "@/components/repo/RepoHeader"
import { WikiSidebar, type RepoView } from "@/components/repo/WikiSidebar"
import { WikiView } from "@/components/repo/WikiView"
import { CodeViewer } from "@/components/repo/CodeViewer"
import { ChatPanel } from "@/components/repo/ChatPanel"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { useAppTheme } from "@/hooks/ThemeProvider"
import { useRepo } from "@/hooks/useRepo"
import { useChat } from "@/hooks/useChat"
import { api } from "@/lib/api"
import { cn, isActive } from "@/lib/utils"
import type { IndexData, TreeNode } from "@/lib/types"

export function RepoPage() {
  const { id } = useParams<{ id: string }>()
  const { theme, toggle } = useAppTheme()
  const { repo, error, loading, refresh, regenerate } = useRepo(id)
  const chat = useChat(id)

  const [view, setView] = useState<RepoView>("wiki")
  const [activeSlug, setActiveSlug] = useState("overview")
  const [activeFile, setActiveFile] = useState<string | null>(null)
  const [index, setIndex] = useState<IndexData | null>(null)
  const [indexLoading, setIndexLoading] = useState(false)
  const [indexError, setIndexError] = useState<string | null>(null)
  const [navOpen, setNavOpen] = useState(false)
  const [chatOpen, setChatOpen] = useState(false)

  const pages = useMemo(() => repo?.wiki?.pages ?? [], [repo?.wiki])
  const page = useMemo(
    () => pages.find((item) => item.slug === activeSlug) ?? pages[0] ?? null,
    [pages, activeSlug],
  )

  useEffect(() => {
    if (pages.length === 0) return
    if (!pages.some((item) => item.slug === activeSlug)) {
      setActiveSlug(pages[0].slug)
    }
  }, [pages, activeSlug])

  useEffect(() => {
    if (view !== "code" || index || indexLoading || !repo) return
    if (repo.status !== "ready") return
    setIndexLoading(true)
    setIndexError(null)
    api
      .getIndex(repo.id)
      .then((data) => setIndex(data))
      .catch((err: unknown) =>
        setIndexError(err instanceof Error ? err.message : "Could not load the source index"),
      )
      .finally(() => setIndexLoading(false))
  }, [view, index, indexLoading, repo])

  useEffect(() => {
    if (view !== "code" || activeFile || !index) return
    const first = index.entryFiles?.[0] ?? index.keyFiles?.[0] ?? firstFile(index.tree)
    if (first) setActiveFile(first)
  }, [view, activeFile, index])

  const handleRegenerate = useCallback(async () => {
    if (!repo) return
    try {
      await regenerate()
      toast.success("Regenerating wiki", {
        description: "The agent is re-reading the repository.",
      })
      setActiveSlug(pages[0]?.slug ?? "overview")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not regenerate")
    }
  }, [regenerate, repo, pages])

  const handleRetry = useCallback(async () => {
    if (!repo) return
    try {
      await regenerate()
      toast.success("Retrying analysis")
      await refresh(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not retry")
    }
  }, [regenerate, refresh, repo])

  const handleSelectFile = useCallback((path: string) => {
    setActiveFile(path)
    setView("code")
    setNavOpen(false)
  }, [])

  if (loading && !repo) {
    return <RepoLoading />
  }

  if (error && !repo) {
    return (
      <div className="grid min-h-dvh place-items-center px-6">
        <div className="max-w-md text-center">
          <div className="mx-auto mb-4 grid size-12 place-items-center rounded-2xl border border-rose-500/20 bg-rose-500/5">
            <ArrowLeft className="size-5 text-rose-500" />
          </div>
          <h1 className="text-lg font-semibold">Repository unavailable</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{error}</p>
          <div className="mt-5 flex justify-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/">Back home</Link>
            </Button>
            <Button onClick={() => void refresh()}>
              <RefreshCw /> Retry
            </Button>
          </div>
        </div>
      </div>
    )
  }

  if (!repo) return null

  const sidebar = (
    <WikiSidebar
      repo={repo}
      view={view}
      onViewChange={(next) => {
        setView(next)
        setNavOpen(false)
      }}
      pages={pages}
      activeSlug={activeSlug}
      onSelectSlug={(slug) => {
        setActiveSlug(slug)
        setView("wiki")
        setNavOpen(false)
      }}
      index={index}
      activeFile={activeFile}
      onSelectFile={handleSelectFile}
    />
  )

  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <RepoHeader
        repo={repo}
        theme={theme}
        onToggleTheme={toggle}
        onAsk={() => setChatOpen(true)}
        onRegenerate={() => void handleRegenerate()}
        onOpenNav={() => setNavOpen(true)}
      />

      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-[252px] shrink-0 border-r border-border/60 lg:block">{sidebar}</aside>

        <main
          className={cn(
            "min-w-0 flex-1",
            view === "wiki" ? "overflow-y-auto" : "flex min-h-0 flex-col",
          )}
        >
          {view === "wiki" ? (
            <WikiView
              repo={repo}
              page={page}
              onNavigate={setActiveSlug}
              onRetry={() => void handleRetry()}
            />
          ) : indexError ? (
            <div className="m-6 rounded-xl border border-rose-500/20 bg-rose-500/5 p-4 text-sm text-rose-500">
              {indexError}
            </div>
          ) : indexLoading || isActive(repo.status) ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-8 w-1/3" />
              <Skeleton className="h-4 w-2/3" />
              <Skeleton className="mt-6 h-[60vh] w-full rounded-xl" />
            </div>
          ) : (
            <CodeViewer repo={repo} path={activeFile} />
          )}
        </main>

        <aside id="chat" className="hidden w-[380px] shrink-0 border-l border-border/60 xl:flex">
          <ChatPanel
            repo={repo}
            messages={chat.messages}
            onSend={(text) => void chat.send(text)}
            onStop={chat.stop}
            onReset={chat.reset}
            streaming={chat.streaming}
            className="w-full"
          />
        </aside>
      </div>

      <Dialog open={navOpen} onOpenChange={setNavOpen}>
        <DialogContent className="h-[82dvh] max-w-sm gap-0 overflow-hidden p-0">
          <DialogHeader className="sr-only">
            <DialogTitle>Repository navigation</DialogTitle>
            <DialogDescription>Browse wiki pages and source files.</DialogDescription>
          </DialogHeader>
          {sidebar}
        </DialogContent>
      </Dialog>

      <Dialog open={chatOpen} onOpenChange={setChatOpen}>
        <DialogContent className="flex h-[88dvh] max-w-lg flex-col gap-0 overflow-hidden p-0">
          <DialogHeader className="sr-only">
            <DialogTitle>Ask this repository</DialogTitle>
            <DialogDescription>
              Chat with an agent grounded in {repo.owner}/{repo.name}.
            </DialogDescription>
          </DialogHeader>
          <ChatPanel
            repo={repo}
            messages={chat.messages}
            onSend={(text) => void chat.send(text)}
            onStop={chat.stop}
            onReset={chat.reset}
            streaming={chat.streaming}
            className="min-h-0 flex-1"
            inDialog
          />
        </DialogContent>
      </Dialog>

      {chatOpen && (
        <span className="sr-only" aria-live="polite">
          {chat.streaming ? "The agent is responding" : "Chat ready"}
        </span>
      )}

      {repo.status === "error" && (
        <div className="pointer-events-none fixed bottom-5 left-1/2 z-40 -translate-x-1/2">
          <div className="pointer-events-auto flex items-center gap-3 rounded-full border border-rose-500/25 bg-background/95 px-4 py-2 text-xs text-rose-500 shadow-lg backdrop-blur">
            <span>Analysis failed</span>
            <button
              type="button"
              onClick={() => void handleRetry()}
              className="inline-flex items-center gap-1 rounded-full bg-rose-500/10 px-2 py-0.5 font-medium hover:bg-rose-500/20"
            >
              <RefreshCw className="size-3" /> Retry
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function firstFile(node: TreeNode): string | null {
  if (node.type === "file") return node.path
  for (const child of node.children ?? []) {
    const found = firstFile(child)
    if (found) return found
  }
  return null
}

function RepoLoading() {
  return (
    <div className="flex h-dvh flex-col">
      <div className="flex h-14 items-center gap-3 border-b border-border/60 px-4">
        <Skeleton className="size-8 rounded-lg" />
        <Skeleton className="h-5 w-40" />
        <div className="ml-auto flex gap-2">
          <Skeleton className="size-8 rounded-lg" />
          <Skeleton className="size-8 rounded-lg" />
        </div>
      </div>
      <div className="flex flex-1">
        <div className="hidden w-[252px] space-y-2 border-r border-border/60 p-3 lg:block">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-4/5" />
        </div>
        <div className="mx-auto w-full max-w-3xl space-y-4 p-8">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-9 w-2/3" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="mt-8 h-64 w-full rounded-xl" />
        </div>
      </div>
      <div className="pointer-events-none fixed inset-0 grid place-items-center">
        <div className="flex items-center gap-2 rounded-full border border-border bg-background/90 px-4 py-2 text-xs text-muted-foreground shadow-lg backdrop-blur">
          <Loader2 className="size-3.5 animate-spin text-primary" /> Loading analysis…
        </div>
      </div>
    </div>
  )
}
