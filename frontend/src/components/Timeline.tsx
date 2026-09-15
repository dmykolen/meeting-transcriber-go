import { useMemo, useRef, useState } from "react"
import { ChevronDown } from "lucide-react"
import type { Group, Mark } from "../api"
import { colourOf } from "../colours"

/** A day, as the whole app keys them. */
const key = (d: Date) => d.toISOString().slice(0, 10)
const add = (d: Date, n: number) => new Date(new Date(d).setDate(d.getDate() + n))
const DAY = 86400000

/**
 * The year, as a small instrument.
 *
 * One lane per project, one column per day, one tick per recording. The strip
 * underneath is the only time filter in the app: drag a window across it and
 * every pane below obeys, which is why there is no filter panel anywhere.
 *
 * It draws the span the recordings actually occupy, not a fixed year. Drawing
 * 365 days for a fortnight of meetings put every tick in the last two per cent
 * of the width — a row of hairlines jammed against the right edge, under twelve
 * month labels pointing at nothing. The axis now starts a little before the
 * first recording and ends today, so the ticks are spread across the width they
 * were given, and the labels below count in whatever unit the span deserves.
 *
 * Below 640 px the lanes fold away and the strip stays, because a range you
 * cannot see is still a range you use, while lanes at that width are four
 * pixels tall and mean nothing.
 */
export default function Timeline({
  marks,
  groups,
  range,
  picked,
  lit,
  onRange,
  onPick,
  onLight,
}: {
  marks: Mark[]
  groups: Group[]
  range: [string, string] | null
  picked: number | null
  /** A recording to pick out, so hovering the list shows where it sits in time. */
  lit: number | null
  onRange: (r: [string, string] | null) => void
  onPick: (id: number | null) => void
  onLight: (recording: number | null) => void
}) {
  const strip = useRef<HTMLDivElement>(null)
  const [drag, setDrag] = useState<string | null>(null)
  const [open, setOpen] = useState(true)
  const [hover, setHover] = useState<number | null>(null)

  const { from, days, lanes, ticks } = useMemo(() => {
    const today = new Date()
    today.setHours(0, 0, 0, 0)

    // A fortnight minimum, so a first week of use is not drawn as four columns.
    const oldest = marks.reduce(
      (min, m) => Math.min(min, +new Date(m.started)),
      +today,
    )
    const days = Math.max(Math.ceil((+today - oldest) / DAY) + 2, 14)
    const from = add(today, -(days - 1))

    // Unfiled is a lane like any other: a year that is mostly unfiled should
    // look mostly unfiled.
    const lanes = [...groups, { id: 0, name: "Поза проєктами", count: 0, colour: "" }]
      .map((g) => ({
        ...g,
        colour: g.id ? colourOf(g.name, g.colour) : "var(--color-faint)",
        held: marks.filter((m) => m.folder === g.id),
      }))
      .filter((l) => l.held.length > 0)

    // Days while there are few enough to read, then weeks, then months. Twelve
    // month names over a fortnight was the old failure in the other direction.
    const step = days <= 21 ? 1 : days <= 90 ? 7 : 30
    const ticks: { at: number; name: string }[] = []
    for (let i = 0; i < days; i += step) {
      const d = add(from, i)
      ticks.push({
        at: (i / days) * 100,
        name:
          step === 1
            ? String(d.getDate())
            : step === 7
              ? d.toLocaleDateString("uk", { day: "numeric", month: "short" })
              : d.toLocaleDateString("uk", { month: "short" }),
      })
    }
    return { from, days, lanes, ticks }
  }, [marks, groups])

  /** Where a day sits along the axis, 0..100. */
  const at = (d: Date) => ((+d - +from) / DAY / days) * 100

  const dayUnder = (clientX: number) => {
    const box = strip.current?.getBoundingClientRect()
    if (!box) return null
    const share = Math.min(Math.max((clientX - box.left) / box.width, 0), 0.999)
    return key(add(from, Math.floor(share * days)))
  }

  // The window is dragged by its edges or drawn fresh on empty strip. Pointer
  // capture keeps it tracking outside the element, which is what makes a brush
  // feel solid rather than slippery.
  const grab = (e: React.PointerEvent, edge: "a" | "b" | "new") => {
    const day = dayUnder(e.clientX)
    if (!day) return
    ;(e.target as Element).setPointerCapture(e.pointerId)
    if (edge === "new") onRange([day, day])
    setDrag(edge === "b" ? "b" : edge === "a" ? "a" : "b")
    e.preventDefault()
  }
  const move = (e: React.PointerEvent) => {
    if (!drag || !range) return
    const day = dayUnder(e.clientX)
    if (!day) return
    const [a, b] = drag === "a" ? [day, range[1]] : [range[0], day]
    onRange(a <= b ? [a, b] : [b, a])
  }

  const nice = (iso: string) =>
    new Date(iso).toLocaleDateString("uk", { day: "numeric", month: "short" })

  return (
    <div className="no-drag @container/time flex-none border-b border-line/70 bg-surface/40 px-4 pb-1.5 pt-1.5">
      <div className="flex items-start gap-2">
        <button
          onClick={() => setOpen((o) => !o)}
          aria-expanded={open}
          className="mt-px hidden shrink-0 items-center gap-1 rounded px-1 py-0.5 text-[9px] uppercase tracking-[0.12em] text-faint transition-colors hover:bg-raised hover:text-soft @min-[640px]/time:flex"
        >
          <ChevronDown
            size={10}
            className={`transition-transform duration-200 ${open ? "" : "-rotate-90"}`}
          />
          {days} дн.
        </button>

        <div className="min-w-0 flex-1">
          {/* The lanes. A row transitions its own height rather than being
              unmounted, so folding them away is a movement and the strip below
              does not jump. */}
          <div
            className="grid transition-[grid-template-rows,opacity] duration-300 ease-[cubic-bezier(.22,1,.36,1)] @max-[640px]/time:grid-rows-[0fr] @max-[640px]/time:opacity-0"
            style={{ gridTemplateRows: open ? "1fr" : "0fr" }}
          >
            <div className="min-h-0 overflow-hidden">
              {lanes.map((lane) => (
                <div
                  key={lane.id}
                  onClick={() => onPick(picked === lane.id ? null : lane.id)}
                  onMouseEnter={() => setHover(lane.id)}
                  onMouseLeave={() => setHover(null)}
                  title={`${lane.name} — ${lane.held.length}`}
                  className="group flex h-[17px] cursor-pointer items-center gap-2"
                >
                  <span
                    className={`w-[92px] shrink-0 truncate text-[9.5px] transition-colors ${
                      picked === lane.id ? "text-text" : "text-faint group-hover:text-soft"
                    }`}
                  >
                    {lane.name}
                  </span>
                  {/* A track, so the name has something to belong to. Without
                      it the labels floated beside empty space and the whole
                      row read as a rendering fault. */}
                  <span
                    className={`relative h-[9px] flex-1 rounded-full transition-colors ${
                      picked === lane.id || hover === lane.id ? "bg-raised" : "bg-raised/45"
                    }`}
                  >
                    {lane.held.map((m) => {
                      const d = new Date(m.started)
                      const inRange = !range || (key(d) >= range[0] && key(d) <= range[1])
                      const shown = lit === m.id
                      const dim =
                        (picked !== null && picked !== lane.id) ||
                        (hover !== null && hover !== lane.id)
                      return (
                        <span
                          key={m.id}
                          onMouseEnter={(e) => {
                            e.stopPropagation()
                            onLight(m.id)
                          }}
                          onMouseLeave={() => onLight(null)}
                          style={{
                            left: `${at(d)}%`,
                            width: `max(3px, ${(Math.min(m.duration, 5400) / 5400) * (100 / days) * 1.6}%)`,
                            background: lane.colour,
                            opacity: dim ? 0.12 : inRange ? 0.95 : 0.22,
                          }}
                          className={`absolute top-1/2 -translate-y-1/2 rounded-full transition-[opacity,height] duration-150 ${
                            shown ? "z-10 h-[13px] ring-1 ring-text" : "h-[7px]"
                          }`}
                        />
                      )
                    })}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* The window. Everything below the timeline reads it. */}
          <div
            ref={strip}
            onPointerDown={(e) => grab(e, "new")}
            onPointerMove={move}
            onPointerUp={() => setDrag(null)}
            className="relative mt-1 h-[15px] cursor-crosshair rounded-[4px] bg-raised/60"
          >
            {marks.map((m) => (
              <span
                key={m.id}
                style={{ left: `${at(new Date(m.started))}%` }}
                className="absolute top-1/2 h-[7px] w-px -translate-y-1/2 bg-faint/50"
              />
            ))}
            {range && (
              <>
                <span
                  style={{ left: 0, width: `${at(new Date(range[0]))}%` }}
                  className="pointer-events-none absolute inset-y-0 rounded-l-[4px] bg-ink/75"
                />
                <span
                  style={{ left: `${at(add(new Date(range[1]), 1))}%`, right: 0 }}
                  className="pointer-events-none absolute inset-y-0 rounded-r-[4px] bg-ink/75"
                />
                {(["a", "b"] as const).map((edge) => (
                  <span
                    key={edge}
                    onPointerDown={(e) => {
                      e.stopPropagation()
                      grab(e, edge)
                    }}
                    style={{
                      left: `${at(add(new Date(range[edge === "a" ? 0 : 1]), edge === "a" ? 0 : 1))}%`,
                    }}
                    className="absolute -top-[3px] h-[21px] w-2.5 -translate-x-1/2 cursor-ew-resize rounded-[3px] border border-line bg-raised transition-colors hover:border-accent"
                  />
                ))}
              </>
            )}
          </div>

          <div className="relative mt-0.5 h-3">
            {ticks.map((t) => (
              <span
                key={t.at}
                style={{ left: `${t.at}%` }}
                className="absolute text-[8.5px] tabular-nums text-faint/70"
              >
                {t.name}
              </span>
            ))}
            {/* What the window says, where the eye already is. */}
            {range && (
              <button
                onClick={() => onRange(null)}
                className="absolute right-0 -top-px rounded bg-raised px-1.5 py-px text-[9px] text-soft transition-colors hover:text-text"
              >
                {nice(range[0])} — {nice(range[1])} ✕
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
