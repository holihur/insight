export interface DiagramRef {
  key: string
  label: string
}

const SKIP = new Set([
  "graph", "flowchart", "sequencediagram", "classdiagram", "statediagram",
  "subgraph", "end", "direction", "tb", "lr", "rl", "bt", "note", "loop",
  "alt", "else", "opt", "par", "and", "rect", "activate", "deactivate",
  "classdef", "class", "click", "style", "linkstyle", "participant", "actor",
])

/**
 * Extracts human labels from a Mermaid sequence or flow diagram so they can be
 * matched against the repository call index. We parse the source text instead
 * of the rendered SVG, which keeps this cheap and immune to Mermaid internals.
 */
export function extractDiagramRefs(chart: string): DiagramRef[] {
  const refs: DiagramRef[] = []
  const seen = new Set<string>()

  const add = (raw?: string) => {
    if (!raw) return
    let label = raw
      .replace(/<br\s*\/?>/gi, " ")
      .replace(/[`*_]/g, "")
      .replace(/\s+/g, " ")
      .trim()
      .replace(/^["']|["']$/g, "")
      .replace(/["']$/, "")
    label = label.replace(/^["']/, "")
    if (label.length < 3 || label.length > 80) return
    if (!/[A-Za-z]/.test(label)) return
    const key = label.toLowerCase()
    if (SKIP.has(key) || seen.has(key)) return
    seen.add(key)
    refs.push({ key, label })
  }

  for (const line of chart.split(/\r?\n/)) {
    const text = line.trim()
    if (!text || text.startsWith("%%")) continue

    const participant = /^(?:participant|actor)\s+([\w.$-]+)(?:\s+as\s+(.+))?$/.exec(text)
    if (participant) {
      add(participant[1])
      add(participant[2])
      continue
    }

    const message = /^[^\s:]+?\s*(?:-->>|->>|-->|->|-\)|--\)|-x|--x)\s*[^\s:]+?\s*:\s*(.+)$/.exec(text)
    if (message) {
      add(message[1])
      continue
    }

    const edgeLabel = /\|([^|]+)\|/.exec(text)
    if (edgeLabel) add(edgeLabel[1])

    const node = /[\w.-]+\s*(?:\[([^\]]+)\]|\(([^)]+)\)|\{([^}]+)\})/g
    for (let match = node.exec(text); match; match = node.exec(text)) {
      add(match[1] || match[2] || match[3])
    }

    const subgraph = /^subgraph\s+(.+)$/.exec(text)
    if (subgraph) add(subgraph[1])
  }

  return refs.slice(0, 40)
}
