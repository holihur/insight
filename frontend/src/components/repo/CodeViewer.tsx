import { useEffect, useMemo, useRef, useState } from "react"
import {
  Check,
  Copy,
  ExternalLink,
  FileCode2,
  FileText,
  Loader2,
  TriangleAlert,
} from "lucide-react"
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
import { api } from "@/lib/api"
import { cn, formatNumber } from "@/lib/utils"
import type { FileContent, Repo } from "@/lib/types"

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
  const [file, setFile] = useState<FileContent | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const requestRef = useRef(0)

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

  const raw = file?.content ?? ""
  const lines = useMemo(() => (raw ? raw.split("\n") : []), [raw])
  const visible = lines.slice(0, MAX_LINES)
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

  if (!path) {
    return (
      <div className="grid h-full place-items-center p-8">
        <div className="max-w-sm text-center">
          <div className="mx-auto mb-4 grid size-14 place-items-center rounded-2xl border border-border/70 bg-card/60">
            <FileCode2 className="size-6 text-muted-foreground/60" />
          </div>
          <h2 className="font-semibold">Browse the source</h2>
          <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
            Pick a file from the explorer to read it with syntax highlighting. The agent uses the same
            tree when answering questions.
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
          <Button variant="ghost" size="icon-sm" onClick={copy} aria-label="Copy file">
            {copied ? <Check className="text-emerald-500" /> : <Copy />}
          </Button>
          <Button variant="ghost" size="icon-sm" asChild aria-label="Open on GitHub">
            <a href={githubURL} target="_blank" rel="noreferrer">
              <ExternalLink />
            </a>
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {loading && (
          <div className="space-y-2 p-5">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Loader2 className="size-3.5 animate-spin" /> Loading {path}…
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
            <pre
              aria-hidden
              className="sticky left-0 z-10 select-none border-r border-border/60 bg-card/60 px-3 py-4 text-right font-mono text-muted-foreground/50"
            >
              {visible.map((_, index) => String(index + 1)).join("\n")}
            </pre>
            <pre className="flex-1 overflow-x-auto px-4 py-4 font-mono">
              <code dangerouslySetInnerHTML={{ __html: highlighted }} />
            </pre>
          </div>
        )}
        {!loading && !error && file && lines.length > MAX_LINES && (
          <div className="border-t border-border/60 px-4 py-2 text-[11px] text-muted-foreground">
            Showing the first {formatNumber(MAX_LINES)} of {formatNumber(file.total)} lines.{" "}
            <a href={githubURL} target="_blank" rel="noreferrer" className="text-primary hover:underline">
              Open the full file on GitHub
            </a>
            .
          </div>
        )}
      </div>
    </div>
  )
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
}
