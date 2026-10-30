import {
  BookOpen,
  ChevronRight,
  FileCode2,
  Folder,
  GitBranch,
  Network,
  Sparkles,
} from "lucide-react"
import { MazeIcon } from "@/components/icons/MazeIcon"

const files = [
  { name: "cmd/", dir: true, active: false },
  { name: "internal/", dir: true, active: true, open: true },
  { name: "  agent/", dir: true, active: false },
  { name: "  api/", dir: true, active: false },
  { name: "  store/", dir: true, active: false },
  { name: "main.go", dir: false, active: false },
  { name: "go.mod", dir: false, active: false },
]

const pages = [
  { title: "Overview", icon: BookOpen, active: true },
  { title: "Getting Started", icon: Sparkles, active: false },
  { title: "Architecture", icon: Network, active: false },
]

export function HeroPreview() {
  return (
    <div className="relative mx-auto w-full max-w-lg animate-fade-up [animation-delay:150ms]">
      <div className="absolute -inset-6 -z-10 rounded-[2rem] bg-gradient-to-br from-amber-500/12 via-orange-700/8 to-transparent blur-2xl" />
      <div className="relative overflow-hidden rounded-sm border border-primary/20 bg-card/75 shadow-[0_36px_90px_-36px_rgba(0,0,0,0.85)] backdrop-blur">
        <MazeIcon className="host-watermark size-40" />
        <div className="flex items-center gap-2 border-b border-border/60 bg-muted/30 px-3 py-2.5">
          <div className="flex gap-1.5">
            <span className="size-2.5 rounded-full bg-[#b4552d]/80" />
            <span className="size-2.5 rounded-full bg-[#d9b45c]/80" />
            <span className="size-2.5 rounded-full bg-[#7f9366]/80" />
          </div>
          <div className="mx-auto flex items-center gap-1.5 rounded-sm border border-border/50 bg-background/60 px-2.5 py-1 font-mono text-[9px] uppercase tracking-[0.1em] text-muted-foreground">
            <GitBranch className="size-2.5" />
            <span>gin-gonic/gin</span>
            <span className="text-border">/</span>
            <span className="text-primary">synced</span>
          </div>
          <MazeIcon className="size-3.5 text-primary" />
        </div>

        <div className="grid grid-cols-[130px_1fr]">
          <div className="border-r border-border/60 bg-muted/15 p-2.5">
            <div className="mb-2 flex items-center gap-1 px-1 font-mono text-[8px] uppercase tracking-[0.16em] text-muted-foreground/70">
              <FileCode2 className="size-2.5" /> Source
            </div>
            <div className="space-y-0.5 font-mono text-[10px]">
              {files.map((file) => (
                <div
                  key={file.name}
                  className={`flex items-center gap-1 rounded px-1 py-0.5 ${
                    file.active ? "bg-accent text-accent-foreground" : "text-muted-foreground"
                  }`}
                >
                  {file.dir ? (
                    <Folder className="size-2.5 shrink-0 opacity-60" />
                  ) : (
                    <FileCode2 className="size-2.5 shrink-0 opacity-50" />
                  )}
                  <span className="truncate">{file.name}</span>
                </div>
              ))}
            </div>
            <div className="mt-3 border-t border-border/50 pt-2">
              <div className="mb-1.5 px-1 font-mono text-[8px] uppercase tracking-[0.16em] text-muted-foreground/70">
                Wiki
              </div>
              {pages.map((page) => (
                <div
                  key={page.title}
                  className={`flex items-center gap-1.5 rounded px-1 py-0.5 text-[10px] ${
                    page.active ? "border-l border-primary bg-primary/8 font-medium text-primary" : "border-l border-transparent text-muted-foreground"
                  }`}
                >
                  <page.icon className="size-2.5 shrink-0" />
                  <span className="truncate">{page.title}</span>
                  {page.active && <ChevronRight className="ml-auto size-2.5 opacity-50" />}
                </div>
              ))}
            </div>
          </div>

          <div className="min-w-0 p-4">
            <div className="flex items-center gap-1.5 text-[9px] text-muted-foreground">
              <BookOpen className="size-2.5" /> Wiki
              <span className="opacity-40">/</span>
              <span>Architecture</span>
              <span className="ml-auto inline-flex items-center gap-1 rounded-sm border border-primary/25 bg-primary/8 px-1.5 py-0.5 font-mono text-[7px] uppercase tracking-[0.12em] text-primary">
                grounded
              </span>
            </div>
            <h3 className="mt-2.5 font-serif text-[19px] font-semibold leading-none tracking-tight">Architecture</h3>
            <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
              How the runtime, agents and tools fit together.
            </p>

            <div className="mt-3 rounded-lg border border-border/60 bg-background/40 p-2.5">
              <svg viewBox="0 0 240 92" className="w-full" aria-hidden>
                <defs>
                  <linearGradient id="hero-line" x1="0" x2="1">
                    <stop offset="0" stopColor="#d9b45c" />
                    <stop offset="1" stopColor="#b4552d" />
                  </linearGradient>
                </defs>
                <rect x="82" y="4" width="76" height="18" rx="5" fill="url(#hero-line)" opacity="0.18" stroke="url(#hero-line)" strokeWidth="0.7" />
                <text x="120" y="16" textAnchor="middle" fontSize="7" fill="currentColor" className="text-foreground">runner</text>
                <rect x="6" y="42" width="62" height="18" rx="5" className="fill-muted stroke-border" strokeWidth="0.7" />
                <text x="37" y="54" textAnchor="middle" fontSize="7" fill="currentColor" className="text-muted-foreground">agent</text>
                <rect x="88" y="42" width="64" height="18" rx="5" className="fill-muted stroke-border" strokeWidth="0.7" />
                <text x="120" y="54" textAnchor="middle" fontSize="7" fill="currentColor" className="text-muted-foreground">session</text>
                <rect x="172" y="42" width="62" height="18" rx="5" className="fill-muted stroke-border" strokeWidth="0.7" />
                <text x="203" y="54" textAnchor="middle" fontSize="7" fill="currentColor" className="text-muted-foreground">tools</text>
                <rect x="48" y="72" width="60" height="16" rx="5" className="fill-muted stroke-border" strokeWidth="0.7" />
                <text x="78" y="83" textAnchor="middle" fontSize="6.5" fill="currentColor" className="text-muted-foreground">read_file</text>
                <rect x="132" y="72" width="60" height="16" rx="5" className="fill-muted stroke-border" strokeWidth="0.7" />
                <text x="162" y="83" textAnchor="middle" fontSize="6.5" fill="currentColor" className="text-muted-foreground">search_code</text>
                <path d="M120 22v14M37 42l70-14M120 42V36M203 42l-70-14" stroke="url(#hero-line)" strokeWidth="0.9" fill="none" opacity="0.7" />
                <path d="M78 60v6M162 60v6" stroke="url(#hero-line)" strokeWidth="0.9" fill="none" opacity="0.7" />
              </svg>
            </div>

            <div className="mt-3 space-y-1.5">
              <div className="h-1.5 w-full rounded-full bg-muted" />
              <div className="h-1.5 w-[92%] rounded-full bg-muted" />
              <div className="h-1.5 w-[75%] rounded-full bg-muted" />
            </div>
            <div className="mt-3 flex items-center gap-1.5 text-[9px] text-muted-foreground">
              <span className="size-1.5 animate-pulse rounded-full bg-primary" />
              Writing 6 sections from source…
            </div>
          </div>
        </div>

        <div className="border-t border-border/60 bg-muted/20 px-3 py-2">
          <div className="flex items-center gap-1.5 rounded-lg border border-border/60 bg-background/60 px-2.5 py-1.5 text-[10px] text-muted-foreground">
            <MazeIcon className="size-3 text-primary" />
            Ask about the request lifecycle…
          </div>
        </div>
      </div>
    </div>
  )
}
