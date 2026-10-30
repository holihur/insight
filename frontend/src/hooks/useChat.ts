import { useCallback, useEffect, useRef, useState } from "react"
import { streamChat } from "@/lib/api"
import { randomId } from "@/lib/utils"
import type { ChatEvent, ChatMessage, ChatPart } from "@/lib/types"

function sessionKey(repoId: string) {
  return `code-insight-chat:${repoId}`
}

function newSessionId() {
  return randomId()
}

function updateAssistant(
  messages: ChatMessage[],
  id: string,
  update: (message: ChatMessage) => ChatMessage,
): ChatMessage[] {
  return messages.map((message) => (message.id === id ? update(message) : message))
}

function appendText(parts: ChatPart[], text: string, replace: boolean): ChatPart[] {
  if (!text) return parts
  const next = [...parts]
  const last = next[next.length - 1]
  if (last?.type === "text") {
    next[next.length - 1] = {
      type: "text",
      text: replace ? text : last.text + text,
    }
    return next
  }
  next.push({ type: "text", text })
  return next
}

function upsertTool(
  parts: ChatPart[],
  id: string,
  name: string,
  args?: Record<string, unknown>,
): ChatPart[] {
  const next = [...parts]
  const index = next.findIndex((part) => part.type === "tool" && part.id === id)
  if (index >= 0) {
    const existing = next[index]
    if (existing.type === "tool") {
      next[index] = { ...existing, name, args: args ?? existing.args }
    }
    return next
  }
  next.push({ type: "tool", id, name, args, status: "running" })
  return next
}

function finishTool(parts: ChatPart[], id: string, name: string, result?: string): ChatPart[] {
  const next = [...parts]
  let index = next.findIndex((part) => part.type === "tool" && part.id === id)
  if (index < 0) {
    for (let i = next.length - 1; i >= 0; i--) {
      const part = next[i]
      if (part.type === "tool" && part.status === "running" && part.name === name) {
        index = i
        break
      }
    }
  }
  if (index >= 0) {
    const existing = next[index]
    if (existing.type === "tool") {
      next[index] = { ...existing, status: "done", result: result ?? existing.result }
    }
  }
  return next
}

export function useChat(repoId: string | undefined) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [sessionId, setSessionId] = useState<string>(() =>
    repoId ? window.localStorage.getItem(sessionKey(repoId)) ?? "" : "",
  )
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const sessionRef = useRef(sessionId)

  useEffect(() => {
    sessionRef.current = sessionId
  }, [sessionId])

  useEffect(() => {
    setMessages([])
    const stored = repoId ? window.localStorage.getItem(sessionKey(repoId)) ?? "" : ""
    setSessionId(stored)
    sessionRef.current = stored
    return () => {
      abortRef.current?.abort()
    }
  }, [repoId])

  const handleEvent = useCallback(
    (event: ChatEvent, assistantId: string) => {
      if (event.type === "meta") {
        setSessionId(event.sessionId)
        sessionRef.current = event.sessionId
        if (repoId) window.localStorage.setItem(sessionKey(repoId), event.sessionId)
        return
      }
      if (event.type === "done") {
        setStreaming(false)
        setMessages((prev) =>
          updateAssistant(prev, assistantId, (message) => ({
            ...message,
            streaming: false,
          })),
        )
        return
      }
      if (event.type === "error") {
        setStreaming(false)
        setMessages((prev) =>
          updateAssistant(prev, assistantId, (message) => ({
            ...message,
            error: event.message,
            streaming: false,
          })),
        )
        return
      }
      setMessages((prev) =>
        updateAssistant(prev, assistantId, (message) => {
          switch (event.type) {
            case "token":
              return { ...message, parts: appendText(message.parts, event.text, false) }
            case "text":
              return { ...message, parts: appendText(message.parts, event.text, true) }
            case "tool_call":
              return {
                ...message,
                parts: upsertTool(message.parts, event.id, event.name, event.args),
              }
            case "tool_result":
              return {
                ...message,
                parts: finishTool(message.parts, event.id, event.name, event.result),
              }
            default:
              return message
          }
        }),
      )
    },
    [repoId],
  )

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim()
      if (!repoId || !trimmed || streaming) return
      const assistantId = randomId()
      const userMessage: ChatMessage = {
        id: randomId(),
        role: "user",
        parts: [{ type: "text", text: trimmed }],
      }
      const assistantMessage: ChatMessage = {
        id: assistantId,
        role: "assistant",
        parts: [],
        streaming: true,
      }
      setMessages((prev) => [...prev, userMessage, assistantMessage])
      setStreaming(true)
      const controller = new AbortController()
      abortRef.current = controller

      try {
        const sid = sessionRef.current || newSessionId()
        if (!sessionRef.current) {
          setSessionId(sid)
          sessionRef.current = sid
        }
        await streamChat(
          repoId,
          { message: trimmed, sessionId: sid },
          (event) => handleEvent(event, assistantId),
          controller.signal,
        )
        setStreaming(false)
        setMessages((prev) =>
          updateAssistant(prev, assistantId, (message) => ({
            ...message,
            streaming: false,
          })),
        )
      } catch (err) {
        if (controller.signal.aborted) {
          setStreaming(false)
          setMessages((prev) =>
            updateAssistant(prev, assistantId, (message) => ({
              ...message,
              streaming: false,
            })),
          )
          return
        }
        const message = err instanceof Error ? err.message : "Chat failed"
        setStreaming(false)
        setMessages((prev) =>
          updateAssistant(prev, assistantId, (m) => ({
            ...m,
            error: message,
            streaming: false,
          })),
        )
      } finally {
        abortRef.current = null
      }
    },
    [repoId, streaming, handleEvent],
  )

  const stop = useCallback(() => {
    abortRef.current?.abort()
    setStreaming(false)
  }, [])

  const reset = useCallback(() => {
    abortRef.current?.abort()
    setMessages([])
    setStreaming(false)
    const sid = newSessionId()
    setSessionId(sid)
    sessionRef.current = sid
    if (repoId) window.localStorage.setItem(sessionKey(repoId), sid)
  }, [repoId])

  return { messages, send, stop, reset, streaming, sessionId }
}
