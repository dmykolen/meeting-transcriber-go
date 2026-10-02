import { useEffect, useState } from "react"
import { motion } from "motion/react"
import { Activity, MessageCircleQuestion, Scale } from "lucide-react"
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

  return (
    <section className="mt-6">
      <h2 className="mb-2 flex items-center gap-1.5 text-[10.5px] font-semibold uppercase tracking-wider text-faint">
        <Activity size={11} /> {t("Голоси й ритм")}
      </h2>

      <div className="rounded-panel border border-line/60 bg-surface/40 px-4 py-3.5">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-faint">
          <span>
            {t("Мовлення")}{" "}
            <b className="font-medium text-soft">{clock(a.speech)}</b>
          </span>
          <span>
            {t("Паузи")}{" "}
            <b className="font-medium text-soft">{clock(a.silence)}</b>
          </span>
          {a.overlap > 1 && (
            <span>
              {t("Перекриття")}{" "}
              <b className="font-medium text-soft">{clock(a.overlap)}</b>
            </span>
          )}
          <span>
            <b className="font-medium text-soft">{Math.round(a.pace)}</b>{" "}
            {t("слів/хв")}
          </span>
          {alone && (
            <span>
              <b className="font-medium text-soft">{a.words}</b> {t("слів")}
            </span>
          )}
          {a.speakers.length > 1 && (
            <span className="flex items-center gap-1">
              <Scale size={11} />
              {a.balance > 0.85
                ? t("Рівномірна участь")
                : a.balance > 0.6
                  ? t("Збалансована участь")
                  : t("Переважає один голос")}
            </span>
          )}
        </div>

        {/* Where the слів were. A meeting has a shape, and it is usually not flat. */}
        {a.busiest && a.busiest.length > 0 && (
          <div className="mt-3 flex h-9 items-end gap-[2px]">
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
                className="flex-1 origin-bottom rounded-[1px] bg-accent/35 transition-colors hover:bg-accent"
              />
            ))}
          </div>
        )}

        {/* Stacked rather than name | bar | figures on one line. This lives in a
            narrow rail now, and three columns of text needed 280 pixels to sit
            in 236 — which clipped every speaker's figures. */}
        {!alone && (
          <div className="mt-3.5 flex flex-col gap-2.5">
            {a.speakers.map((v, i) => (
              <div key={v.speaker} className="text-[11px]">
                <div className="flex items-baseline justify-between gap-2">
                  <span
                    className="min-w-0 truncate font-medium"
                    style={{ color: colours.get(v.speaker) }}
                    title={v.speaker}
                  >
                    {v.speaker}
                  </span>
                  <span className="shrink-0 tabular-nums text-soft">
                    {Math.round(v.share * 100)}%
                  </span>
                </div>
                <span className="mt-1 block h-1.5 overflow-hidden rounded-full bg-raised">
                  <motion.span
                    className="block h-full rounded-full"
                    style={{
                      background:
                        colours.get(v.speaker) ?? "var(--color-accent)",
                    }}
                    initial={{ width: 0 }}
                    animate={{ width: `${v.share * 100}%` }}
                    transition={{
                      delay: 0.1 + i * 0.06,
                      duration: 0.6,
                      ease: [0.22, 1, 0.36, 1],
                    }}
                  />
                </span>
                <span className="mt-1 block text-[9.5px] tabular-nums text-faint">
                  {t("{n} {word} · {pace} wpm", {
                    n: v.turns,
                    word: many(v.turns, "репліка", "репліки", "реплік"),
                    pace: Math.round(v.pace),
                  })}
                </span>
              </div>
            ))}
          </div>
        )}

        {!alone && a.speakers.some((v) => v.questions > 0) && (
          <p className="mt-3 flex items-start gap-1.5 border-t border-line/40 pt-2.5 text-[10.5px] leading-relaxed text-faint">
            <MessageCircleQuestion size={11} className="mt-[3px] shrink-0" />
            <span className="min-w-0">
              {t("Питань:")}{" "}
              {a.speakers
                .filter((v) => v.questions > 0)
                .sort((x, y) => y.questions - x.questions)
                .map((v) => `${v.speaker} ${v.questions}`)
                .join(" · ")}
            </span>
          </p>
        )}
      </div>
    </section>
  )
}
