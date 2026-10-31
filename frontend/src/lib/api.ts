import type {
  CallResult,
  CallSitesResult,
  ChatEvent,
  Coverage,
  CoverageTree,
  FileContent,
  Health,
  IndexData,
  Repo,
  ResolveMatch,
  TraceResult,
} from "@/lib/types"

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "/api"

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  })
  if (!res.ok) {
    let message = `Request failed (${res.status})`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // ignore
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  health: () => request<Health>("/health"),

  listRepos: async () => {
    const data = await request<{ repos: Repo[] }>("/repos")
    return data.repos ?? []
  },

  getRepo: (id: string) => request<Repo>(`/repos/${id}`),

  createRepo: (url: string, force = false) =>
    request<Repo>("/repos", {
      method: "POST",
      body: JSON.stringify({ url, force }),
    }),

  deleteRepo: (id: string) =>
    request<{ deleted: boolean }>(`/repos/${id}`, { method: "DELETE" }),

  regenerate: (id: string) =>
    request<Repo>(`/repos/${id}/regenerate`, { method: "POST" }),

  getIndex: (id: string) => request<IndexData>(`/repos/${id}/index`),

  getFile: (id: string, path: string, start?: number, end?: number) => {
    const params = new URLSearchParams({ path })
    if (start) params.set("start", String(start))
    if (end) params.set("end", String(end))
    return request<FileContent>(`/repos/${id}/file?${params.toString()}`)
  },

  getCoverageTree: (id: string) => request<CoverageTree>(`/repos/${id}/coverage/tree`),

  getCoverage: (id: string, path?: string) => {
    const params = new URLSearchParams()
    if (path) params.set("path", path)
    const suffix = params.toString()
    return request<Coverage>(`/repos/${id}/coverage${suffix ? `?${suffix}` : ""}`)
  },

  reviewFunction: (id: string, functionId: string, reviewed: boolean) =>
    request<Coverage>(`/repos/${id}/coverage/review`, {
      method: "POST",
      body: JSON.stringify({ functionId, reviewed }),
    }),

  getCalls: (id: string, functionId: string) =>
    request<CallResult>(`/repos/${id}/calls?function=${encodeURIComponent(functionId)}`),

  resolve: (id: string, queries: string[]) =>
    request<{ matches: ResolveMatch[] }>(`/repos/${id}/resolve`, {
      method: "POST",
      body: JSON.stringify({ queries }),
    }),

  getTrace: (id: string, functionId: string, direction: "callees" | "callers", depth = 2, limit = 10) => {
    const params = new URLSearchParams({
      function: functionId,
      direction,
      depth: String(depth),
      limit: String(limit),
    })
    return request<TraceResult>(`/repos/${id}/trace?${params.toString()}`)
  },

  getCallSites: (id: string, path: string) =>
    request<CallSitesResult>(`/repos/${id}/callsites?path=${encodeURIComponent(path)}`),

  updateIgnore: (id: string, patterns: string[]) =>
    request<Repo>(`/repos/${id}/ignore`, {
      method: "PUT",
      body: JSON.stringify({ patterns }),
    }),
}

export async function streamChat(
  repoId: string,
  payload: { message: string; sessionId: string },
  onEvent: (event: ChatEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/repos/${repoId}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    signal,
  })
  if (!res.ok) {
    let message = `Chat failed (${res.status})`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // ignore
    }
    throw new ApiError(res.status, message)
  }
  if (!res.body) throw new ApiError(500, "Streaming is not supported by this browser")

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  const dispatch = (block: string) => {
    const lines = block.split("\n")
    const data = lines
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trim())
      .join("\n")
    if (!data) return
    try {
      onEvent(JSON.parse(data) as ChatEvent)
    } catch {
      // Ignore malformed keep-alives.
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let boundary = buffer.indexOf("\n\n")
    while (boundary !== -1) {
      const block = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      dispatch(block)
      boundary = buffer.indexOf("\n\n")
    }
  }
  buffer += decoder.decode()
  if (buffer.trim()) dispatch(buffer)
}
