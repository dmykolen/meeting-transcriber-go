import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { AnimatePresence, motion } from "motion/react"
import { ArrowUpRight, AudioLines, ListChecks } from "lucide-react"
import { length, type Recording } from "../api"
export default function MeetingPeek({
  recording,
  children,
  onOpen,
  disabled = false,
}: {
  recording: Recording
  children: React.ReactNode
  onOpen: () => void
  disabled?: boolean
}) {
  const root = useRef<HTMLDivElement>(null),
    timer = useRef(0)
  const [position, setPosition] = useState<{
    left: number
    top: number
  } | null>(null)
  const show = () => {
    clearTimeout(timer.current)
    if (disabled) return
    timer.current = window.setTimeout(() => {
      const r = root.current?.getBoundingClientRect()
      if (r)
        setPosition({
          left: Math.max(8, Math.min(r.right + 8, innerWidth - 360)),
          top: Math.max(40, Math.min(r.top, innerHeight - 245)),
        })
    }, 450)
  }
  const leave = () => {
    clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setPosition(null), 180)
  }
  useEffect(() => () => clearTimeout(timer.current), [])
  useEffect(() => {
    if (!position) return
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        clearTimeout(timer.current)
        setPosition(null)
      }
    }
    window.addEventListener("keydown", key)
    return () => window.removeEventListener("keydown", key)
  }, [position])
  return (
    <div ref={root} onPointerEnter={show} onPointerLeave={leave}>
      {children}
      {createPortal(
        <AnimatePresence>
          {position && (
            <motion.aside
              className="meeting-peek"
              style={position}
              initial={{ opacity: 0, x: -8, scale: 0.98 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: -5 }}
              transition={{ duration: 0.2 }}
              onPointerEnter={() => clearTimeout(timer.current)}
              onPointerLeave={leave}
            >
              <header>
                <AudioLines size={14} />
                <span>Швидкий перегляд</span>
                <small>{length(recording.duration)}</small>
              </header>
              <h3>{recording.title}</h3>
              <p>
                {recording.summary?.overview ||
                  ({
                    done: "Підсумку ще немає",
                    queued: "У черзі обробки",
                    transcribing: "Створюється розшифровка",
                    summarising: "Готується підсумок",
                    failed: "Обробка зупинилася",
                  }[recording.status] ??
                    "Підсумку ще немає")}
              </p>
              <footer>
                <span>
                  <ListChecks size={13} />
                  {recording.summary?.action_items?.filter((a) => !a.done)
                    .length || 0}{" "}
                  відкритих справ
                </span>
                <button
                  className="ui-chip"
                  onClick={() => {
                    setPosition(null)
                    onOpen()
                  }}
                >
                  Відкрити <ArrowUpRight size={13} />
                </button>
              </footer>
            </motion.aside>
          )}
        </AnimatePresence>,
        document.body,
      )}
    </div>
  )
}
