import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"
import { ArrowLeft, GripHorizontal, Loader2, MessageSquare, RefreshCw } from "lucide-react"
import { toast } from "sonner"
import { RepoHeader } from "@/components/repo/RepoHeader"
import { LanguageSwitcher } from "@/components/layout/LanguageSwitcher"
import { ThemeToggle } from "@/components/layout/ThemeToggle"
import { WikiSidebar, type RepoView } from "@/components/repo/WikiSidebar"
import { WikiView } from "@/components/repo/WikiView"
import { CodeViewer } from "@/components/repo/CodeViewer"
import { DirectoryView } from "@/components/repo/DirectoryView"
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
import { useI18n } from "@/i18n"

const CHAT_POSITION_KEY = "code-insight-chat-position"

type ChatPosition = { x: number; y: number }

function readChatPosition(): ChatPosition | null {
  try {
    const raw = window.localStorage.getItem(CHAT_POSITION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<ChatPosition>
    if (typeof parsed.x === "number" && typeof parsed.y === "number") {
      return { x: parsed.x, y: parsed.y }
    }
  } catch {
    // Ignore invalid persisted state.
  }
  return null
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

export function RepoPage() {
  const { id } = useParams<{ id: string }>()
  const { theme, toggle } = useAppTheme()
  const { t } = useI18n()
  const { repo, error, loading, refresh, regenerate } = useRepo(id)
  const chat = useChat(id)

  const [searchParams, setSearchParams] = useSearchParams()
  const [index, setIndex] = useState<IndexData | null>(null)
  const [indexLoading, setIndexLoading] = useState(false)
  const [indexError, setIndexError] = useState<string | null>(null)
  const [navOpen, setNavOpen] = useState(false)
  const [chatPos, setChatPos] = useState<ChatPosition | null>(readChatPosition)
  const chatPanelRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef<{
    startX: number
    startY: number
    originX: number
    originY: number
    width: number
    height: number
  } | null>(null)

  const pages = useMemo(() => repo?.wiki?.pages ?? [], [repo?.wiki])
  const requestedSlug = searchParams.get("page") ?? ""
  const activeFile = searchParams.get("path")
  const dirParam = searchParams.get("dir")
  const activeDir = activeFile ? null : dirParam === null ? null : dirParam === "." ? "" : dirParam
  const view: RepoView = searchParams.get("view") === "code" || activeFile !== null || activeDir !== null ? "code" : "wiki"
  const chatOpen = searchParams.get("ask") === "1"
  const page = useMemo(
    () => pages.find((item) => item.slug === requestedSlug) ?? pages[0] ?? null,
    [pages, requestedSlug],
  )
  const activeSlug = page?.slug ?? requestedSlug

  const updateParams = useCallback(
    (patch: Record<string, string | null>) => {
      const next = new URLSearchParams(searchParams)
      for (const [key, value] of Object.entries(patch)) {
        if (value === null || value === "") next.delete(key)
        else next.set(key, value)
      }
      setSearchParams(next, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  useEffect(() => {
    if (pages.length === 0) return
    if (!requestedSlug || !pages.some((item) => item.slug === requestedSlug)) {
      updateParams({ page: pages[0].slug })
    }
  }, [pages, requestedSlug, updateParams])

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
    if (view !== "code" || activeFile || activeDir !== null || !index) return
    const first = index.entryFiles?.[0] ?? index.keyFiles?.[0] ?? firstFile(index.tree)
    if (first) updateParams({ path: first })
  }, [view, activeFile, activeDir, index, updateParams])

  const handleRegenerate = useCallback(async () => {
    if (!repo) return
    try {
      await regenerate()
      toast.success(t("card.regenerating"), {
        description: t("repo.regeneratingDesc"),
      })
      updateParams({ page: pages[0]?.slug ?? "overview" })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("card.regenerateError"))
    }
  }, [regenerate, repo, pages])

  const handleRetry = useCallback(async () => {
    if (!repo) return
    try {
      await regenerate()
      toast.success(t("repo.retrying"))
      await refresh(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("repo.retryError"))
    }
  }, [regenerate, refresh, repo])

  const handleSelectFile = useCallback(
    (path: string) => {
      updateParams({ view: "code", path, dir: null })
      setNavOpen(false)
    },
    [updateParams],
  )

  const handleSelectDir = useCallback(
    (path: string) => {
      updateParams({ view: "code", dir: path === "" ? "." : path, path: null })
      setNavOpen(false)
    },
    [updateParams],
  )

  const handleSelectSlug = useCallback(
    (slug: string) => {
      updateParams({ view: "wiki", page: slug, path: null, dir: null })
      setNavOpen(false)
    },
    [updateParams],
  )

  const handleViewChange = useCallback(
    (next: RepoView) => {
      if (next === "wiki") updateParams({ view: "wiki", path: null, dir: null })
      else updateParams({ view: "code" })
      setNavOpen(false)
    },
    [updateParams],
  )

  const openChat = useCallback(() => updateParams({ ask: "1" }), [updateParams])
  const closeChat = useCallback(() => updateParams({ ask: null }), [updateParams])

  const startChatDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return
    const panel = chatPanelRef.current
    if (!panel) return
    const rect = panel.getBoundingClientRect()
    dragRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      originX: rect.left,
      originY: rect.top,
      width: rect.width,
      height: rect.height,
    }
    setChatPos({ x: rect.left, y: rect.top })
    event.currentTarget.setPointerCapture(event.pointerId)
    event.preventDefault()
  }, [])

  const moveChatDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) return
    setChatPos({
      x: clamp(drag.originX + event.clientX - drag.startX, 8, window.innerWidth - drag.width - 8),
      y: clamp(drag.originY + event.clientY - drag.startY, 8, window.innerHeight - drag.height - 8),
    })
  }, [])

  const endChatDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (!dragRef.current) return
    dragRef.current = null
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    const panel = chatPanelRef.current
    if (!panel) return
    const rect = panel.getBoundingClientRect()
    const position = { x: rect.left, y: rect.top }
    setChatPos(position)
    window.localStorage.setItem(CHAT_POSITION_KEY, JSON.stringify(position))
  }, [])

  const resetChatPosition = useCallback(() => {
    setChatPos(null)
    window.localStorage.removeItem(CHAT_POSITION_KEY)
  }, [])

  useEffect(() => {
    if (!chatOpen || !chatPos) return
    const panel = chatPanelRef.current
    if (!panel) return
    const rect = panel.getBoundingClientRect()
    const next = {
      x: clamp(chatPos.x, 8, window.innerWidth - rect.width - 8),
      y: clamp(chatPos.y, 8, window.innerHeight - rect.height - 8),
    }
    if (next.x !== chatPos.x || next.y !== chatPos.y) setChatPos(next)
  }, [chatOpen, chatPos])

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
          <h1 className="text-lg font-semibold">{t("repo.unavailable")}</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{error}</p>
          <div className="mt-5 flex justify-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/">{t("repo.backHome")}</Link>
            </Button>
            <Button onClick={() => void refresh()}>
              <RefreshCw /> {t("gen.retry")}
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
      onViewChange={handleViewChange}
      pages={pages}
      activeSlug={activeSlug}
      onSelectSlug={handleSelectSlug}
      index={index}
      activeFile={activeFile}
      onSelectFile={handleSelectFile}
      activeDir={activeDir}
      onSelectDir={handleSelectDir}
    />
  )

  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <RepoHeader
        repo={repo}
        onAsk={openChat}
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
              onNavigate={handleSelectSlug}
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
          ) : activeDir !== null && index ? (
            <DirectoryView
              repo={repo}
              node={findNode(index.tree, activeDir) ?? index.tree}
              onOpen={(path, kind) =>
                kind === "dir" ? handleSelectDir(path) : handleSelectFile(path)
              }
            />
          ) : (
            <CodeViewer repo={repo} path={activeFile} />
          )}
        </main>


      </div>

      <footer className="flex h-9 shrink-0 items-center justify-end gap-1 border-t border-border/60 bg-background/60 px-3">
        <LanguageSwitcher />
        <ThemeToggle theme={theme} onToggle={toggle} />
      </footer>

      <Dialog open={navOpen} onOpenChange={setNavOpen}>
        <DialogContent className="h-[82dvh] max-w-sm gap-0 overflow-hidden p-0">
          <DialogHeader className="sr-only">
            <DialogTitle>Repository navigation</DialogTitle>
            <DialogDescription>Browse wiki pages and source files.</DialogDescription>
          </DialogHeader>
          {sidebar}
        </DialogContent>
      </Dialog>

      {chatOpen && (
        <div
          ref={chatPanelRef}
          style={
            chatPos
              ? { left: chatPos.x, top: chatPos.y, bottom: "auto", transform: "none" }
              : undefined
          }
          className={cn(
            "fixed z-50 flex h-[min(640px,calc(100dvh-6rem))] w-[min(560px,calc(100vw-1.5rem))] flex-col overflow-hidden rounded-sm border border-border bg-background/95 shadow-2xl backdrop-blur-md",
            !chatPos && "bottom-11 left-1/2 -translate-x-1/2",
          )}
        >
          <div
            onPointerDown={startChatDrag}
            onPointerMove={moveChatDrag}
            onPointerUp={endChatDrag}
            onPointerCancel={endChatDrag}
            onDoubleClick={resetChatPosition}
            title={t("chat.dragHint")}
            className="flex h-6 shrink-0 cursor-grab touch-none items-center justify-center border-b border-border/50 bg-muted/25 active:cursor-grabbing"
          >
            <GripHorizontal className="size-3.5 text-muted-foreground/50" />
          </div>
          <ChatPanel
            repo={repo}
            messages={chat.messages}
            onSend={(text) => void chat.send(text)}
            onStop={chat.stop}
            onReset={chat.reset}
            streaming={chat.streaming}
            className="min-h-0 flex-1"
            onClose={closeChat}
          />
        </div>
      )}

      {!chatOpen && (
        <button
          type="button"
          onClick={openChat}
          className="fixed bottom-12 left-1/2 z-40 inline-flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-primary/30 bg-primary px-4 py-2 text-xs font-medium text-primary-foreground shadow-lg transition-transform hover:-translate-y-0.5"
        >
          <MessageSquare className="size-3.5" />
          {t("repo.ask")}
        </button>
      )}

      {chatOpen && (
        <span className="sr-only" aria-live="polite">
          {chat.streaming ? "The agent is responding" : "Chat ready"}
        </span>
      )}

      {repo.status === "error" && (
        <div className="pointer-events-none fixed bottom-5 left-1/2 z-40 -translate-x-1/2">
          <div className="pointer-events-auto flex items-center gap-3 rounded-full border border-rose-500/25 bg-background/95 px-4 py-2 text-xs text-rose-500 shadow-lg backdrop-blur">
            <span>{t("gen.failed")}</span>
            <button
              type="button"
              onClick={() => void handleRetry()}
              className="inline-flex items-center gap-1 rounded-full bg-rose-500/10 px-2 py-0.5 font-medium hover:bg-rose-500/20"
            >
              <RefreshCw className="size-3" /> {t("gen.retry")}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function findNode(node: TreeNode, path: string): TreeNode | null {
  if (node.path === path) return node
  for (const child of node.children ?? []) {
    const found = findNode(child, path)
    if (found) return found
  }
  return null
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
  const { t } = useI18n()
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
          <Loader2 className="size-3.5 animate-spin text-primary" /> {t("repo.loading")}
        </div>
      </div>
    </div>
  )
}
