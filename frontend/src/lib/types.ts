export type RepoStatus =
  | "queued"
  | "cloning"
  | "indexing"
  | "planning"
  | "generating"
  | "ready"
  | "error"

export interface Language {
  name: string
  files: number
  lines: number
}

export interface Stats {
  files: number
  lines: number
  bytes: number
  languages: Language[] | null
}

export interface Page {
  id: string
  title: string
  slug: string
  description: string
  content: string
  order: number
  status: "pending" | "writing" | "ready" | "error"
}

export interface Wiki {
  title: string
  summary: string
  pages: Page[]
  generatedAt: string
  demo: boolean
  provider: string
}

export interface Repo {
  id: string
  url: string
  webUrl: string
  owner: string
  name: string
  branch: string
  commit: string
  status: RepoStatus
  progress: number
  message: string
  error?: string
  demo: boolean
  provider: string
  createdAt: string
  updatedAt: string
  stats: Stats
  wiki?: Wiki
}

export interface TreeNode {
  name: string
  path: string
  type: "dir" | "file"
  size?: number
  children?: TreeNode[]
}

export interface Module {
  name: string
  files: number
  lines: number
  top: string[] | null
}

export interface IndexData {
  tree: TreeNode
  keyFiles: string[] | null
  entryFiles: string[] | null
  modules: Module[] | null
  stats: Stats
  readme: string
  truncated: boolean
}

export interface FileContent {
  path: string
  content: string
  language: string
  lines: number
  start: number
  end: number
  total: number
}

export type ChatEvent =
  | { type: "meta"; sessionId: string }
  | { type: "token"; text: string }
  | { type: "text"; text: string }
  | { type: "tool_call"; id: string; name: string; args?: Record<string, unknown> }
  | { type: "tool_result"; id: string; name: string; result?: string }
  | { type: "error"; message: string }
  | { type: "done"; sessionId: string }

export type ChatPart =
  | { type: "text"; text: string }
  | {
      type: "tool"
      id: string
      name: string
      args?: Record<string, unknown>
      result?: string
      status: "running" | "done"
    }

export interface ChatMessage {
  id: string
  role: "user" | "assistant"
  parts: ChatPart[]
  error?: string
  streaming?: boolean
}

export interface Health {
  status: string
  provider: string
  model: string
  label: string
  demo: boolean
}
