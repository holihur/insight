import type { CallSite } from "@/lib/types"

export interface TokenSpan {
  start: number
  end: number
  site: CallSite
}

/** Byte offsets of the first character of each visible line (1-based line -> index). */
export function lineStartOffsets(lines: string[]): number[] {
  const starts = new Array<number>(lines.length + 1).fill(0)
  let offset = 0
  for (let i = 0; i < lines.length; i++) {
    starts[i + 1] = offset
    offset += lines[i].length + 1
  }
  return starts
}

/** Convert call sites (line/col) into absolute character spans. */
export function siteSpans(sites: CallSite[], starts: number[], visibleLines: number): TokenSpan[] {
  const spans: TokenSpan[] = []
  for (const site of sites) {
    if (site.line < 1 || site.line > visibleLines) continue
    if (!site.token) continue
    const start = (starts[site.line] ?? 0) + site.col
    const end = start + Math.max(1, site.endCol - site.col)
    spans.push({ start, end, site })
  }
  spans.sort((a, b) => a.start - b.start || a.end - b.end)
  return spans
}

const ENTITY = /^&(#\d+|#x[0-9a-fA-F]+|[a-zA-Z]+);/

/**
 * Wrap token spans into the highlighted HTML. Works on text offsets, skipping
 * tags and counting entities as one character, so it stays aligned with the
 * plain source text that highlight.js received.
 */
export function injectTokenSpans(html: string, spans: TokenSpan[]): string {
  if (!spans.length) return html
  const opens = new Map<number, TokenSpan[]>()
  const closes = new Map<number, TokenSpan[]>()
  for (const span of spans) {
    const list = opens.get(span.start)
    if (list) list.push(span)
    else opens.set(span.start, [span])
    const ends = closes.get(span.end)
    if (ends) ends.push(span)
    else closes.set(span.end, [span])
  }

  let out = ""
  let i = 0
  let raw = 0
  const flush = (at: number) => {
    const ending = closes.get(at)
    if (ending) for (let n = 0; n < ending.length; n++) out += "</span>"
    const starting = opens.get(at)
    if (starting) {
      for (const span of starting) {
        const ambiguous = span.site.confidence === "multiple" || (span.site.targets?.length ?? 0) > 1
        const cls = ["cg-token", span.site.kind === "def" ? "cg-token-def" : "", ambiguous ? "cg-token-ambiguous" : ""]
          .filter(Boolean)
          .join(" ")
        out += `<span class="${cls}" data-cg-key="${span.site.line}:${span.site.col}">`
      }
    }
  }

  while (i < html.length) {
    const ch = html[i]
    if (ch === "<") {
      const close = html.indexOf(">", i)
      if (close < 0) {
        out += html.slice(i)
        break
      }
      out += html.slice(i, close + 1)
      i = close + 1
      continue
    }
    flush(raw)
    if (ch === "&") {
      const match = ENTITY.exec(html.slice(i))
      if (match && match[0].length <= 12) {
        out += match[0]
        i += match[0].length
        raw += 1
        continue
      }
    }
    out += ch
    i += 1
    raw += 1
  }
  flush(raw)
  return out
}
