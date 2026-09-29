import { useEffect, useState } from "react"
import { motion } from "motion/react"
import { Pause, Play, Square } from "lucide-react"
import { Meetings, clock, recording, type Line, type Listening } from "../api"

/**
 * The strip that floats under the menu bar while a meeting is recorded.
 *
 * It does the one thing the recorder cannot: have the people on the call told.
 * So it asks for that until somebody says it is done, then shows what is being
 * heard, and keeps Pause and Stop next to the meeting window rather than behind
 * it. It is a window of its own that never takes the keyboard from the call.
 */
export default function Strip() {
  const [state, setState] = useState<Listening | null>(null)
  const [told, setTold] = useState(false)
  const [last, setLast] = useState<Line | null>(null)

  useEffect(() => {
    let alive = true
    const read = () => {
      Meetings.Listening()
        .then((s) => alive && setState(s as Listening))
        .catch(() => {})
      Meetings.Live()
        .then((l) => alive && setLast(((l as Line[]) ?? []).at(-1) ?? null))
        .catch(() => {})
    }
    read()
    const timer = setInterval(read, 1000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  if (!recording(state)) return null
  const held = state!.phase === "held"

  return (
    <motion.div
      className="strip"
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
    >
      <span
        className="flex shrink-0 items-center gap-2"
        aria-label={`${held ? "Пауза" : "Запис"} ${clock(state!.elapsed)}`}
      >
        <span className="relative flex size-2">
          {!held && (
            <span className="breathe absolute inset-0 rounded-full bg-warn" />
          )}
          <span
            className={`relative size-2 rounded-full ${held ? "bg-faint" : "bg-warn"}`}
          />
        </span>
        <span className={`tabular-nums ${held ? "text-soft" : "text-text"}`}>
          {clock(state!.elapsed)}
        </span>
      </span>

      <p className="strip-say" aria-live="polite">
        {!told ? (
          <>
            <span className="truncate">
              Попередьте учасників, що зустріч записується
            </span>
            <button onClick={() => setTold(true)} className="strip-told">
              Попереджено
            </button>
          </>
        ) : held ? (
          <span className="truncate text-soft">
            Пауза: сказане зараз не записується
          </span>
        ) : last ? (
          <span className="truncate text-soft">
            <span className={last.who === "you" ? "text-accent" : "text-good"}>
              {last.who === "you" ? "Ви: " : "Співрозмовники: "}
            </span>
            {last.text}
          </span>
        ) : (
          <span className="text-faint">Слухаю</span>
        )}
      </p>

      <span className="flex shrink-0 items-center gap-1">
        <button
          onClick={() => Meetings.Hold(!held).catch(() => {})}
          aria-label={held ? "Продовжити запис" : "Призупинити запис"}
          title={
            held
              ? "Продовжити запис"
              : "Призупинити: сказане під час паузи не записується"
          }
          className="strip-button"
        >
          {held ? (
            <Play size={13} fill="currentColor" />
          ) : (
            <Pause size={13} fill="currentColor" />
          )}
        </button>
        <button
          onClick={() => Meetings.Record().catch(() => {})}
          aria-label="Зупинити запис"
          title="Зупинити запис"
          className="strip-button"
        >
          <Square size={11} fill="currentColor" />
        </button>
      </span>
    </motion.div>
  )
}
