import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(n >= 10_000_000 ? 0 : 1) + "m"
  if (n >= 1_000) return (n / 1_000).toFixed(n >= 10_000 ? 0 : 1) + "k"
  return String(n)
}

export function relativeTime(value: string | number | Date, locale = "en"): string {
  const date = typeof value === "string" ? new Date(value) : new Date(value)
  const seconds = Math.round((Date.now() - date.getTime()) / 1000)
  try {
    const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
    if (seconds < 45) return rtf.format(0, "second")
    if (seconds < 90) return rtf.format(-1, "minute")
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) return rtf.format(-minutes, "minute")
    const hours = Math.round(minutes / 60)
    if (hours < 24) return rtf.format(-hours, "hour")
    const days = Math.round(hours / 24)
    if (days < 30) return rtf.format(-days, "day")
    const months = Math.round(days / 30)
    if (months < 12) return rtf.format(-months, "month")
    return rtf.format(-Math.round(months / 12), "year")
  } catch {
    if (seconds < 45) return "just now"
    const minutes = Math.max(1, Math.round(seconds / 60))
    if (minutes < 60) return `${minutes} min ago`
    const hours = Math.round(minutes / 60)
    if (hours < 24) return `${hours} hour${hours === 1 ? "" : "s"} ago`
    const days = Math.round(hours / 24)
    return `${days} day${days === 1 ? "" : "s"} ago`
  }
}

export function truncateMiddle(value: string, max = 42): string {
  if (value.length <= max) return value
  const half = Math.floor((max - 1) / 2)
  return `${value.slice(0, half)}…${value.slice(-half)}`
}

export function initials(value: string): string {
  return value
    .split(/[-_/\s]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("")
}

export const statusMeta: Record<
  string,
  { label: string; tone: string; dot: string }
> = {
  queued: {
    label: "Queued",
    tone: "bg-muted text-muted-foreground border-border",
    dot: "bg-muted-foreground",
  },
  cloning: {
    label: "Cloning",
    tone: "bg-[#d9b45c]/10 text-[#9a6b0f] dark:text-[#d9b45c] border-[#d9b45c]/25",
    dot: "bg-[#d9b45c]",
  },
  indexing: {
    label: "Indexing",
    tone: "bg-[#b4552d]/10 text-[#8a4b2a] dark:text-[#d98d63] border-[#b4552d]/25",
    dot: "bg-[#b4552d]",
  },
  planning: {
    label: "Planning",
    tone: "bg-[#d9b45c]/10 text-[#9a6b0f] dark:text-[#e2c176] border-[#d9b45c]/25",
    dot: "bg-[#d9b45c]",
  },
  generating: {
    label: "Writing",
    tone: "bg-[#d9b45c]/10 text-[#9a6b0f] dark:text-[#e2c176] border-[#d9b45c]/25",
    dot: "bg-[#d9b45c]",
  },
  ready: {
    label: "Ready",
    tone: "bg-[#7f9366]/10 text-[#4e6b3a] dark:text-[#a8bd86] border-[#7f9366]/30",
    dot: "bg-[#8fa872]",
  },
  error: {
    label: "Failed",
    tone: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
    dot: "bg-rose-500",
  },
}

export function isActive(status: string): boolean {
  return ["queued", "cloning", "indexing", "planning", "generating"].includes(status)
}

export function languageColor(name: string): string {
  const palette: Record<string, string> = {
    Go: "#00add8",
    TypeScript: "#3178c6",
    JavaScript: "#f1e05a",
    Python: "#3572a5",
    Rust: "#dea584",
    Java: "#b07219",
    Ruby: "#701516",
    "C++": "#f34b7d",
    C: "#555555",
    "C#": "#178600",
    PHP: "#4f5d95",
    Swift: "#f05138",
    Kotlin: "#a97bff",
    Dart: "#00b4ab",
    Vue: "#41b883",
    HTML: "#e34c26",
    CSS: "#563d7c",
    Shell: "#89e051",
    Markdown: "#083fa1",
    JSON: "#292929",
    YAML: "#cb171e",
  }
  return palette[name] ?? "#8b5cf6"
}


/** UUID v4 helper that also works outside secure contexts (plain HTTP on a LAN). */
export function randomId(prefix = ""): string {
  const cryptoObj = globalThis.crypto
  if (typeof cryptoObj?.randomUUID === "function") {
    return prefix + cryptoObj.randomUUID()
  }
  if (typeof cryptoObj?.getRandomValues === "function") {
    const bytes = new Uint8Array(16)
    cryptoObj.getRandomValues(bytes)
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("")
    return `${prefix}${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }
  return `${prefix}${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`
}
