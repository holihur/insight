import { Children, isValidElement, useMemo, useRef, type ReactNode, Fragment, createElement } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeHighlight from "rehype-highlight"
import { Check, Copy, FileCode2 } from "lucide-react"
import { useState } from "react"
import { cn } from "@/lib/utils"
import { Mermaid } from "@/components/repo/Mermaid"

function CodeFrame({ children, language }: { children: ReactNode; language?: string }) {
  const ref = useRef<HTMLPreElement>(null)
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    const text = ref.current?.innerText ?? ""
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      // Clipboard is unavailable (e.g. insecure context).
    }
  }

  return (
    <div className="group/code relative my-4">
      {language && language !== "text" && (
        <span className="absolute left-2.5 top-2.5 z-10 rounded-sm border border-border/60 bg-background/80 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-[0.12em] text-muted-foreground backdrop-blur">
          {language}
        </span>
      )}
      <button
        type="button"
        onClick={copy}
        aria-label="Copy code"
        className="absolute right-2.5 top-2.5 z-10 inline-flex items-center gap-1 rounded-md border border-border/70 bg-background/80 px-2 py-1 text-[10px] text-muted-foreground opacity-0 backdrop-blur transition-opacity hover:text-foreground group-hover/code:opacity-100"
      >
        {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
        {copied ? "Copied" : "Copy"}
      </button>
      <pre ref={ref} className="!my-0">
        {children}
      </pre>
    </div>
  )
}

const FILE_RE =
  /\b((?:[a-zA-Z0-9_\-.]+\/)+[a-zA-Z0-9_\-.]+\.[a-zA-Z0-9]{1,8})(?::(\d+)(?:-(\d+))?)?\b/g

function FileChip({
  path,
  lineStart,
  lineEnd,
  onClick,
}: {
  path: string
  lineStart?: number
  lineEnd?: number
  onClick?: (path: string, lines?: [number, number]) => void
}) {
  const label = lineStart ? `${path}:${lineStart}${lineEnd ? `-${lineEnd}` : ""}` : path
  if (!onClick) {
    return (
      <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.82em]">{label}</code>
    )
  }
  return (
    <button
      type="button"
      onClick={() => onClick(path, lineStart ? [lineStart, lineEnd ?? lineStart] : undefined)}
      title={`在代码视图打开 ${label}`}
      className="inline-flex items-center gap-1 rounded-sm border border-primary/25 bg-primary/8 px-1.5 py-0.5 font-mono text-[0.8em] text-primary transition-colors hover:bg-primary/15"
    >
      <FileCode2 className="size-3" />
      {label}
    </button>
  )
}

export function linkifyText(text: string, onFileOpen?: (path: string, lines?: [number, number]) => void): ReactNode[] {
  const parts: ReactNode[] = []
  let last = 0
  let m: RegExpExecArray | null
  FILE_RE.lastIndex = 0
  while ((m = FILE_RE.exec(text)) !== null) {
    const [full, path, s, e] = m
    const idx = m.index
    if (idx > last) parts.push(text.slice(last, idx))
    parts.push(
      <FileChip
        key={`${path}-${s}-${e}-${idx}`}
        path={path}
        lineStart={s ? Number(s) : undefined}
        lineEnd={e ? Number(e) : undefined}
        onClick={onFileOpen}
      />,
    )
    last = idx + full.length
  }
  if (last < text.length) parts.push(text.slice(last))
  return parts.length ? parts : [text]
}

function Linkified({ children, onFileOpen }: { children: ReactNode; onFileOpen?: (p: string, l?: [number, number]) => void }) {
  const arr = Children.toArray(children)
  return (
    <>
      {arr.map((c, i) => {
        if (typeof c === "string") return <Fragment key={i}>{linkifyText(c, onFileOpen)}</Fragment>
        return <Fragment key={i}>{c}</Fragment>
      })}
    </>
  )
}

export function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .slice(0, 64)
}

function headingId(children: ReactNode): string {
  const text = Children.toArray(children)
    .map((c) => (typeof c === "string" ? c : isValidElement(c) ? String((c.props as { children?: ReactNode })?.children ?? "") : ""))
    .join("")
  return slugify(text) || "section"
}

export function Markdown({
  content,
  className,
  onFileOpen,
  repoId,
}: {
  content: string
  className?: string
  onFileOpen?: (path: string, lines?: [number, number]) => void
  repoId?: string
}) {
  // Generated wikis often label real code fences as ```text / ```plain. That
  // stops rehype-highlight from auto-detecting, so the block renders without
  // colors. Drop those placeholder languages and let detection do its job.
  const normalized = useMemo(
    () => content.replace(/```[ \t]*(?:text|plain|plaintext|txt|none)[ \t]*\r?\n/gi, "```\n"),
    [content],
  )

  return (
    <div className={cn("markdown prose prose-neutral dark:prose-invert max-w-none", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[[rehypeHighlight, { detect: true, ignoreMissing: true }]]}
        components={{
          pre({ children }) {
            const child = Children.toArray(children)[0]
            if (isValidElement(child)) {
              const props = child.props as { className?: string; children?: ReactNode }
              const language = /language-([\w-]+)/.exec(props.className ?? "")?.[1]
              if (language === "mermaid") {
                return (
                  <Mermaid
                    chart={String(props.children ?? "").trim()}
                    repoId={repoId}
                    onOpenFile={onFileOpen}
                  />
                )
              }
              return <CodeFrame language={language}>{children}</CodeFrame>
            }
            return <CodeFrame>{children}</CodeFrame>
          },
          // inline code: if it looks like a file path, make it clickable
          code(props) {
            const { children, className: cls, ...rest } = props as { children?: ReactNode; className?: string }
            const txt = String(children ?? "")
            const isInline = !cls
            if (isInline && FILE_RE.test(txt)) {
              FILE_RE.lastIndex = 0
              const m = FILE_RE.exec(txt)
              if (m && m[0] === txt) {
                const [, path, s, e] = m
                return createElement("span", rest, createElement(FileChip as never, { path, lineStart: s ? Number(s) : undefined, lineEnd: e ? Number(e) : undefined, onClick: onFileOpen } as never))
              }
              FILE_RE.lastIndex = 0
            }
            return createElement("code", { className: cls, ...rest }, children)
          },
          p({ children, ...props }) {
            return (
              <p {...props}>
                <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
              </p>
            )
          },
          li({ children, ...props }) {
            return (
              <li {...props}>
                <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
              </li>
            )
          },
          td({ children, ...props }) {
            return (
              <td {...props}>
                <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
              </td>
            )
          },
          blockquote({ children, ...props }) {
            return (
              <blockquote {...props}>
                <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
              </blockquote>
            )
          },
          h2({ children, ...props }) {
            const id = headingId(children)
            return (
              <h2 id={id} {...props} className={cn("group scroll-mt-20", (props as { className?: string }).className)}>
                <a href={`#${id}`} className="no-underline group-hover:text-primary">
                  <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
                </a>
              </h2>
            )
          },
          h3({ children, ...props }) {
            const id = headingId(children)
            return (
              <h3 id={id} {...props} className={cn("group scroll-mt-20", (props as { className?: string }).className)}>
                <a href={`#${id}`} className="no-underline group-hover:text-primary">
                  <Linkified onFileOpen={onFileOpen}>{children}</Linkified>
                </a>
              </h3>
            )
          },
          a({ href, children, ...props }) {
            // internal file: protocol
            if (href?.startsWith("file:")) {
              const raw = href.slice(5)
              const [p, linePart] = raw.split(":")
              const lines = linePart ? linePart.split("-").map(Number) : undefined
              return (
                <button
                  type="button"
                  onClick={() => onFileOpen?.(p, lines as never)}
                  className="inline-flex items-center gap-1 rounded-sm border border-primary/20 bg-primary/5 px-1 py-0.5 font-mono text-[0.85em] text-primary hover:bg-primary/10"
                >
                  <FileCode2 className="size-3" />
                  {children}
                </button>
              )
            }
            const external = href?.startsWith("http")
            return (
              <a
                href={href}
                target={external ? "_blank" : undefined}
                rel={external ? "noreferrer noopener" : undefined}
                {...props}
              >
                {children}
              </a>
            )
          },
        }}
      >
        {normalized}
      </ReactMarkdown>
    </div>
  )
}
