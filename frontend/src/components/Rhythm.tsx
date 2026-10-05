import { useEffect, useState } from "react"
import { motion, useReducedMotion } from "motion/react"
import { Meetings, length, type Rhythm } from "../api"
import { colourOf } from "../colours"
import { locale, t } from "../i18n"

const out = [0.22, 1, 0.36, 1] as const

const hours = (h: number) =>
  h < 1 ? length(h * 3600) : `${h.toLocaleString(locale(), { maximumFractionDigits: h < 10 ? 1 : 0 })} ${t("год")}`

/**
 * How the time goes, drawn from what was recorded and nothing else: hours in
 * meetings by week, when in the week they happen, and who spoke. The bars and
 * the grid cover the last twelve weeks whatever window is chosen above; the
 * voices follow the window.
 */
export default function Rhythm({ days }: { days: number }) {
  const [rhythm, setRhythm] = useState<Rhythm | null>(null)

  useEffect(() => {
    let alive = true
    Meetings.Rhythm(days)
      .then((r) => alive && setRhythm(r as Rhythm))
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [days])

  if (!rhythm || !rhythm.weeks.some((w) => w.meetings)) return null
  return (
    // A container cannot be styled by its own query, hence the wrapper.
    <div className="@container/rhythm">
      <section
        aria-label={t("Ритм")}
        className="grid grid-cols-1 gap-x-5 gap-y-6 @[560px]/rhythm:grid-cols-2"
      >
        <Weeks weeks={rhythm.weeks} />
        <Clock clock={rhythm.clock} />
        <Voices voices={rhythm.voices} days={days} />
      </section>
    </div>
  )
}

function Panel({
  title,
  note,
  wide,
  children,
}: {
  title: string
  note?: string
  wide?: boolean
  children: React.ReactNode
}) {
  return (
    <div className={wide ? "col-span-full" : ""}>
      <h2 className="mb-1.5 flex items-baseline justify-between gap-3 text-[10.5px] font-semibold uppercase tracking-wider text-faint">
        {title}
        {note && (
          <span className="font-normal normal-case tracking-normal tabular-nums">
            {note}
          </span>
        )}
      </h2>
      <div className="rounded-panel border border-line/60 bg-surface/40 px-4 py-3.5">
        {children}
      </div>
    </div>
  )
}

function Weeks({ weeks }: { weeks: Rhythm["weeks"] }) {
  const still = useReducedMotion()
  const top = Math.max(...weeks.map((w) => w.hours), 0.5)
  const now = weeks[weeks.length - 1]
  const mean = weeks.reduce((n, w) => n + w.hours, 0) / weeks.length
  const label = (iso: string) =>
    new Date(iso).toLocaleDateString(locale(), { day: "numeric", month: "short" })
  return (
    <Panel
      wide
      title={t("Години в нарадах за тиждень")}
      note={t("цей тиждень {now}, у середньому {mean}", {
        now: hours(now.hours),
        mean: hours(mean),
      })}
    >
      <div className="flex h-24 items-end gap-1.5">
        {weeks.map((w, i) => (
          <div
            key={w.start}
            className="group relative flex h-full flex-1 items-end"
            title={`${t("тиждень з {date}", { date: label(w.start) })} · ${hours(w.hours)} · ${t("{n} нарад", { n: w.meetings })}${
              w.decisions ? ` · ${t("{n} рішень", { n: w.decisions })}` : ""
            }${w.commitments ? ` · ${t("{n} домовленостей", { n: w.commitments })}` : ""}`}
          >
            <motion.div
              className={`w-full origin-bottom rounded-t-[3px] ${
                i === weeks.length - 1 ? "bg-accent" : "bg-accent/45 group-hover:bg-accent/70"
              }`}
              style={{ height: `${Math.max((w.hours / top) * 100, w.meetings ? 3 : 0)}%` }}
              initial={still ? false : { scaleY: 0 }}
              animate={{ scaleY: 1 }}
              transition={{ duration: 0.5, delay: i * 0.025, ease: out }}
            />
          </div>
        ))}
      </div>
      <div className="mt-1.5 flex gap-1.5 text-[10px] text-faint">
        {weeks.map((w, i) => (
          <span key={w.start} className="flex-1 truncate tabular-nums">
            {i % 3 === 0 ? label(w.start) : ""}
          </span>
        ))}
      </div>
    </Panel>
  )
}

function Clock({ clock }: { clock: number[][] }) {
  const busy = clock.flatMap((row, d) =>
    row.map((m, h) => (m > 0 ? { d, h } : null)).filter(Boolean),
  ) as { d: number; h: number }[]
  const from = Math.min(8, ...busy.map((b) => b.h))
  const to = Math.max(19, ...busy.map((b) => b.h))
  const cols = Array.from({ length: to - from + 1 }, (_, i) => from + i)
  const top = Math.max(...clock.flat(), 1)
  // 2026-10-05 is a Monday.
  const day = (d: number) =>
    new Date(2026, 9, 5 + d).toLocaleDateString(locale(), { weekday: "short" })
  return (
    <Panel title={t("Коли відбуваються наради")} note={t("12 тижнів")}>
      <div
        className="grid items-center gap-[3px]"
        style={{ gridTemplateColumns: `1.6rem repeat(${cols.length}, minmax(0, 1fr))` }}
      >
        {clock.map((row, d) => (
          <div key={d} className="contents">
            <span className="text-[10px] text-faint">{day(d)}</span>
            {cols.map((h) => (
              <span
                key={h}
                className="h-[18px] rounded-[3px] bg-raised/60"
                style={
                  row[h] > 0
                    ? {
                        background: `color-mix(in oklch, var(--color-accent) ${Math.round(18 + (row[h] / top) * 82)}%, transparent)`,
                      }
                    : undefined
                }
                title={
                  row[h] > 0
                    ? `${day(d)} ${h}:00 · ${length(row[h] * 60)}`
                    : undefined
                }
              />
            ))}
          </div>
        ))}
        <span />
        {cols.map((h) => (
          <span key={h} className="text-center text-[9px] tabular-nums text-faint">
            {h % 3 === 0 ? h : ""}
          </span>
        ))}
      </div>
    </Panel>
  )
}

function Voices({ voices, days }: { voices: Rhythm["voices"]; days: number }) {
  const still = useReducedMotion()
  const total = voices.reduce((n, v) => n + v.seconds, 0)
  const shown = voices.slice(0, 6)
  return (
    <Panel
      title={t("Хто говорив")}
      note={days === 1 ? t("сьогодні") : t("{n} днів", { n: days })}
    >
      {total === 0 ? (
        <p className="text-[11.5px] text-faint">{t("За цей період ніхто не говорив у записаних нарадах.")}</p>
      ) : (
        <ul className="space-y-2">
          {shown.map((v, i) => {
            const name = v.speaker || t("Без імені")
            const colour = colourOf(v.speaker || "?")
            return (
              <li key={v.speaker} title={`${name} · ${length(v.seconds)}`}>
                <div className="flex items-baseline justify-between gap-3 text-[11.5px]">
                  <span className="truncate">{name}</span>
                  <span className="tabular-nums text-faint">
                    {Math.round((v.seconds / total) * 100)}%
                  </span>
                </div>
                <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-raised/70">
                  <motion.div
                    className="h-full origin-left rounded-full"
                    style={{ width: `${(v.seconds / total) * 100}%`, background: v.speaker ? colour : "var(--color-faint)" }}
                    initial={still ? false : { scaleX: 0 }}
                    animate={{ scaleX: 1 }}
                    transition={{ duration: 0.5, delay: i * 0.05, ease: out }}
                  />
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}
