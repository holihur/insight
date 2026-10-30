import { useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from "react"
import {
  Check,
  CheckCheck,
  Copy,
  ExternalLink,
  FileCode2,
  FileText,
  ListTree,
  Loader2,
  MessageSquarePlus,
  Search,
  SlidersHorizontal,
  TriangleAlert,
  X,
  ChevronUp,
  ChevronDown,
} from "lucide-react"
import { toast } from "sonner"
import hljs from "highlight.js/lib/core"
import go from "highlight.js/lib/languages/go"
import typescript from "highlight.js/lib/languages/typescript"
import javascript from "highlight.js/lib/languages/javascript"
import python from "highlight.js/lib/languages/python"
import rust from "highlight.js/lib/languages/rust"
import java from "highlight.js/lib/languages/java"
import cpp from "highlight.js/lib/languages/cpp"
import c from "highlight.js/lib/languages/c"
import csharp from "highlight.js/lib/languages/csharp"
import php from "highlight.js/lib/languages/php"
import ruby from "highlight.js/lib/languages/ruby"
import kotlin from "highlight.js/lib/languages/kotlin"
import swift from "highlight.js/lib/languages/swift"
import dart from "highlight.js/lib/languages/dart"
import scala from "highlight.js/lib/languages/scala"
import bash from "highlight.js/lib/languages/bash"
import powershell from "highlight.js/lib/languages/powershell"
import sql from "highlight.js/lib/languages/sql"
import yaml from "highlight.js/lib/languages/yaml"
import json from "highlight.js/lib/languages/json"
import markdown from "highlight.js/lib/languages/markdown"
import xml from "highlight.js/lib/languages/xml"
import css from "highlight.js/lib/languages/css"
import scss from "highlight.js/lib/languages/scss"
import dockerfile from "highlight.js/lib/languages/dockerfile"
import makefile from "highlight.js/lib/languages/makefile"
import ini from "highlight.js/lib/languages/ini"
import graphql from "highlight.js/lib/languages/graphql"
import protobuf from "highlight.js/lib/languages/protobuf"
import lua from "highlight.js/lib/languages/lua"
import perl from "highlight.js/lib/languages/perl"
import r from "highlight.js/lib/languages/r"
import elixir from "highlight.js/lib/languages/elixir"
import haskell from "highlight.js/lib/languages/haskell"

import { Badge } from "@/components/ui/badge"
import { CallGraphDialog } from "@/components/repo/CallGraphDialog"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { CodeMinimap } from "@/components/repo/CodeMinimap"
import { Progress } from "@/components/ui/progress"
import { Textarea } from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { api } from "@/lib/api"
import { injectTokenSpans, lineStartOffsets, siteSpans } from "@/lib/codeTokens"
import { cn, formatNumber } from "@/lib/utils"
import type { CallSite, Coverage, CoverageFunction, FileContent, Repo } from "@/lib/types"
import { useI18n } from "@/i18n"

const languages: Array<[string, unknown]> = [
  ["go", go],
  ["typescript", typescript],
  ["javascript", javascript],
  ["python", python],
  ["rust", rust],
  ["java", java],
  ["cpp", cpp],
  ["c", c],
  ["csharp", csharp],
  ["php", php],
  ["ruby", ruby],
  ["kotlin", kotlin],
  ["swift", swift],
  ["dart", dart],
  ["scala", scala],
  ["bash", bash],
  ["powershell", powershell],
  ["sql", sql],
  ["yaml", yaml],
  ["json", json],
  ["markdown", markdown],
  ["xml", xml],
  ["css", css],
  ["scss", scss],
  ["dockerfile", dockerfile],
  ["makefile", makefile],
  ["ini", ini],
  ["graphql", graphql],
  ["protobuf", protobuf],
  ["lua", lua],
  ["perl", perl],
  ["r", r],
  ["elixir", elixir],
  ["haskell", haskell],
]

for (const [name, language] of languages) {
  hljs.registerLanguage(name, language as never)
}

const languageMap: Record<string, string> = {
  Go: "go",
  TypeScript: "typescript",
  JavaScript: "javascript",
  Python: "python",
  Rust: "rust",
  Java: "java",
  "C++": "cpp",
  C: "c",
  "C#": "csharp",
  PHP: "php",
  Ruby: "ruby",
  Kotlin: "kotlin",
  Swift: "swift",
  Dart: "dart",
  Scala: "scala",
  Shell: "bash",
  PowerShell: "powershell",
  SQL: "sql",
  YAML: "yaml",
  JSON: "json",
  Markdown: "markdown",
  HTML: "xml",
  XML: "xml",
  Vue: "xml",
  Svelte: "xml",
  CSS: "css",
  SCSS: "scss",
  Dockerfile: "dockerfile",
  Makefile: "makefile",
  TOML: "ini",
  INI: "ini",
  GraphQL: "graphql",
  "Protocol Buffers": "protobuf",
  Lua: "lua",
  Perl: "perl",
  R: "r",
  Elixir: "elixir",
  Haskell: "haskell",
  "Go module": "ini",
  Gradle: "groovy",
}

const MAX_LINES = 8000
const PAGE_LINES = 2500

export function CodeViewer({
  repo,
  path,
  highlightLines,
  onAskSelection,
  onOpenDir,
  onOpenFile,
}: {
  repo: Repo
  path: string | null
  highlightLines?: [number, number] | null
  onAskSelection?: (code: string, path: string, lines: [number, number]) => void
  onOpenDir?: (path: string) => void
  onOpenFile?: (path: string, lines?: [number, number]) => void
}) {
  const { t } = useI18n()
  const [file, setFile] = useState<FileContent | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [coverage, setCoverage] = useState<Coverage | null>(null)
  const [coverageLoading, setCoverageLoading] = useState(false)
  const [coverageError, setCoverageError] = useState<string | null>(null)
  const [reviewing, setReviewing] = useState<string | null>(null)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [patternsText, setPatternsText] = useState((repo.ignore ?? []).join("\n"))
  const [savingFilters, setSavingFilters] = useState(false)
  const [visibleCount, setVisibleCount] = useState(PAGE_LINES)
  const [findQuery, setFindQuery] = useState("")
  const [findIndex, setFindIndex] = useState(0)
  const [selection, setSelection] = useState<{ text: string; lines: [number, number] } | null>(null)
  const [callSites, setCallSites] = useState<CallSite[]>([])
  const [callsIndexed, setCallsIndexed] = useState(false)
  const [tokenSheet, setTokenSheet] = useState<{ site: CallSite; mode: "menu" | "def" | "calls" } | null>(null)
  const [callPanel, setCallPanel] = useState<string | null>(null)
  const callSitesRequestRef = useRef(0)
  const requestRef = useRef(0)
  const coverageRequestRef = useRef(0)
  const codeScrollRef = useRef<HTMLDivElement>(null)
  const preRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (!path) {
      setFile(null)
      return
    }
    const id = ++requestRef.current
    setLoading(true)
    setError(null)
    api
      .getFile(repo.id, path)
      .then((data) => {
        if (requestRef.current === id) setFile(data)
      })
      .catch((err: unknown) => {
        if (requestRef.current === id) {
          setError(err instanceof Error ? err.message : "Could not load this file")
        }
      })
      .finally(() => {
        if (requestRef.current === id) setLoading(false)
      })
  }, [repo.id, path])

  useEffect(() => {
    if (!path) {
      setCoverage(null)
      return
    }
    const requestID = ++coverageRequestRef.current
    setCoverageLoading(true)
    setCoverageError(null)
    api
      .getCoverage(repo.id, path)
      .then((data) => {
        if (coverageRequestRef.current === requestID) setCoverage(data)
      })
      .catch((err: unknown) => {
        if (coverageRequestRef.current === requestID) {
          setCoverageError(err instanceof Error ? err.message : t("coverage.error"))
        }
      })
      .finally(() => {
        if (coverageRequestRef.current === requestID) setCoverageLoading(false)
      })
  }, [repo.id, path, t])

  useEffect(() => {
    if (!path) {
      setCallSites([])
      setCallsIndexed(false)
      return
    }
    const requestID = ++callSitesRequestRef.current
    api
      .getCallSites(repo.id, path)
      .then((data) => {
        if (callSitesRequestRef.current !== requestID) return
        setCallSites(data.sites ?? [])
        setCallsIndexed(data.indexed)
      })
      .catch(() => {
        if (callSitesRequestRef.current !== requestID) return
        setCallSites([])
        setCallsIndexed(false)
      })
  }, [repo.id, path])

  useEffect(() => {
    setPatternsText((repo.ignore ?? []).join("\n"))
  }, [repo.ignore])

  const raw = file?.content ?? ""
  const lines = useMemo(() => (raw ? raw.split("\n") : []), [raw])
  // pagination: show at least visibleCount but respect MAX_LINES
  const effectiveVisible = Math.min(lines.length, Math.max(visibleCount, highlightLines ? highlightLines[1] : 0))
  const visible = lines.slice(0, Math.min(effectiveVisible, MAX_LINES))
  const functionsByStart = useMemo(() => {
    const map = new Map<number, CoverageFunction>()
    for (const fn of coverage?.functions ?? []) {
      map.set(fn.startLine, fn)
    }
    return map
  }, [coverage])

  // search matches
  const searchMatches = useMemo(() => {
    if (!findQuery.trim()) return [] as number[]
    const q = findQuery.toLowerCase()
    const out: number[] = []
    visible.forEach((line, i) => {
      if (line.toLowerCase().includes(q)) out.push(i + 1)
    })
    return out
  }, [findQuery, visible])
  const searchSet = useMemo(() => new Set(searchMatches), [searchMatches])

  // symbols sorted
  const symbols = useMemo(() => (coverage?.functions ?? []).slice().sort((a, b) => a.startLine - b.startLine), [coverage])

  const highlighted = useMemo(() => {
    if (!file) return ""
    const code = visible.join("\n")
    const lang = languageMap[file.language]
    try {
      if (lang && hljs.getLanguage(lang)) {
        return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value
      }
      return hljs.highlightAuto(code).value
    } catch {
      return escapeHtml(code)
    }
  }, [file, visible])

  const allSites = useMemo<CallSite[]>(() => {
    const known = new Set(callSites.filter((site) => site.kind === "def").map((site) => `${site.line}:${site.token}`))
    const extra: CallSite[] = []
    for (const fn of coverage?.functions ?? []) {
      const base = fn.name.slice(fn.name.lastIndexOf(".") + 1)
      const line = lines[fn.startLine - 1]
      if (!line || known.has(`${fn.startLine}:${base}`)) continue
      const col = line.indexOf(base)
      if (col < 0) continue
      extra.push({
        path: path ?? "",
        line: fn.startLine,
        col,
        endCol: col + base.length,
        token: base,
        kind: "def",
        functionId: fn.id,
        targets: [{ id: fn.id, path: path ?? "", name: fn.name, line: fn.startLine }],
      })
    }
    return [...callSites, ...extra]
  }, [callSites, coverage, lines, path])

  const lineStarts = useMemo(() => lineStartOffsets(visible), [visible])
  const tokenSpans = useMemo(
    () => siteSpans(allSites, lineStarts, visible.length),
    [allSites, lineStarts, visible.length],
  )
  const highlightedMarkup = useMemo(() => injectTokenSpans(highlighted, tokenSpans), [highlighted, tokenSpans])
  const siteByKey = useMemo(() => {
    const map = new Map<string, CallSite>()
    for (const site of allSites) map.set(`${site.line}:${site.col}`, site)
    return map
  }, [allSites])

  // jump to highlighted lines when they change
  useEffect(() => {
    if (highlightLines && visible.length) {
      const t = setTimeout(() => jumpToLine(highlightLines[0]), 100)
      return () => clearTimeout(t)
    }
  }, [highlightLines, visible.length])

  // keep findIndex in bounds
  useEffect(() => {
    if (searchMatches.length === 0) setFindIndex(0)
    else if (findIndex >= searchMatches.length) setFindIndex(0)
  }, [searchMatches, findIndex])

  // reset visibleCount on file change
  useEffect(() => {
    setVisibleCount(PAGE_LINES)
    setFindQuery("")
    setSelection(null)
  }, [path])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(raw)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      // ignore
    }
  }

  const toggleReviewed = async (fn: CoverageFunction) => {
    if (!coverage || reviewing) return
    setReviewing(fn.id)
    try {
      const next = await api.reviewFunction(repo.id, fn.id, !fn.reviewed)
      setCoverage(next)
    } catch (err) {
      setCoverageError(err instanceof Error ? err.message : t("coverage.error"))
    } finally {
      setReviewing(null)
    }
  }

  const jumpToLine = (line: number) => {
    const scroller = codeScrollRef.current
    if (!scroller) return
    scroller.scrollTo({ top: Math.max(0, (line - 1) * 20.4), behavior: "smooth" })
  }

  const copyLinkWithLine = async (line: number) => {
    if (!path) return
    const url = new URL(window.location.href)
    url.searchParams.set("path", path)
    url.searchParams.set("lines", String(line))
    url.searchParams.set("view", "code")
    try {
      await navigator.clipboard.writeText(url.toString())
      toast.success("链接已复制", { description: `${path}:${line}` })
    } catch {
      toast.error("无法复制链接")
    }
  }

  const handleSelectUp = () => {
    const sel = window.getSelection()
    if (!sel || sel.isCollapsed || !preRef.current) {
      setSelection(null)
      return
    }
    const text = sel.toString()
    if (!text || text.length < 2 || text.length > 5000) {
      setSelection(null)
      return
    }
    // Estimate lines from selection: find start/end offsets
    // Simplify: if selection inside pre, we guess lines by counting newlines before anchor
    // Use naive: count selected lines by splitting text
    const lineCount = text.split("\n").length
    // Try to find line numbers by locating text in raw
    // fallback to 1 based on visible
    let startLine = 1
    const idx = raw.indexOf(text.slice(0, 80))
    if (idx >= 0) startLine = raw.slice(0, idx).split("\n").length
    const endLine = startLine + lineCount - 1
    setSelection({ text, lines: [startLine, endLine] })
  }

  const batchToggle = async (reviewed: boolean) => {
    if (!coverage || !path) return
    const todo = coverage.functions.filter((f) => f.reviewed !== reviewed)
    if (todo.length === 0) return
    setReviewing("batch")
    try {
      let last: Coverage | null = null
      for (const fn of todo) {
        last = await api.reviewFunction(repo.id, fn.id, reviewed)
      }
      if (last) setCoverage(last)
      else {
        const cur = await api.getCoverage(repo.id, path)
        setCoverage(cur)
      }
      toast.success(reviewed ? `已标记 ${todo.length} 个函数为已读` : `已取消 ${todo.length} 个`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("coverage.error"))
    } finally {
      setReviewing(null)
    }
  }

  const saveFilters = async () => {
    if (savingFilters) return
    setSavingFilters(true)
    try {
      const patterns = patternsText
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
      await api.updateIgnore(repo.id, patterns)
      setFiltersOpen(false)
      toast.success(t("coverage.saved"))
      if (path) {
        const next = await api.getCoverage(repo.id, path)
        setCoverage(next)
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("coverage.error"))
    } finally {
      setSavingFilters(false)
    }
  }

  const isHighlighted = (line: number) => {
    if (!highlightLines) return false
    const [a, b] = highlightLines
    return line >= a && line <= b
  }

  const goTo = (targetPath: string, line: number) => {
    if (onOpenFile) onOpenFile(targetPath, [line, line])
    else if (targetPath === path) jumpToLine(line)
  }

  const chooseDefinitions = (site: CallSite) => {
    const targets = site.targets ?? []
    if (!targets.length) {
      toast.info(t("callgraph.noTargets"))
      return
    }
    if (targets.length === 1) {
      goTo(targets[0].path, targets[0].line)
      return
    }
    setTokenSheet({ site, mode: "def" })
  }

  const openDefinitions = async (site: CallSite) => {
    if (site.kind === "def" && site.functionId) {
      try {
        const res = await api.getCalls(repo.id, site.functionId)
        const impls = (res.implementations ?? []).filter((item) => item.id !== site.functionId)
        if (impls.length > 0) {
          setTokenSheet({ site: { ...site, targets: impls }, mode: "def" })
          return
        }
        toast.info(t("callgraph.onlyDefinition"))
        return
      } catch {
        // fall through to the known targets
      }
    }
    chooseDefinitions(site)
  }

  const chooseCalls = (site: CallSite) => {
    const targets = site.targets ?? []
    if (!targets.length) {
      toast.info(t("callgraph.noTargets"))
      return
    }
    if (targets.length === 1) {
      setCallPanel(targets[0].id)
      return
    }
    setTokenSheet({ site, mode: "calls" })
  }

  const handleTokenClick = (event: ReactMouseEvent<HTMLElement>) => {
    const element = (event.target as HTMLElement).closest("[data-cg-key]")
    if (!element) return
    const site = siteByKey.get(element.getAttribute("data-cg-key") ?? "")
    if (!site) return
    const coarse = typeof window !== "undefined" && window.matchMedia?.("(pointer: coarse)").matches
    if (coarse) {
      setTokenSheet({ site, mode: "menu" })
      return
    }
    if (event.metaKey || event.ctrlKey) {
      event.preventDefault()
      void openDefinitions(site)
      return
    }
    if (event.altKey) {
      event.preventDefault()
      chooseCalls(site)
    }
  }

  if (!path) {
    return (
      <div className="grid h-full place-items-center p-8">
        <div className="max-w-sm text-center">
          <div className="mx-auto mb-4 grid size-14 place-items-center rounded-2xl border border-border/70 bg-card/60">
            <FileCode2 className="size-6 text-muted-foreground/60" />
          </div>
          <h2 className="font-semibold">{t("code.browseTitle")}</h2>
          <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
            {t("code.browseBody")}
          </p>
        </div>
      </div>
    )
  }

  const githubURL = `${repo.webUrl}/blob/${repo.branch || "main"}/${path}`

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b border-border/60 px-4">
        <FileText className="size-3.5 shrink-0 text-muted-foreground" />
        <div className="flex min-w-0 items-center gap-1.5 overflow-x-auto font-mono text-[11px] text-muted-foreground">
          <button type="button" title="根目录" onClick={() => onOpenDir?.("")} className="shrink-0 hover:text-foreground hover:underline underline-offset-2">{repo.name}</button>
          <span className="opacity-40">/</span>
          {path.split("/").map((part, index, array) => {
            const isLast = index === array.length - 1
            const dirPath = array.slice(0, index + 1).join("/")
            return (
              <span key={index} className="inline-flex shrink-0 items-center gap-1">
                {isLast ? (
                  <span className="text-foreground font-medium">{part}</span>
                ) : (
                  <button type="button" onClick={() => onOpenDir?.(dirPath)} className="hover:text-foreground hover:underline underline-offset-2">{part}</button>
                )}
                {index < array.length - 1 && <span className="opacity-40">/</span>}
              </span>
            )
          })}
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-1">
          {coverage && coverage.functions.length > 0 && (
            <span className="hidden items-center gap-1 font-mono text-[10px] text-muted-foreground sm:inline-flex">
              <ListTree className="size-3" /> {coverage.functions.length} 函数
            </span>
          )}
          {file?.language && (
            <Badge variant="outline" className="hidden text-[10px] sm:inline-flex">
              {file.language}
            </Badge>
          )}
          <Button variant="ghost" size="icon-sm" onClick={copy} aria-label={t("code.copy")}>
            {copied ? <Check className="text-emerald-500" /> : <Copy />}
          </Button>
          <Button variant="ghost" size="icon-sm" asChild aria-label={t("code.github")}>
            <a href={githubURL} target="_blank" rel="noreferrer">
              <ExternalLink />
            </a>
          </Button>
        </div>
      </div>

      {/* toolbar: symbols + find + batch */}
      {!loading && !error && file && (
        <div className="flex flex-wrap items-center gap-1.5 border-b border-border/60 bg-muted/20 px-2 py-1.5">
          {/* symbols */}
          {symbols.length > 0 && (
            <div className="flex items-center gap-1">
              <ListTree className="size-3 text-muted-foreground" />
              <select
                onChange={(e) => {
                  const v = Number(e.target.value)
                  if (v) jumpToLine(v)
                }}
                defaultValue=""
                className="h-7 rounded-sm border border-border/70 bg-background px-2 font-mono text-[11px] outline-none focus:border-primary/40"
                aria-label="跳转到函数"
              >
                <option value="">跳转函数… ({symbols.length})</option>
                {symbols.map((fn) => (
                  <option key={fn.id} value={fn.startLine}>
                    {fn.name} :{fn.startLine}{fn.changed ? " · changed" : ""}{fn.reviewed ? " ✓" : ""}
                  </option>
                ))}
              </select>
            </div>
          )}
          {/* find */}
          <div className="flex items-center gap-1 rounded-sm border border-border/70 bg-background px-1.5 py-0.5 focus-within:border-primary/40">
            <Search className="size-3 text-muted-foreground" />
            <input
              value={findQuery}
              onChange={(e) => setFindQuery(e.target.value)}
              placeholder="查找…"
              onKeyDown={(e) => {
                if (e.key === "Enter" && searchMatches.length) {
                  e.preventDefault()
                  const next = (findIndex + 1) % searchMatches.length
                  setFindIndex(next)
                  jumpToLine(searchMatches[next])
                }
              }}
              className="h-6 w-28 bg-transparent font-mono text-[11px] outline-none placeholder:text-muted-foreground/50 sm:w-40"
              aria-label="文件内查找"
            />
            {findQuery && (
              <button type="button" onClick={() => setFindQuery("")} className="grid size-5 place-items-center rounded text-muted-foreground hover:bg-muted">
                <X className="size-3" />
              </button>
            )}
          </div>
          {findQuery && (
            <span className="flex items-center gap-1 font-mono text-[11px] text-muted-foreground">
              <span>{searchMatches.length ? `${findIndex + 1}/${searchMatches.length}` : "0"}</span>
              <Button variant="ghost" size="icon-sm" className="size-6" disabled={!searchMatches.length} onClick={() => { const prev = (findIndex - 1 + searchMatches.length) % searchMatches.length; setFindIndex(prev); jumpToLine(searchMatches[prev])}} aria-label="上一个"><ChevronUp className="size-3" /></Button>
              <Button variant="ghost" size="icon-sm" className="size-6" disabled={!searchMatches.length} onClick={() => { const nxt = (findIndex + 1) % searchMatches.length; setFindIndex(nxt); jumpToLine(searchMatches[nxt])}} aria-label="下一个"><ChevronDown className="size-3" /></Button>
            </span>
          )}
          {callsIndexed && (
            <span className="hidden items-center gap-1 text-[10px] text-muted-foreground xl:inline-flex" title={t("callgraph.hint")}>
              <ListTree className="size-3" /> {t("callgraph.hint")}
            </span>
          )}
          <div className="ml-auto flex items-center gap-1">
            {coverage && coverage.functions.length > 1 && (
              <>
                <Button variant="ghost" size="sm" className="h-7 text-[11px]" disabled={reviewing === "batch"} onClick={() => void batchToggle(true)}><CheckCheck className="size-3" /> 全部已读</Button>
                <Button variant="ghost" size="sm" className="h-7 text-[11px]" disabled={reviewing === "batch"} onClick={() => void batchToggle(false)}>全部未读</Button>
              </>
            )}
          </div>
        </div>
      )}

      {(coverage || coverageLoading || coverageError) && (
        <div className="shrink-0 border-b border-border/60 bg-card/25 px-4 py-2">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
            <span className="font-mono text-[9px] uppercase tracking-[0.16em] text-muted-foreground">
              {t("coverage.title")}
            </span>
            {coverage ? (
              <>
                <CoverageMetric
                  label={t("coverage.overall")}
                  reviewed={coverage.reviewedFunctions}
                  total={coverage.totalFunctions}
                  percent={coverage.overallPercent}
                />
                {coverage.changedAvailable ? (
                  <CoverageMetric
                    label={t("coverage.changed")}
                    reviewed={coverage.reviewedChanged}
                    total={coverage.changedFunctions}
                    percent={coverage.changedPercent}
                  />
                ) : (
                  <span className="text-[10px] text-muted-foreground">
                    {t("coverage.unavailable")}
                  </span>
                )}
                <Badge variant="outline" className="font-mono text-[9px]">
                  {coverage.commit}
                </Badge>
                {coverage.ignoredFiles > 0 && (
                  <Badge variant="secondary" className="text-[9px]">
                    {t("coverage.ignored", { count: coverage.ignoredFiles })}
                  </Badge>
                )}
              </>
            ) : coverageError ? (
              <span className="inline-flex items-center gap-1 text-[10px] text-amber-500">
                <TriangleAlert className="size-3" /> {coverageError}
              </span>
            ) : (
              <Skeleton className="h-3 w-48" />
            )}
            <div className="ml-auto flex items-center gap-1.5">
              {coverage && coverage.functions.some((f) => !f.reviewed) && (
                <Button variant="outline" size="sm" className="h-6 text-[10px]" onClick={() => setDetailsOpen(true)} title="查看未覆盖">
                  未覆盖 {coverage.functions.filter((f) => !f.reviewed).length}
                </Button>
              )}
              {coverage && (
                <span className="font-mono text-[10px] text-muted-foreground">
                  {t("coverage.functions", { count: coverage.functions.length })}
                </span>
              )}
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => setFiltersOpen(true)}
                aria-label={t("coverage.filtersTitle")}
                className="text-muted-foreground"
              >
                <SlidersHorizontal />
              </Button>
            </div>
          </div>
          {coverage && (
            <div className="mt-1.5 grid grid-cols-2 gap-3">
              <Progress value={coverage.overallPercent} className="h-1" />
              <Progress
                value={coverage.changedAvailable ? coverage.changedPercent : 0}
                className="h-1"
              />
            </div>
          )}
        </div>
      )}

      <div className="flex min-h-0 flex-1">
        <div ref={codeScrollRef} className="min-h-0 flex-1 overflow-auto">
        {loading && (
          <div className="space-y-2 p-5">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Loader2 className="size-3.5 animate-spin" /> {t("code.loading", { path })}
            </div>
            {Array.from({ length: 14 }).map((_, index) => (
              <Skeleton
                key={index}
                className="h-4"
                style={{ width: `${45 + ((index * 17) % 50)}%` }}
              />
            ))}
          </div>
        )}
        {!loading && error && (() => {
          const lower = error.toLowerCase()
          const isBinary = lower.includes("binary")
          const isLarge = lower.includes("too large")
          const isImage = /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(path ?? "")
          const rawURL = `https://raw.githubusercontent.com/${repo.owner}/${repo.name}/${encodeURIComponent(repo.branch || "main")}/${(path ?? "").split("/").map(encodeURIComponent).join("/")}`
          return (
            <div className="m-5 rounded-xl border border-rose-500/20 bg-rose-500/5 p-4 text-sm text-rose-500">
              <div className="flex items-start gap-2">
                <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                <span>{error}</span>
              </div>
              {(isBinary || isLarge) && (
                <div className="mt-3 flex flex-wrap gap-2">
                  <Button variant="outline" size="sm" asChild className="h-7 text-xs border-rose-500/30 hover:bg-rose-500/10"><a href={githubURL} target="_blank" rel="noreferrer"><ExternalLink className="size-3" /> 在 GitHub 查看</a></Button>
                  {isImage && (
                    <Button variant="outline" size="sm" asChild className="h-7 text-xs"><a href={rawURL} target="_blank" rel="noreferrer">打开原图</a></Button>
                  )}
                </div>
              )}
              {isBinary && isImage && (
                <div className="mt-3 overflow-hidden rounded-sm border border-border">
                  <img src={rawURL} alt={path ?? ""} className="max-h-[60vh] w-full object-contain bg-white" loading="lazy" onError={(e) => ((e.target as HTMLImageElement).style.display = "none")} />
                </div>
              )}
            </div>
          )
        })()}
        {!loading && !error && file && (
          <div className="flex min-w-fit text-[12px] leading-[1.7]" onMouseUp={handleSelectUp}>
            <div className="sticky left-0 z-10 shrink-0 select-none border-r border-border/60 bg-card/60 py-4 pr-2 font-mono text-[11px] text-muted-foreground/50">
              {visible.map((_, index) => {
                const line = index + 1
                const fn = functionsByStart.get(line)
                const isHLS = isHighlighted(line)
                const isFind = searchSet.has(line)
                const blameURL = `${repo.webUrl}/blame/${encodeURIComponent(repo.branch || "main")}/${path.split("/").map(encodeURIComponent).join("/")}#L${line}`
                return (
                  <div
                    key={line}
                    className={cn(
                      "group flex h-[20.4px] items-center justify-end gap-1 pl-1 leading-[20.4px]",
                      isHLS && "bg-amber-500/15",
                      !isHLS && fn?.reviewed && "bg-emerald-500/[0.06]",
                      !isHLS && isFind && "bg-amber-500/10",
                    )}
                  >
                    {fn ? (
                      <button
                        type="button"
                        onClick={() => void toggleReviewed(fn)}
                        aria-pressed={fn.reviewed}
                        aria-label={
                          fn.reviewed ? t("coverage.unreviewTip") : t("coverage.reviewTip")
                        }
                        title={`${fn.name} · ${
                          fn.reviewed ? t("coverage.reviewed") : t("coverage.reviewTip")
                        } [空格]`}
                        disabled={reviewing === fn.id || reviewing === "batch"}
                        className={cn(
                          "grid size-4 shrink-0 place-items-center rounded-[2px] border transition-colors",
                          fn.reviewed
                            ? "border-emerald-500/45 bg-emerald-500/15 text-emerald-500"
                            : "border-border/80 text-muted-foreground/25 hover:border-primary/50 hover:text-primary",
                        )}
                      >
                        {reviewing === fn.id ? (
                          <Loader2 className="size-2.5 animate-spin" />
                        ) : (
                          <Check className={cn("size-2.5", !fn.reviewed && "opacity-50")} />
                        )}
                      </button>
                    ) : (
                      <span className="size-4 shrink-0" />
                    )}
                    <button type="button" onClick={() => void copyLinkWithLine(line)} className={cn("min-w-6 text-right tabular-nums hover:text-primary", isHLS && "font-bold text-amber-600 dark:text-amber-400")}>{line}</button>
                    <a href={blameURL} target="_blank" rel="noreferrer" className="hidden size-4 place-items-center rounded text-muted-foreground/20 hover:text-primary group-hover:grid" title="Blame - 在 GitHub 查看此行提交" aria-label="Blame"><ExternalLink className="size-2.5" /></a>
                  </div>
                )
              })}
            </div>
            <pre ref={preRef} onClick={handleTokenClick} className="flex-1 overflow-x-auto px-4 py-4 font-mono">
              <code dangerouslySetInnerHTML={{ __html: highlightedMarkup }} />
            </pre>
          </div>
        )}
        {!loading && !error && file && selection && onAskSelection && path && (
          <div className="sticky bottom-3 mx-4 flex items-center gap-2 rounded-full border border-primary/25 bg-background/95 px-3 py-1.5 shadow-lg backdrop-blur">
            <span className="max-w-[260px] truncate font-mono text-[11px] text-muted-foreground">{selection.lines[0]}:{selection.lines[1]} · {selection.text.slice(0, 40)}…</span>
            <Button size="sm" className="h-6 rounded-full px-2.5 text-[11px]" onClick={() => { onAskSelection(selection.text, path, selection.lines); setSelection(null) }}><MessageSquarePlus className="size-3" /> 问问这段</Button>
            <Button variant="ghost" size="icon-sm" className="size-6" onClick={() => setSelection(null)}><X className="size-3" /></Button>
          </div>
        )}
        {!loading && !error && file && lines.length > visible.length && (
          <div className="flex items-center justify-between border-t border-border/60 px-4 py-2 text-[11px] text-muted-foreground">
            <span>已显示 {formatNumber(visible.length)} / {formatNumber(file.total)} 行</span>
            <Button variant="outline" size="sm" className="h-7 text-[11px]" onClick={() => setVisibleCount((c) => Math.min(MAX_LINES, c + PAGE_LINES))}>加载更多 {formatNumber(Math.min(PAGE_LINES, file.total - visible.length))} 行</Button>
          </div>
        )}
        {!loading && !error && file && lines.length > MAX_LINES && visible.length >= MAX_LINES && (
          <div className="border-t border-border/60 px-4 py-2 text-[11px] text-muted-foreground">
            已达到显示上限 {formatNumber(MAX_LINES)} 行，<a href={githubURL} target="_blank" rel="noreferrer" className="text-primary hover:underline">{t("code.openFull")}</a>
          </div>
        )}
        </div>
        <CodeMinimap
          totalLines={Math.min(lines.length, MAX_LINES)}
          functions={coverage?.functions ?? []}
          scrollRef={codeScrollRef}
          onJumpLine={jumpToLine}
          highlights={highlightLines ?? undefined}
          searchLines={searchMatches}
        />
      </div>

      <Dialog open={tokenSheet !== null} onOpenChange={(open) => !open && setTokenSheet(null)}>
        <DialogContent className="max-sm:bottom-0 max-sm:top-auto max-sm:max-h-[70dvh] max-sm:w-full max-sm:max-w-none max-sm:translate-y-0 max-sm:rounded-b-none sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-mono text-sm">
              {t("callgraph.openToken", { name: tokenSheet?.site.token ?? "" })}
            </DialogTitle>
            <DialogDescription className="font-mono text-[10px]">
              {tokenSheet?.mode === "menu"
                ? `${tokenSheet.site.path}:${tokenSheet.site.line}`
                : t("callgraph.pick")}
            </DialogDescription>
          </DialogHeader>
          {tokenSheet?.mode === "menu" ? (
            <div className="grid gap-2">
              <Button
                variant="outline"
                className="h-10 justify-start gap-2"
                onClick={() => {
                  const site = tokenSheet.site
                  setTokenSheet(null)
                  void openDefinitions(site)
                }}
              >
                <FileCode2 className="size-4" />
                {t("callgraph.goToDef")}
                {(tokenSheet.site.targets?.length ?? 0) > 1 && (
                  <Badge variant="outline" className="ml-auto text-[9px]">
                    {tokenSheet.site.targets?.length}
                  </Badge>
                )}
              </Button>
              <Button
                variant="outline"
                className="h-10 justify-start gap-2"
                onClick={() => {
                  const site = tokenSheet.site
                  setTokenSheet(null)
                  chooseCalls(site)
                }}
              >
                <ListTree className="size-4" />
                {t("callgraph.showCalls")}
                {(tokenSheet.site.targets?.length ?? 0) > 1 && (
                  <Badge variant="outline" className="ml-auto text-[9px]">
                    {tokenSheet.site.targets?.length}
                  </Badge>
                )}
              </Button>
            </div>
          ) : (
            <div className="max-h-[50dvh] space-y-1 overflow-y-auto">
              {(tokenSheet?.site.targets ?? []).map((target) => (
                <button
                  key={target.id}
                  type="button"
                  onClick={() => {
                    const mode = tokenSheet?.mode
                    setTokenSheet(null)
                    if (mode === "calls") setCallPanel(target.id)
                    else goTo(target.path, target.line)
                  }}
                  className="flex w-full items-center gap-2 rounded-sm border border-border/50 px-2.5 py-2 text-left transition-colors hover:border-primary/40 hover:bg-primary/5"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-mono text-[11px]">{target.name}</span>
                    <span className="block truncate font-mono text-[10px] text-muted-foreground">
                      {target.path}:{target.line}
                    </span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>

      <CallGraphDialog
        repoId={repo.id}
        functionId={callPanel}
        open={callPanel !== null}
        onOpenChange={(open) => !open && setCallPanel(null)}
        onNavigate={goTo}
      />

      <Dialog open={filtersOpen} onOpenChange={setFiltersOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("coverage.filtersTitle")}</DialogTitle>
            <DialogDescription>{t("coverage.filtersDesc")}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={patternsText}
            onChange={(event) => setPatternsText(event.target.value)}
            rows={8}
            spellCheck={false}
            className="font-mono text-[11px]"
            placeholder={"vendor/\ndir:node_modules\npackage:internal/generated\n**/*_test.go"}
          />
          <p className="font-mono text-[10px] text-muted-foreground">
            vendor/ · dir:frontend/node_modules · package:internal/generated · **/*_test.go
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setFiltersOpen(false)}>
              {t("coverage.cancel")}
            </Button>
            <Button onClick={() => void saveFilters()} disabled={savingFilters}>
              {savingFilters && <Loader2 className="animate-spin" />}
              {t("coverage.save")}
            </Button>
          </DialogFooter>
          <p className="font-mono text-[10px] text-muted-foreground">共 {patternsText.split("\n").filter(Boolean).length} 条规则 · 输入时下方即时生效预览</p>
        </DialogContent>
      </Dialog>
      <Dialog open={detailsOpen} onOpenChange={setDetailsOpen}>
        <DialogContent className="max-w-lg max-h-[70dvh] flex flex-col">
          <DialogHeader>
            <DialogTitle>未覆盖函数 · {coverage?.functions.filter((f) => !f.reviewed).length ?? 0}</DialogTitle>
            <DialogDescription>点击跳转到对应行，勾选即标记已读。最多显示 100 条。</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto space-y-1 pr-1">
            {(coverage?.functions.filter((f) => !f.reviewed) ?? []).slice(0, 100).map((fn) => (
              <div key={fn.id} className="flex items-center gap-2 rounded-sm border border-border/50 px-2 py-1.5 text-xs">
                <span className="min-w-0 flex-1 truncate font-mono">{fn.name} <span className="text-muted-foreground">:{fn.startLine}</span></span>
                {fn.changed && <Badge variant="secondary" className="text-[9px]">{t("coverage.changedBadge")}</Badge>}
                <Button variant="ghost" size="sm" className="h-6 text-[11px]" onClick={() => { setDetailsOpen(false); jumpToLine(fn.startLine) }}>跳转</Button>
                <Button variant="outline" size="sm" className="h-6 text-[11px]" disabled={reviewing === fn.id} onClick={() => void toggleReviewed(fn)}><Check className="size-3" /> 标记</Button>
              </div>
            ))}
            {(coverage?.functions.filter((f) => !f.reviewed).length ?? 0) === 0 && (
              <p className="py-6 text-center text-sm text-muted-foreground">全部已覆盖 🎉</p>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function CoverageMetric({
  label,
  reviewed,
  total,
  percent,
}: {
  label: string
  reviewed: number
  total: number
  percent: number
}) {
  return (
    <span className="inline-flex items-center gap-1.5 text-[10px] text-muted-foreground">
      <span className="uppercase tracking-[0.08em]">{label}</span>
      <span className="font-mono text-foreground/80">
        {formatNumber(reviewed)}/{formatNumber(total)}
      </span>
      <span className="font-mono text-primary">{percent}%</span>
    </span>
  )
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
}
