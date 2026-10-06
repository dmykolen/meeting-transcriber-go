import { useId, useMemo, useState } from "react"
import { motion, useReducedMotion } from "motion/react"
import {
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
  forceX,
  forceY,
  type SimulationLinkDatum,
  type SimulationNodeDatum,
} from "d3-force"
import { locale, t } from "../i18n"

type Topic = { topic: string; count: number; last: string }
type Link = { a: string; b: string; n: number }
type Orb = SimulationNodeDatum & {
  key: string
  topic: string
  count: number
  last: string
  r: number
  x: number
  y: number
}

const day = 86400000
const norm = (s: string) => s.trim().replace(/\s+/g, " ").toLowerCase()
/**
 * The colour of a topic is how lately a meeting had it, along a ramp the person
 * picks: the first colour is this fortnight, the last is what the project has not
 * touched for three months. Interpolated in oklch so every step is even.
 */
type Stop = { at: number; l: number; c: number; h: number }
/** Each palette is a ramp from "this fortnight" to "not touched for three months". */
const PALETTES: Record<string, { name: string; ramp: Stop[] }> = {
  ocean: {
    name: "Океан",
    ramp: [
      { at: 0, l: 0.8, c: 0.12, h: 190 },
      { at: 0.35, l: 0.68, c: 0.13, h: 230 },
      { at: 0.7, l: 0.56, c: 0.14, h: 262 },
      { at: 1, l: 0.44, c: 0.1, h: 278 },
    ],
  },
  warm: {
    name: "Тепла",
    ramp: [
      { at: 0, l: 0.77, c: 0.13, h: 58 },
      { at: 0.35, l: 0.67, c: 0.18, h: 12 },
      { at: 0.7, l: 0.56, c: 0.17, h: -40 },
      { at: 1, l: 0.5, c: 0.12, h: 262 },
    ],
  },
  mono: {
    name: "Графіт",
    ramp: [
      { at: 0, l: 0.9, c: 0.01, h: 260 },
      { at: 0.35, l: 0.76, c: 0.012, h: 260 },
      { at: 0.7, l: 0.6, c: 0.014, h: 260 },
      { at: 1, l: 0.46, c: 0.016, h: 260 },
    ],
  },
  moss: {
    name: "Мох",
    ramp: [
      { at: 0, l: 0.82, c: 0.16, h: 135 },
      { at: 0.35, l: 0.7, c: 0.13, h: 160 },
      { at: 0.7, l: 0.58, c: 0.09, h: 200 },
      { at: 1, l: 0.46, c: 0.06, h: 235 },
    ],
  },
}
function heat(ramp: Stop[], daysAgo: number) {
  const t = Math.min(Math.max(daysAgo / 90, 0), 1)
  const k = Math.max(1, ramp.findIndex((s) => s.at >= t))
  const a = ramp[k - 1],
    b = ramp[k]
  const f = (t - a.at) / (b.at - a.at || 1)
  const mix = (x: number, y: number) => x + (y - x) * f
  return { l: mix(a.l, b.l), c: mix(a.c, b.c), h: mix(a.h, b.h) }
}
const ok = (c: { l: number; c: number; h: number }, dl = 0, dc = 1, alpha = 1) =>
  `oklch(${((c.l + dl) * 100).toFixed(1)}% ${(c.c * dc).toFixed(3)} ${c.h.toFixed(1)}${alpha < 1 ? ` / ${alpha}` : ""})`

const lightLabel = {
  fill: "oklch(18% 0.02 265)",
  stroke: "oklch(98% 0 0 / 0.35)",
} as const

/**
 * Lay the orbs out once. Size says how many meetings had a topic; a link pulls
 * two topics together when the same meetings had both, so what a project keeps
 * circling gathers in one place and the rest drifts to the edge. The layout is
 * settled before anything is drawn and is the same every time for the same
 * data: nothing here moves by itself except a slow breath.
 */
function layout(topics: Topic[], links: Link[]) {
  const top = Math.max(...topics.map((x) => x.count), 1)
  const orbs: Orb[] = topics.map((x) => ({
    key: norm(x.topic),
    topic: x.topic,
    count: x.count,
    last: x.last,
    r: 22 + (top > 1 ? Math.sqrt((x.count - 1) / (top - 1)) : 0.35) * 44,
    x: 0,
    y: 0,
  }))
  const at = new Map(orbs.map((o) => [o.key, o]))
  const edges: (SimulationLinkDatum<Orb> & { n: number })[] = links.flatMap((l) => {
    const a = at.get(norm(l.a)),
      b = at.get(norm(l.b))
    return a && b ? [{ source: a, target: b, n: l.n }] : []
  })
  const strongest = Math.max(...edges.map((e) => e.n), 1)
  const sim = forceSimulation(orbs)
    .force("collide", forceCollide<Orb>((o) => o.r + 8).strength(1))
    .force("charge", forceManyBody<Orb>().strength(-90))
    .force(
      "link",
      forceLink<Orb, (typeof edges)[number]>(edges)
        .distance((e) => (e.source as Orb).r + (e.target as Orb).r + 46 + (strongest - e.n) * 22)
        .strength((e) => 0.12 + (0.5 * e.n) / strongest),
    )
    // Wider than tall: the section is a band, not a square.
    .force("x", forceX<Orb>(0).strength(0.05))
    .force("y", forceY<Orb>(0).strength(0.16))
    .stop()
  for (let i = 0; i < 420; i++) sim.tick()
  const left = Math.min(...orbs.map((o) => o.x - o.r)),
    right = Math.max(...orbs.map((o) => o.x + o.r)),
    up = Math.min(...orbs.map((o) => o.y - o.r)),
    down = Math.max(...orbs.map((o) => o.y + o.r))
  const pad = 18
  return {
    orbs,
    edges: edges as { source: Orb; target: Orb; n: number }[],
    strongest,
    box: [left - pad, up - pad, right - left + pad * 2, down - up + pad * 2] as const,
  }
}

/** One or two lines of the label, and the size that fits an orb of radius r. */
function fit(text: string, r: number) {
  const room = r * 1.7
  const wide = (s: string, size: number) => s.length * size * 0.56
  for (const size of [r * 0.42, r * 0.34, r * 0.28]) {
    const s = Math.max(10, Math.min(size, 22))
    if (wide(text, s) <= room) return { lines: [text], size: s }
    const words = text.split(" ")
    if (words.length > 1) {
      let best: string[] | null = null
      for (let i = 1; i < words.length; i++) {
        const pair = [words.slice(0, i).join(" "), words.slice(i).join(" ")]
        if (Math.max(...pair.map((p) => wide(p, s))) <= room && (!best || Math.abs(pair[0].length - pair[1].length) < Math.abs(best[0].length - best[1].length)))
          best = pair
      }
      if (best) return { lines: best, size: s }
    }
  }
  return null // too long for the orb: the label goes beneath it
}

/**
 * What a project's meetings were about, as a constellation. Each topic is an
 * orb as large as the number of meetings that had it and as bright as it is
 * recent. Two topics are joined when the same meetings had both. Pointing at
 * one lights what it is tied to and dims the rest; pressing it narrows the
 * meetings list to that topic.
 */
export default function TopicCloud({
  topics,
  links,
  picked,
  onPick,
}: {
  topics: Topic[]
  links: Link[]
  picked: string | null
  onPick: (topic: string) => void
}) {
  const still = useReducedMotion()
  const id = useId().replace(/:/g, "")
  const [hot, setHot] = useState<string | null>(null)
  const [palette, setPalette] = useState(() => {
    try {
      const saved = localStorage.getItem("mt.topics.palette")
      return saved && saved in PALETTES ? saved : "ocean"
    } catch {
      return "ocean"
    }
  })
  const ramp = PALETTES[palette].ramp
  const shown = useMemo(() => topics.slice(0, 28), [topics])
  const sig = JSON.stringify([shown, links])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const map = useMemo(() => (shown.length ? layout(shown, links) : null), [sig])
  if (!map) return null

  const now = Date.now()
  const active = hot ?? (picked ? norm(picked) : null)
  const tied = new Set<string>()
  if (active)
    for (const e of map.edges) {
      if (e.source.key === active) tied.add(e.target.key)
      if (e.target.key === active) tied.add(e.source.key)
    }

  return (
    <figure className="topic-map">
      <div className="topic-palettes" role="radiogroup" aria-label={t("Кольори тем")}>
        {Object.entries(PALETTES).map(([key, p]) => (
          <button
            key={key}
            role="radio"
            aria-checked={key === palette}
            title={t(p.name as never)}
            aria-label={t(p.name as never)}
            onClick={() => {
              setPalette(key)
              try {
                localStorage.setItem("mt.topics.palette", key)
              } catch {
                // The choice then lasts until the next launch.
              }
            }}
            style={{
              background: `linear-gradient(135deg, ${ok(p.ramp[0])}, ${ok(p.ramp[1])} 45%, ${ok(p.ramp[3])})`,
            }}
          />
        ))}
      </div>
      <svg
        viewBox={map.box.join(" ")}
        role="group"
        aria-label={t("Теми")}
        style={{ width: "100%", height: "auto", maxHeight: 430, display: "block" }}
      >
        <defs>
          {map.orbs.map((o, i) => {
            const c = heat(ramp, (now - new Date(o.last).getTime()) / day)
            return (
              <radialGradient key={o.key} id={`${id}-o${i}`} cx="34%" cy="26%" r="90%">
                <stop offset="0%" stopColor={ok(c, 0.1, 0.85)} />
                <stop offset="50%" stopColor={ok(c)} />
                <stop offset="100%" stopColor={ok(c, -0.24, 0.9)} />
              </radialGradient>
            )
          })}
          {map.edges.map((e, i) => (
            <linearGradient key={i} id={`${id}-e${i}`} gradientUnits="userSpaceOnUse" x1={e.source.x} y1={e.source.y} x2={e.target.x} y2={e.target.y}>
              <stop offset="0%" stopColor={ok(heat(ramp, (now - new Date(e.source.last).getTime()) / day), 0.05)} />
              <stop offset="100%" stopColor={ok(heat(ramp, (now - new Date(e.target.last).getTime()) / day), 0.05)} />
            </linearGradient>
          ))}
        </defs>

        <g aria-hidden>
          {map.edges.map((e, i) => {
            const on = active && (e.source.key === active || e.target.key === active)
            const mx = (e.source.x + e.target.x) / 2,
              my = (e.source.y + e.target.y) / 2
            // A slight bow, so a web of straight lines does not read as a graph paper.
            const bow = ((i % 2 ? 1 : -1) * Math.hypot(e.target.x - e.source.x, e.target.y - e.source.y)) / 14
            const nx = -(e.target.y - e.source.y),
              ny = e.target.x - e.source.x,
              len = Math.hypot(nx, ny) || 1
            return (
              <path
                key={i}
                d={`M${e.source.x} ${e.source.y} Q${mx + (nx / len) * bow} ${my + (ny / len) * bow} ${e.target.x} ${e.target.y}`}
                fill="none"
                stroke={`url(#${id}-e${i})`}
                strokeLinecap="round"
                strokeWidth={1 + (2.6 * e.n) / map.strongest}
                style={{
                  opacity: active ? (on ? 0.9 : 0.05) : 0.22 + (0.4 * e.n) / map.strongest,
                  transition: "opacity 0.25s",
                }}
              />
            )
          })}
        </g>

        {map.orbs.map((o, i) => {
          const age = (now - new Date(o.last).getTime()) / day
          const dim = active !== null && o.key !== active && !tied.has(o.key)
          const chosen = picked !== null && norm(picked) === o.key
          const text = fit(o.topic, o.r)
          const shade = heat(ramp, age)
          // A pale orb takes dark type; a deep one takes white.
          const light = shade.l > 0.72
          const top = Math.max(...map.orbs.map((x) => x.count))
          const neighbours = map.edges
            .filter((e) => e.source.key === o.key || e.target.key === o.key)
            .map((e) => (e.source.key === o.key ? e.target.topic : e.source.topic))
          return (
            <g key={o.key} transform={`translate(${o.x} ${o.y})`}>
              <motion.g
                initial={still ? false : { scale: 0.4, opacity: 0 }}
                animate={{ scale: o.key === hot ? 1.07 : 1, opacity: 1 }}
                transition={{
                  scale: { type: "spring", stiffness: 420, damping: 26 },
                  opacity: { duration: 0.35, delay: Math.min(i * 0.025, 0.5) },
                }}
              >
                <g
                  className="orb"
                  role="button"
                  tabIndex={0}
                  aria-pressed={chosen}
                  aria-label={`${o.topic}, ${t("{n} нарад", { n: o.count })}`}
                  style={{
                    ["--float" as string]: `${5 + (i % 5)}s`,
                    ["--lag" as string]: `${-(i * 0.9)}s`,
                    opacity: dim ? 0.2 : 1,
                    transition: "opacity 0.25s",
                    cursor: "pointer",
                  }}
                  onPointerEnter={() => setHot(o.key)}
                  onPointerLeave={() => setHot(null)}
                  onFocus={() => setHot(o.key)}
                  onBlur={() => setHot(null)}
                  onClick={() => onPick(o.topic)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault()
                      onPick(o.topic)
                    }
                  }}
                >
                  <title>
                    {`${o.topic} · ${t("{n} нарад", { n: o.count })} · ${t("востаннє")} ${new Date(o.last).toLocaleDateString(locale(), { day: "numeric", month: "short" })}${neighbours.length ? `\n${t("разом із")}: ${neighbours.join(", ")}` : ""}`}
                  </title>
                  <circle r={o.r * 1.9} fill={ok(shade, 0, 1, 0.5)} style={{ filter: `blur(${o.r * 0.5}px)`, opacity: o.key === active || chosen ? 0.6 : 0.1 + 0.16 * (o.count / top) }} />
                  <circle
                    r={o.r}
                    fill={`url(#${id}-o${i})`}
                    stroke={chosen ? "white" : "white"}
                    strokeOpacity={chosen ? 0.95 : 0.16}
                    strokeWidth={chosen ? 2.4 : 1}
                  />
                  <ellipse cx={-o.r * 0.28} cy={-o.r * 0.42} rx={o.r * 0.5} ry={o.r * 0.26} fill="white" opacity={0.14} />
                  {text ? (
                    <text
                      textAnchor="middle"
                      className="orb-label"
                      style={{ fontSize: text.size, ...(light ? lightLabel : {}) }}
                      y={(-(text.lines.length - 1) * text.size * 0.6) / 2 + text.size * 0.33}
                    >
                      {text.lines.map((l, k) => (
                        <tspan key={k} x={0} dy={k ? text.size * 1.12 : 0}>
                          {l}
                        </tspan>
                      ))}
                    </text>
                  ) : (
                    <text textAnchor="middle" className="orb-label orb-out" y={o.r + 15} style={{ fontSize: 12.5 }}>
                      {o.topic}
                    </text>
                  )}
                  {o.count > 1 && (
                    <g transform={`translate(${o.r * 0.72} ${-o.r * 0.72})`}>
                      <circle r={9} fill="var(--color-ink)" stroke="white" strokeOpacity={0.25} />
                      <text textAnchor="middle" y={3.6} className="orb-count">
                        {o.count}
                      </text>
                    </g>
                  )}
                </g>
              </motion.g>
            </g>
          )
        })}
      </svg>
      <figcaption>
        {t("Розмір — скільки нарад мали тему. Колір — як давно: від теплого, свіжого, до холодного. Лінія — теми з одних нарад.")}
      </figcaption>
    </figure>
  )
}
