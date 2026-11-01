import { useEffect, useState } from "react"
import { AnimatePresence, motion } from "motion/react"
import { Check, Mic, PenLine } from "lucide-react"
import { Meetings, clock, recording, why, type Listening } from "../api"
import { t, tr } from "../i18n"

/**
 * The Note button, above Record at the foot of the rail. Press, say it, press
 * again: only the microphone is recorded, and the note is transcribed ahead of
 * the queue. While something else records the microphone is taken, so the
 * button takes a typed line for that recording instead.
 */
export default function Dictate({ state }: { state: Listening }) {
  const [typing, setTyping] = useState(false)
  const [text, setText] = useState("")
  const [problem, setProblem] = useState("")
  // "Added" on the button for a moment after a typed line is kept; why a press
  // did nothing, beside the button, which has no room for a sentence.
  const [flash, setFlash] = useState("")
  const [refused, setRefused] = useState("")
  const dictating = state.dictating
  const other = recording(state) && !dictating
  const usable =
    state.phase === "listening" || recording(state) || (state.phase === "paused" && !!state.until)
  const missed = !other && !!state.missed && Date.now() - Date.parse(state.missed) < 5000

  useEffect(() => {
    if (!flash && !refused) return
    const timer = setTimeout(() => (setFlash(""), setRefused("")), 4000)
    return () => clearTimeout(timer)
  }, [flash, refused])
  useEffect(() => {
    if (!other) setTyping(false)
  }, [other])

  const press = () => {
    if (other) {
      setProblem("")
      setTyping(!typing)
      return
    }
    Meetings.Dictate().catch((e) => setRefused(why(e)))
  }
  const save = async () => {
    if (!text.trim()) return
    try {
      await Meetings.Jot(text)
      setText("")
      setTyping(false)
      setFlash(t("Додано"))
    } catch (e) {
      setProblem(why(e))
    }
  }

  const Icon = dictating || flash ? Check : other ? PenLine : Mic
  const label = missed ? t("Тиша") : flash || (dictating ? t("Готово") : t("Нотатка"))
  const title = !usable
    ? state.phase === "broken"
      ? tr(state.problem) || t("Не вдалося відкрити мікрофон")
      : state.phase === "opening"
        ? t("Відкриваю мікрофон. macOS може запитати дозвіл.")
        : t("Слухання вимкнено. Увімкніть його в параметрах.")
    : missed
      ? t("Нічого не почув: нотатку не збережено")
      : dictating
        ? t("Завершити нотатку")
        : other
          ? t("Іде запис: додати до нього текстову нотатку")
          : t("Записати голосову нотатку: лише мікрофон. Натисніть ще раз, щоб завершити.")

  return (
    <div className="relative mt-auto w-full">
      <button
        onClick={press}
        disabled={!usable}
        title={title}
        aria-label={title}
        aria-expanded={other ? typing : undefined}
        className={`jot group flex w-full flex-col items-center gap-1 rounded-xl px-1 py-2.5 transition-colors enabled:hover:bg-raised disabled:cursor-default ${dictating ? "jot-live" : ""} ${missed ? "jot-missed" : ""}`}
      >
        <span className="relative flex size-[18px] items-center justify-center">
          {dictating && <span className="breathe absolute -inset-1 rounded-full bg-accent/25" />}
          <Icon
            size={16}
            strokeWidth={1.9}
            className={`relative z-10 ${dictating ? "text-accent" : missed ? "text-warn" : "text-faint group-enabled:group-hover:text-soft"}`}
          />
        </span>
        <span
          aria-live="polite"
          className={`w-full truncate text-center text-[9.5px] font-medium ${dictating ? "text-accent" : missed ? "text-warn" : "text-faint"}`}
        >
          {label}
        </span>
      </button>
      <AnimatePresence>
        {refused && (
          <motion.p
            role="alert"
            className="jot-panel jot-refused"
            initial={{ opacity: 0, x: -8 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, transition: { duration: 0.12 } }}
          >
            {refused}
          </motion.p>
        )}
        {typing && other && (
          <motion.form
            className="jot-panel"
            initial={{ opacity: 0, x: -8 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -6, transition: { duration: 0.12 } }}
            transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
            onSubmit={(e) => {
              e.preventDefault()
              void save()
            }}
            onKeyDown={(e) => {
              if (e.key === "Escape") setTyping(false)
            }}
          >
            <label htmlFor="jot-text">
              {t("Нотатка до цього запису")} <span className="tabular-nums">· {clock(state.elapsed)}</span>
            </label>
            <textarea
              id="jot-text"
              autoFocus
              rows={3}
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault()
                  void save()
                }
              }}
            />
            {problem && (
              <p role="alert" className="reader-error">
                {problem}
              </p>
            )}
            <footer>
              <small>{t("З’явиться в записі на цій хвилині")}</small>
              <button className="ui-primary" disabled={!text.trim()}>
                {t("Зберегти")}
              </button>
            </footer>
          </motion.form>
        )}
      </AnimatePresence>
    </div>
  )
}
