import { Suspense, lazy, type ComponentProps } from "react"
import type { ChatPanel as ChatPanelComponent } from "@/components/repo/ChatPanel"

// Type-only import above keeps ChatPanel (and markdown rendering) out of the
// repository chunk; it is fetched only when the chat panel is actually shown.
const ChatPanel = lazy(() =>
  import("@/components/repo/ChatPanel").then((m) => ({ default: m.ChatPanel })),
)

type ChatPanelProps = ComponentProps<typeof ChatPanelComponent>

export function ChatPanelHost(props: ChatPanelProps) {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-0 flex-1 items-center justify-center">
          <span className="size-5 animate-spin rounded-full border-2 border-primary/25 border-t-primary" />
        </div>
      }
    >
      <ChatPanel {...props} />
    </Suspense>
  )
}
