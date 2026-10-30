import { useCallback, useEffect, useRef, useState } from "react"
import { api } from "@/lib/api"
import { isActive } from "@/lib/utils"
import type { Repo } from "@/lib/types"

export function useRepo(id: string | undefined) {
  const [repo, setRepo] = useState<Repo | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const alive = useRef(true)

  const load = useCallback(
    async (quiet = false) => {
      if (!id) return
      if (!quiet) setLoading(true)
      try {
        const next = await api.getRepo(id)
        if (!alive.current) return
        setRepo(next)
        setError(null)
      } catch (err) {
        if (!alive.current) return
        setError(err instanceof Error ? err.message : "Failed to load repository")
      } finally {
        if (alive.current && !quiet) setLoading(false)
      }
    },
    [id],
  )

  useEffect(() => {
    alive.current = true
    setRepo(null)
    setLoading(true)
    void load()
    return () => {
      alive.current = false
    }
  }, [load])

  const status = repo?.status
  useEffect(() => {
    if (!id || !status || !isActive(status)) return
    const timer = window.setInterval(() => {
      void load(true)
    }, 1300)
    return () => window.clearInterval(timer)
  }, [id, status, load])

  const regenerate = useCallback(async () => {
    if (!id) return
    try {
      const next = await api.regenerate(id)
      setRepo(next)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start regeneration")
    }
  }, [id])

  return { repo, setRepo, error, loading, refresh: load, regenerate }
}
