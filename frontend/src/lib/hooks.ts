import { useCallback, useEffect, useState } from 'react'
import { errMsg } from './api'
import { useStore } from '../store'

export function useWorkspace() {
  const open = useStore((s) => s.open)
  return open?.workspace ?? null
}

/** Runs an async loader when deps change, exposing loading/error/reload. */
export function useAsync<T>(fn: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [n, setN] = useState(0)
  useEffect(() => {
    let live = true
    setLoading(true)
    fn().then((d) => live && (setData(d), setError(''))).catch((e) => live && setError(errMsg(e))).finally(() => live && setLoading(false))
    return () => { live = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, n])
  const reload = useCallback(() => setN((x) => x + 1), [])
  return { data, error, loading, reload }
}
