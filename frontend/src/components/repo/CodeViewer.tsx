import { useEffect, useMemo, useRef, useState } from "react"
import {
  Check,
  Copy,
  ExternalLink,
  FileCode2,
  FileText,
  Loader2,
  SlidersHorizontal,
  TriangleAlert,
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
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
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
import { cn, formatNumber } from "@/lib/utils"
import type { Coverage, CoverageFunction, FileContent, Repo } from "@/lib/types"
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

const MAX_LINES = 2500

export function CodeViewer({
  repo,
  path,
}: {
  repo: Repo
  path: string | null
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
  const [patternsText, setPatternsText] = useState((repo.ignore ?? []).join("\n"))
  const [savingFilters, setSavingFilters] = useState(false)
  const requestRef = useRef(0)
  const coverageRequestRef = useRef(0)
  const codeScrollRef = useRef<HTMLDivElement>(null)

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
    setPatternsText((repo.ignore ?? []).join("\n"))
  }, [repo.ignore])

  const raw = file?.content ?? ""
  const lines = useMemo(() => (raw ? raw.split("\n") : []), [raw])
  const visible = lines.slice(0, MAX_LINES)
  const functionsByStart = useMemo(() => {
    const map = new Map<number, CoverageFunction>()
    for (const fn of coverage?.functions ?? []) {
      map.set(fn.startLine, fn)
    }
    return map
  }, [coverage])

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
        <div className="flex min-w-0 items-center gap-1 overflow-x-auto font-mono text-[11px] text-muted-foreground">
          {path.split("/").map((part, index, array) => (
            <span key={index} className="inline-flex shrink-0 items-center gap-1">
              <span className={cn(index === array.length - 1 && "text-foreground")}>{part}</span>
              {index < array.length - 1 && <span className="opacity-40">/</span>}
            </span>
          ))}
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-1">
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
            <div className="ml-auto flex items-center gap-2">
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
        {!loading && error && (
          <div className="m-5 flex items-start gap-2 rounded-xl border border-rose-500/20 bg-rose-500/5 p-4 text-sm text-rose-500">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            {error}
          </div>
        )}
        {!loading && !error && file && (
          <div className="flex min-w-fit text-[12px] leading-[1.7]">
            <div className="sticky left-0 z-10 shrink-0 select-none border-r border-border/60 bg-card/60 py-4 pr-2 font-mono text-[11px] text-muted-foreground/50">
              {visible.map((_, index) => {
                const line = index + 1
                const fn = functionsByStart.get(line)
                return (
                  <div
                    key={line}
                    className={cn(
                      "flex h-[20.4px] items-center justify-end gap-1 pl-1 leading-[20.4px]",
                      fn?.reviewed && "bg-emerald-500/[0.06]",
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
                        }`}
                        disabled={reviewing === fn.id}
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
                    <span className="min-w-6 text-right">{line}</span>
                  </div>
                )
              })}
            </div>
            <pre className="flex-1 overflow-x-auto px-4 py-4 font-mono">
              <code dangerouslySetInnerHTML={{ __html: highlighted }} />
            </pre>
          </div>
        )}
        {!loading && !error && file && lines.length > MAX_LINES && (
          <div className="border-t border-border/60 px-4 py-2 text-[11px] text-muted-foreground">
            {t("code.showing", {
              shown: formatNumber(MAX_LINES),
              total: formatNumber(file.total),
            })}{" "}
            <a href={githubURL} target="_blank" rel="noreferrer" className="text-primary hover:underline">
              {t("code.openFull")}
            </a>
            .
          </div>
        )}
      </div>

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
