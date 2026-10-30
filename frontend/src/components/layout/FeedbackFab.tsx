import { MessageCircleQuestion } from "lucide-react"

export function FeedbackFab() {
  return (
    <a
      href="https://github.com/holihur/insight/issues/new?title=%5BFeedback%5D%20&body=%E6%8F%8F%E8%BF%B0%E9%97%AE%E9%A2%98%E6%88%96%E5%BB%BA%E8%AE%AE%EF%BC%9A%0A%0A%E7%8E%AF%E5%A2%83%EF%BC%9A"
      target="_blank"
      rel="noreferrer"
      className="fixed bottom-4 right-4 z-40 inline-flex items-center gap-1.5 rounded-full border border-border bg-background/90 px-3 py-1.5 text-xs shadow-lg backdrop-blur hover:bg-accent"
      title="提交反馈到 GitHub"
      aria-label="提交反馈"
    >
      <MessageCircleQuestion className="size-3.5 text-primary" /> 反馈
    </a>
  )
}
