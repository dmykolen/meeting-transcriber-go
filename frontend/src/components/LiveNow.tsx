import { useEffect, useRef, useState } from "react"
import { AnimatePresence, motion } from "motion/react"
import { ChevronDown, Radio } from "lucide-react"
import { Meetings, clock, recording, type Line, type Listening } from "../api"

/**
 * The meeting, as it happens.
 *
 * The detector already cuts the audio where somebody stopped talking, because
 * it has to know that anyway — so each utterance goes through Whisper as it
 * finishes and lands here a second or two later. Nothing extra is recorded and
 * nothing leaves the machine; this is the same model, on the same audio, a few
 * seconds earlier.
 *
 * It is not the transcript, and says so. Whisper on a four-second fragment has
 * no context, cannot spell a name it has not heard yet, and cannot be given
 * speakers. The real one is written from the file afterwards and replaces this.
 */
export default function LiveNow({ state }: { state: Listening }) {
  const [lines, setLines] = useState<Line[]>([])
  const [open, setOpen] = useState(true)
  const foot = useRef<HTMLDivElement>(null)

  const running = recording(state)

  useEffect(() => {
    if (!running) {
      setLines([])
      return
    }
    let alive = true
    const read = () =>
      Meetings.Live()
        .then((l) => alive && setLines((l as Line[]) ?? []))
        .catch(() => {})
    read()
    const timer = setInterval(read, 1200)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [running])

  useEffect(() => {
    foot.current?.scrollIntoView({ behavior: "smooth", block: "end" })
  }, [lines.length])

  if (!running) return null

  return (
    <motion.section
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
      className="mb-3 overflow-hidden rounded-panel border border-warn/30 bg-warn/[0.06]"
    >
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center gap-2 px-4 py-2.5 text-left"
      >
        <span className="relative flex size-2">
          <span className="breathe absolute inset-0 rounded-full bg-warn" />
          <span className="relative size-2 rounded-full bg-warn" />
        </span>
        <Radio size={13} className="text-warn" />
        <h2 className="text-[12.5px] font-medium text-warn">
          {state.phase === "held"
            ? "Запис на паузі"
            : state.kind === "note"
              ? "Запис нотатки"
              : "Триває запис"}
        </h2>
        <span className="text-[11px] tabular-nums text-faint">
          {clock(state.elapsed)}
        </span>
        {state.phase === "wrapping up" && (
          <span className="text-[11px] text-faint">· тиша {state.quiet} с</span>
        )}
        <ChevronDown
          size={14}
          className={`ml-auto text-faint transition-transform ${open ? "" : "-rotate-90"}`}
        />
      </button>

      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            initial={{ height: 0 }}
            animate={{ height: "auto" }}
            exit={{ height: 0 }}
            transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="max-h-36 overflow-y-auto px-4 pb-3">
              {lines.length === 0 ? (
                <p className="text-[12px] leading-relaxed text-faint">
                  Слухаю. Перші репліки з’являться за кілька секунд.
                </p>
              ) : (
                <div className="flex flex-col gap-1">
                  {lines.map((l, i) => (
                    <motion.p
                      key={`${l.at}-${i}`}
                      initial={{ opacity: 0, x: -6 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ duration: 0.25 }}
                      className="text-[12.5px] leading-[1.45]"
                    >
                      <span className="mr-2 tabular-nums text-[11px] text-faint">
                        {clock(l.at)}
                      </span>
                      <span
                        className={`mr-1.5 text-[11px] font-medium ${
                          l.who === "you" ? "text-accent" : "text-good"
                        }`}
                      >
                        {l.who === "you" ? "Ви" : "Співрозмовники"}
                      </span>
                      <span className="text-soft">{l.text}</span>
                    </motion.p>
                  ))}
                  <div ref={foot} />
                </div>
              )}
              <p className="mt-2 border-t border-warn/15 pt-2 text-[10.5px] leading-relaxed text-faint">
                Попередня розшифровка. Після зустрічі з’явиться остаточний текст
                з учасниками й пунктуацією.
              </p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.section>
  )
}
