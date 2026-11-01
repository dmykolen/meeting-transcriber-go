import { useEffect, useState, useSyncExternalStore } from "react"
import { Meetings, why, type Summary } from "./api"

/**
 * A summary the model is writing, or has written and nobody has looked at yet.
 *
 * It lives here and not in the reader, which unmounts when another meeting is
 * opened: the model went on working and its answer had nowhere to go, so coming
 * back showed the button as if it had never been pressed.
 */
export type Draft = {
  id: number
  title: string
  /** The meeting had no summary yet. */
  first: boolean
  /** A voice note's text rather than a meeting's summary. */
  note?: boolean
  began: number
  summary?: Summary
  problem?: string
}

let drafts: Draft[] = []
const heard = new Set<() => void>()
const set = (next: Draft[]) => {
  drafts = next
  heard.forEach((f) => f())
}
const listen = (f: () => void) => {
  heard.add(f)
  return () => void heard.delete(f)
}

export const useDrafts = () => useSyncExternalStore(listen, () => drafts)

export const working = (d?: Draft) => !!d && !d.summary && !d.problem

/** Starts a draft unless one is already being written for this meeting. */
export function draft(id: number, title: string, first: boolean, note = false) {
  if (working(drafts.find((d) => d.id === id))) return
  const began = Date.now()
  set([...drafts.filter((d) => d.id !== id), { id, title, first, note, began }])
  // Only the draft that asked takes the answer: one dropped and asked again
  // must not be filled by the call before it.
  const settle = (patch: Partial<Draft>) =>
    set(drafts.map((d) => (d.id === id && d.began === began ? { ...d, ...patch } : d)))
  Meetings.PreviewSummary(id).then(
    (summary) => settle({ summary: summary as Summary }),
    (e) => settle({ problem: why(e) }),
  )
}

export const drop = (id: number) => set(drafts.filter((d) => d.id !== id))

/** The time, a second at a time, while something is counting. */
export function useNow(ticking: boolean) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!ticking) return
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [ticking])
  return now
}
