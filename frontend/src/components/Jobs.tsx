import { useEffect, useRef, useState } from "react"
import { RotateCcw, X } from "lucide-react"
import { Meetings, clock, type Recording } from "../api"
import { draft, drop, useDrafts, useNow, working, type Draft } from "../drafts"
import { t } from "../i18n"

type Job = {
  id: number
  title: string
  state: "working" | "ready" | "failed"
  /** What is happening, in two words, and then in full. */
  verb: string
  what: string
  /** How far a transcription has got, 0..1. */
  share?: number
  draft?: Draft
}

/**
 * Work going on behind the screen, in the corner of the title bar: summaries
 * somebody asked for and the recording the queue is busy with. It shows only
 * while there is something to show, on every screen, and each line opens its
 * meeting.
 */
export default function Jobs({ onOpen }: { onOpen: (id: number) => void }) {
  const drafts = useDrafts()
  const [queue, setQueue] = useState<Recording[]>([])
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  const now = useNow(drafts.some(working))
  useEffect(() => {
    let alive = true
    const read = () =>
      Meetings.Working()
        .then((r) => alive && setQueue((r as Recording[]) ?? []))
        .catch(() => {})
    read()
    const timer = setInterval(read, 2000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])
  useEffect(() => {
    if (!open) return
    const away = (e: PointerEvent) =>
      !box.current?.contains(e.target as Node) && setOpen(false)
    const key = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false)
    document.addEventListener("pointerdown", away)
    window.addEventListener("keydown", key)
    return () => {
      document.removeEventListener("pointerdown", away)
      window.removeEventListener("keydown", key)
    }
  }, [open])

  const jobs: Job[] = [
    ...queue.map((r): Job => {
      const reading = r.status === "transcribing"
      return {
        id: r.id,
        title: r.title,
        state: "working",
        verb: reading ? t("Розшифровую") : t("Готую підсумок"),
        what: reading
          ? t("Розшифровую · {n}%", { n: Math.round(r.progress * 100) })
          : t("Готую підсумок"),
        share: reading ? r.progress : undefined,
      }
    }),
    ...drafts.map((d): Job => {
      const done = d.first ? t("Підсумок готовий") : t("Нова редакція готова")
      return d.problem
        ? { id: d.id, title: d.title, draft: d, state: "failed", verb: t("Підсумок не вдався"), what: d.problem }
        : d.summary
          ? { id: d.id, title: d.title, draft: d, state: "ready", verb: done, what: done }
          : {
              id: d.id,
              title: d.title,
              draft: d,
              state: "working",
              verb: t("Готую підсумок"),
              what: t("Готую підсумок · {time}", { time: clock((now - d.began) / 1000) }),
            }
    }),
  ]
  useEffect(() => {
    if (!jobs.length) setOpen(false)
  }, [jobs.length])
  if (!jobs.length) return null

  const running = jobs.filter((j) => j.state === "working"),
    ready = jobs.filter((j) => j.state === "ready"),
    failed = jobs.filter((j) => j.state === "failed"),
    lead = running[0] ?? ready[0] ?? jobs[0]
  const label =
    running.length > 1
      ? t("У роботі: {n}", { n: running.length })
      : running.length === 1 || jobs.length === 1
        ? `${lead.verb} · ${lead.title}`
        : ready.length
          ? t("Готово: {n}", { n: ready.length })
          : t("Не вдалося: {n}", { n: jobs.length })

  return (
    <div ref={box} className="jobs no-drag">
      <button
        className="jobs-pill"
        aria-expanded={open}
        aria-haspopup="true"
        title={t("Фонові завдання")}
        onClick={() => setOpen(!open)}
      >
        <Mark job={lead} />
        <span>{label}</span>
        {lead.state !== "ready" && ready.length > 0 && (
          <span className="jobs-count ready" aria-label={t("Готово: {n}", { n: ready.length })}>
            {ready.length}
          </span>
        )}
        {lead.state !== "failed" && failed.length > 0 && (
          <span className="jobs-count failed" aria-label={t("Не вдалося: {n}", { n: failed.length })}>
            {failed.length}
          </span>
        )}
      </button>
      {open && (
        <ul className="jobs-list" aria-label={t("Фонові завдання")}>
          {jobs.map((j) => (
            <li key={`${j.draft ? "d" : "q"}${j.id}`} className={j.state}>
              <button
                className="jobs-open"
                onClick={() => {
                  setOpen(false)
                  onOpen(j.id)
                }}
              >
                <Mark job={j} />
                <span>
                  <b>{j.title}</b>
                  <small>{j.what}</small>
                </span>
              </button>
              {j.draft && j.state === "failed" && (
                <button
                  className="ui-icon"
                  aria-label={t("Спробувати ще раз")}
                  title={t("Спробувати ще раз")}
                  onClick={() => draft(j.id, j.title, !!j.draft?.first)}
                >
                  <RotateCcw size={13} />
                </button>
              )}
              {j.draft && j.state !== "working" && (
                <button
                  className="ui-icon"
                  aria-label={t("Прибрати зі списку")}
                  title={t("Прибрати зі списку")}
                  onClick={() => drop(j.id)}
                >
                  <X size={13} />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/** Working turns, a transcription fills its ring, done is green, failed coral. */
function Mark({ job }: { job: Job }) {
  if (job.state !== "working") return <i className={`jobs-dot ${job.state}`} aria-hidden />
  return (
    <svg className={`jobs-ring ${job.share === undefined ? "spin" : ""}`} viewBox="0 0 16 16" aria-hidden>
      <circle cx="8" cy="8" r="6" />
      <circle
        cx="8"
        cy="8"
        r="6"
        pathLength={1}
        style={{ strokeDashoffset: job.share === undefined ? 0.7 : 1 - job.share }}
      />
    </svg>
  )
}
