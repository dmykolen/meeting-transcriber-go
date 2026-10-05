import { useEffect, useState } from "react"
import { motion } from "motion/react"
import { Activity, Scale } from "lucide-react"
import { Meetings, clock, many, type Analytics } from "../api"
import { t } from "../i18n"

/**
 * The shape of a meeting: who held the floor, how fast, who was asking rather
 * than telling, and where the hour was dense.
 *
 * Measured from the rows in the database rather than stored, so it follows a
 * speaker being renamed the moment it happens and needs no key, no model and no
 * network. Everything here is a fact about the recording, not an opinion about
 * it — the app does not tell anybody they talked too much.
 */
export default function Shape({
  id,
  colours,
  onJump,
}: {
  id: number
  colours: Map<string, string>
  onJump: (seconds: number) => void
}) {
  const [a, setA] = useState<Analytics | null>(null)
  // A rename changes what Analytics returns, and only the names say so: the
  // map itself is rebuilt on every load of the meeting.
  const names = [...colours.keys()].join("\n")

  useEffect(() => {
    let alive = true
    Meetings.Analytics(id)
      .then((got) => alive && setA(got as Analytics))
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [id, names])

  // One voice is a note, and a bar chart of one person holding 100% of the
  // floor tells nobody anything. The pace and the shape still do, so those stay
  // and the table goes.
  if (!a || a.speakers.length === 0) return null
  const alone = a.speakers.length === 1
  const most = Math.max(...(a.busiest ?? []).map((m) => m.words), 1)
  const stats: [string, string][] = [
    [t("Мовлення"), clock(a.speech)],
    [t("Паузи"), clock(a.silence)],
    ...(a.overlap > 1 ? [[t("Перекриття"), clock(a.overlap)] as [string, string]] : []),
    [t("слів/хв"), String(Math.round(a.pace))],
    ...(alone ? [[t("слів"), String(a.words)] as [string, string]] : []),
  ]

  return (
    <section className="inspector">
      <h2 className="flex items-center gap-1.5 text-[10.5px] font-semibold uppercase tracking-wider text-faint">
        <Activity size={11} /> {t("Голоси й ритм")}
      </h2>

      <dl className="inspector-stats">
        {stats.map(([label, value]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      {a.speakers.length > 1 && (
        <p className="mt-2.5 flex items-center gap-1.5 text-[11px] text-faint">
          <Scale size={11} />
          {a.balance > 0.85
            ? t("Рівномірна участь")
            : a.balance > 0.6
              ? t("Збалансована участь")
              : t("Переважає один голос")}
        </p>
      )}

      {/* Where the words were. A meeting has a shape, and it is usually not flat. */}
      {a.busiest && a.busiest.length > 0 && (
        <div className="mt-5">
          <div className="flex h-16 items-end gap-[2px]">
            {a.busiest.map((m, i) => (
              <motion.button
                key={i}
                onClick={() => onJump(m.at)}
                title={`${clock(m.at)} — ${m.words} ${t("слів")}`}
                initial={{ scaleY: 0 }}
                animate={{ scaleY: 1 }}
                transition={{
                  delay: i * 0.006,
                  duration: 0.35,
                  ease: [0.22, 1, 0.36, 1],
                }}
                style={{ height: `${Math.max((m.words / most) * 100, 4)}%` }}
                className="flex-1 origin-bottom rounded-[2px] bg-accent/35 transition-colors hover:bg-accent"
              />
            ))}
          </div>
          <div className="mt-1 flex justify-between text-[9.5px] tabular-nums text-faint">
            <span>{clock(a.busiest[0].at)}</span>
            <span>{clock(a.busiest[a.busiest.length - 1].at)}</span>
          </div>
        </div>
      )}

      {!alone && (
        <ul className="mt-5 flex flex-col gap-3.5">
          {a.speakers.map((v, i) => {
            const colour = colours.get(v.speaker) ?? "var(--color-accent)"
            return (
              <li key={v.speaker} className="flex gap-2.5">
                <span
                  aria-hidden
                  className="mt-px grid size-7 shrink-0 place-items-center rounded-full text-[12px] font-semibold"
                  style={{ background: `color-mix(in oklch, ${colour} 22%, transparent)`, color: colour }}
                >
                  {v.speaker.trim()[0]?.toUpperCase()}
                </span>
                <div className="min-w-0 flex-1 text-[11.5px]">
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="min-w-0 truncate font-medium" title={v.speaker}>
                      {v.speaker}
                    </span>
                    <span className="shrink-0 text-[13px] font-medium tabular-nums">
                      {Math.round(v.share * 100)}%
                    </span>
                  </div>
                  <span className="mt-1.5 block h-1.5 overflow-hidden rounded-full bg-raised">
                    <motion.span
                      className="block h-full rounded-full"
                      style={{ background: colour }}
                      initial={{ width: 0 }}
                      animate={{ width: `${v.share * 100}%` }}
                      transition={{ delay: 0.1 + i * 0.06, duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
                    />
                  </span>
                  <span className="mt-1 block text-[10px] tabular-nums text-faint">
                    {t("{n} {word} · {pace} wpm", {
                      n: v.turns,
                      word: many(v.turns, "репліка", "репліки", "реплік"),
                      pace: Math.round(v.pace),
                    })}
                    {v.questions > 0 && ` · ${t("питань {n}", { n: v.questions })}`}
                  </span>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
