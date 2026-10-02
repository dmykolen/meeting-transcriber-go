import { useMemo, useState } from "react"
import type { Turn } from "../api"
import { clock } from "../api"
import { t } from "../i18n"

/**
 * Who held the floor, in order — a row each.
 *
 * This was one row for everybody, a segment per turn in that person's colour.
 * On a real meeting — twenty-three minutes, six people, 281 turns — that is two
 * hundred hairlines in six colours packed into a strip eight pixels tall: a
 * barcode, and the loudest object on the screen. Merging consecutive turns by
 * one person helped and did not fix it, because the meeting genuinely was that
 * much back-and-forth. The failure was not the merging, it was asking one row
 * to carry six voices.
 *
 * A row each fixes it by construction. Every row holds one colour, so no row
 * can ever become a barcode however finely the talking is cut; the shape of the
 * conversation is in the *pattern between* rows, which is where it always was.
 * One long bar against five empty rows is somebody presenting. Five rows all
 * dashed is an argument. That is legible at a glance and the strip never was.
 *
 * Click anywhere to send the audio and the transcript there.
 */
export default function Voices({
  turns,
  colours,
  at,
  lit,
  length,
  onJump,
  onLight,
}: {
  turns: Turn[]
  colours: Map<string, string>
  /** The recording's full length in seconds, so this shares the player's axis. */
  length: number
  /** Where the player is, in seconds. */
  at: number
  /** A speaker to pick out, or null for all of them. */
  lit: string | null
  onJump: (seconds: number) => void
  onLight: (speaker: string | null) => void
}) {
  const [over, setOver] = useState<{
    who: string
    start: number
    end: number
  } | null>(null)

  const { people, span } = useMemo(() => {
    const held = new Map<
      string,
      { who: string; said: number; bars: [number, number][] }
    >()
    for (const t of turns) {
      const who = t.speaker || "—"
      const row = held.get(who) ?? { who, said: 0, bars: [] }
      const last = row.bars[row.bars.length - 1]
      // A breath is not a new turn. Merging inside a row keeps the DOM small
      // and stops a sentence read in three gulps looking like three remarks.
      if (last && t.start - last[1] < 1.5) last[1] = Math.max(last[1], t.end)
      else row.bars.push([t.start, Math.max(t.end, t.start + 0.5)])
      row.said += Math.max(t.end - t.start, 0)
      held.set(who, row)
    }
    const ranked = [...held.values()].sort((a, b) => b.said - a.said)
    // The axis is the recording, from nothing to its full length — not from
    // the first word to the last. The waveform under the player is drawn over
    // the whole file, and two strips on one screen showing the same instant in
    // two different places is the thing this map exists to make legible. When
    // the length is not known yet, the last word stands in for it.
    const last = turns.reduce((m, t) => Math.max(m, t.end), 1)
    return { people: ranked, span: Math.max(length || last, 1) }
  }, [turns, length])

  if (people.length === 0) return null
  const where = Math.min(Math.max(at / span, 0), 1) * 100

  return (
    <div className="voice-map" onMouseLeave={() => setOver(null)}>
      <div className="relative">
        {people.map((p) => {
          const colour = colours.get(p.who) || "var(--color-line)"
          const dim = lit !== null && lit !== p.who
          return (
            <div
              key={p.who}
              className="group flex items-center gap-2 py-[1.5px]"
            >
              <button
                onClick={() => onLight(lit === p.who ? null : p.who)}
                className={`w-[86px] shrink-0 truncate text-right text-[9.5px] leading-none transition-colors ${
                  dim ? "text-faint/50" : "text-faint group-hover:text-soft"
                }`}
              >
                {p.who}
              </button>
              <span
                className={`relative h-[7px] flex-1 rounded-full transition-colors ${
                  dim ? "bg-raised/30" : "bg-raised/70"
                }`}
              >
                {p.bars.map(([s, e], i) => (
                  <button
                    key={i}
                    onClick={() => onJump(s)}
                    onMouseEnter={() =>
                      setOver({ who: p.who, start: s, end: e })
                    }
                    aria-label={`${p.who}, ${clock(s)}`}
                    style={{
                      left: `${(s / span) * 100}%`,
                      width: `max(2px, ${((e - s) / span) * 100}%)`,
                      background: colour,
                      opacity: dim ? 0.2 : 1,
                    }}
                    className="absolute inset-y-0 rounded-full transition-opacity duration-200 hover:!opacity-100"
                  />
                ))}
              </span>
            </div>
          )
        })}

        {/* One playhead through every row, so the rows read as one instrument. */}
        <span
          style={{
            left: `calc(86px + 0.5rem + ${where}% - ${where / 100} * (86px + 0.5rem))`,
          }}
          className="pointer-events-none absolute inset-y-0 w-px bg-text/70 transition-[left] duration-200"
        />
      </div>

      {/* One readout for the whole block, in space it already occupies. A native
          title waits half a second and arrives in a font nothing else here uses. */}
      <p className="mt-1 h-3 pl-[94px] text-[10px] leading-3 text-faint">
        {over && (
          <span className="tabular-nums">
            {clock(over.start)} — {clock(over.end)} ·{" "}
            {t("{n} с", { n: Math.round(over.end - over.start) })}
          </span>
        )}
      </p>
    </div>
  )
}
