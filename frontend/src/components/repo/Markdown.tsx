import { Children, isValidElement, useRef, type ReactNode } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeHighlight from "rehype-highlight"
import { Check, Copy } from "lucide-react"
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

export function Markdown({ content, className }: { content: string; className?: string }) {
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
                return <Mermaid chart={String(props.children ?? "").trim()} />
              }
              return <CodeFrame language={language}>{children}</CodeFrame>
            }
            return <CodeFrame>{children}</CodeFrame>
          },
          a({ href, children, ...props }) {
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
        {content}
      </ReactMarkdown>
    </div>
  )
}
