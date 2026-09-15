import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react"
import {
  motion,
  useMotionValue,
  useReducedMotion,
  useSpring,
  useTransform,
} from "motion/react"
import { FolderPlus, Layers, Grid2X2 } from "lucide-react"
import type { Group } from "../api"
import { colourOf, on as legible } from "../colours"

/**
 * The projects, as a dock.
 *
 * Three earlier attempts at this control were rejected — a coloured dot beside
 * a tab, a proportional strip, a row of chips — and all three failed the same
 * way: they were labels wearing a colour. A dock is an object. It has a place
 * of its own at the bottom of the window, it is always there, and a project is
 * a tile you press rather than a word you read.
 *
 * The motion is the point, not decoration. One name rides above the row and
 * *travels* to whatever the cursor is nearest, stretching to fit the new name
 * as it goes. That is what makes the row read as one instrument rather than as
 * six buttons that each happen to have a tooltip: the label is a single object
 * being handed from tile to tile.
 *
 * Everything moves on a critically damped spring — stiffness 420, damping 41,
 * so it arrives without a wobble and without a bounce to sit through. Hover is
 * deliberately not instant: 55 ms of intent before the first name appears, 40 ms
 * between tiles, 150 ms before it leaves, and a 4 px dead band at the midpoint
 * between two tiles so that trackpad noise cannot make the label flicker.
 *
 * It is also where a meeting is filed: drag a card onto a tile. While a card is
 * in the air the whole dock rises and the name says what dropping would do —
 * the feedback that was missing when dragging worked but looked like it hadn't.
 */
export default function Dock({
  groups,
  picked,
  over,
  dragging,
  onPick,
  onNew,
}: {
  groups: Group[]
  picked: number | null
  /** The tile a dragged meeting is currently over. */
  over: number | null
  /** Whether a meeting is in the air at all. */
  dragging?: boolean
  onPick: (id: number | null) => void
  onNew: () => void
}) {
  const [query, setQuery] = useState("")
  const dock = useRef<HTMLDivElement>(null)
  const tiles = useRef<(HTMLButtonElement | null)[]>([])
  const label = useRef<HTMLSpanElement>(null)
  const still = useReducedMotion()

  // -1 is "nothing", which is also the resting state.
  const [at, setAt] = useState(-1)
  const [pressed, setPressed] = useState(-1)
  const intent = useRef(0)
  const leaving = useRef(0)
  const centres = useRef<number[]>([])

  const items: {
    key: string
    id: number | null
    name: string
    note: string
    colour: string
  }[] = [
    {
      key: "all",
      id: null,
      name: "Усі записи",
      note: "",
      colour: "var(--color-raised)",
    },
    ...[...groups]
      .sort((a, b) => b.count - a.count)
      .filter((g, i) => i < 6 || g.id === picked)
      .map((g) => ({
        key: `g${g.id}`,
        id: g.id,
        name: g.name,
        note: `${g.count}`,
        colour: colourOf(g.name, g.colour),
      })),
  ]

  const spring = { stiffness: 420, damping: 41, mass: 1, restDelta: 0.001 }
  const x = useSpring(useMotionValue(0), spring)
  const width = useSpring(useMotionValue(120), spring)
  const shown = useSpring(useMotionValue(0), spring)
  // The name lifts the last few pixels into place as it appears.
  const rise = useTransform(shown, [0, 1], [10, 0])

  const measure = useCallback(() => {
    const box = dock.current?.getBoundingClientRect()
    if (!box) return
    centres.current = tiles.current.map((t) => {
      const r = t?.getBoundingClientRect()
      return r ? r.left + r.width / 2 - box.left : 0
    })
  }, [])

  // The name that is showing wins over the one being hovered: a drag says what
  // dropping would do, and that matters more than where the cursor is.
  const target = over !== null ? items.findIndex((i) => i.id === over) : at
  const item = items[target]

  useLayoutEffect(() => {
    measure()
    if (target < 0 || !label.current) {
      shown.set(0)
      return
    }
    const box = dock.current?.getBoundingClientRect()
    const wide = label.current.scrollWidth
    const half = wide / 2
    // Clamped to the window: a project at the end of a long dock must not push
    // its own name off the screen.
    const centre = Math.min(
      Math.max(centres.current[target] ?? 0, half + 12 - (box?.left ?? 0)),
      window.innerWidth - 12 - (box?.left ?? 0) - half,
    )
    const first = shown.get() < 0.01
    width[first ? "jump" : "set"](wide)
    x[first ? "jump" : "set"](centre)
    shown.set(1)
  }, [target, item?.name, item?.note, measure, shown, width, x])

  useEffect(() => {
    const again = () => measure()
    window.addEventListener("resize", again)
    return () => window.removeEventListener("resize", again)
  }, [measure])

  /** Which tile the cursor is nearest, with a dead band at the boundary. */
  const under = (clientX: number) => {
    const box = dock.current?.getBoundingClientRect()
    if (!box) return -1
    const local = clientX - box.left
    let best = 0
    centres.current.forEach((c, i) => {
      if (Math.abs(local - c) < Math.abs(local - centres.current[best]))
        best = i
    })
    if (at >= 0 && best !== at) {
      const mid = (centres.current[at] + centres.current[best]) / 2
      if (Math.abs(local - mid) < 4) return at
    }
    return best
  }

  const want = (next: number) => {
    window.clearTimeout(leaving.current)
    if (next === at) return
    window.clearTimeout(intent.current)
    intent.current = window.setTimeout(() => setAt(next), at < 0 ? 55 : 40)
  }
  const gone = () => {
    window.clearTimeout(intent.current)
    leaving.current = window.setTimeout(() => setAt(-1), 150)
  }

  const keys = (e: React.KeyboardEvent, i: number) => {
    const to =
      e.key === "ArrowRight"
        ? (i + 1) % items.length
        : e.key === "ArrowLeft"
          ? (i + items.length - 1) % items.length
          : e.key === "Home"
            ? 0
            : e.key === "End"
              ? items.length - 1
              : -1
    if (e.key === "Escape") return setAt(-1)
    if (to < 0) return
    e.preventDefault()
    tiles.current[to]?.focus()
    setAt(to)
  }

  return (
    <div className="project-dock no-drag pointer-events-none absolute inset-x-0 bottom-2 z-30 flex justify-center">
      <motion.div
        ref={dock}
        onPointerMove={(e) => {
          if (e.pointerType === "touch") return
          measure()
          want(under(e.clientX))
        }}
        onPointerLeave={gone}
        onPointerUp={() => setPressed(-1)}
        animate={{ y: dragging ? -8 : 0, scale: dragging ? 1.04 : 1 }}
        transition={still ? { duration: 0 } : { type: "spring", ...spring }}
        className={`pointer-events-auto relative flex h-[53px] max-w-[calc(100%-1.5rem)] items-center gap-1.5 rounded-[15px] border bg-ink/90 px-2 backdrop-blur-xl transition-[border-color,box-shadow] duration-200 ${
          dragging
            ? "border-accent/60 shadow-[0_10px_34px_rgba(0,0,0,0.5)]"
            : "border-line/80 shadow-[0_5px_20px_rgba(0,0,0,0.35)]"
        }`}
      >
        {items.map((it, i) => (
          <Tile
            key={it.key}
            hold={(el) => (tiles.current[i] = el)}
            project={it.id}
            colour={it.colour}
            on={picked === it.id}
            // Only while something is actually in the air, and never on the
            // "all" tile: its id is null, and so is `over` when nothing is
            // being dragged, so `over === it.id` was true from the moment the
            // app opened and left that tile permanently ringed.
            lit={!!dragging && it.id !== null && over === it.id}
            hot={target === i}
            down={pressed === i}
            still={!!still}
            label={it.name}
            onDown={() => setPressed(i)}
            onFocus={(e) =>
              e.currentTarget.matches(":focus-visible") && setAt(i)
            }
            onBlur={gone}
            onKeyDown={(e) => keys(e, i)}
            onClick={() =>
              onPick(picked === it.id && it.id !== null ? null : it.id)
            }
            divider={i === 1}
          >
            {it.id === null ? (
              <Layers size={17} className="text-soft" />
            ) : (
              // Half the wheel is dark now, so a hard-coded dark initial is
              // unreadable on five of the ten tiles. Safari works out which of
              // black or white wins on this exact colour.
              <span
                style={{ color: legible(it.colour) }}
                className="text-[15px] font-semibold tracking-tight"
              >
                {it.name.trim()[0]?.toUpperCase()}
              </span>
            )}
          </Tile>
        ))}

        <button
          className="dock-more ui-icon"
          aria-label="Усі проєкти"
          popoverTarget="all-projects"
        >
          <Grid2X2 size={17} />
        </button>
        <div id="all-projects" popover="auto" className="all-projects">
          <header>
            <strong>Проєкти · {groups.length}</strong>
          </header>
          <input
            aria-label="Знайти проєкт"
            placeholder="Знайти проєкт…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <div>
            {groups
              .filter((g) => g.name.toLowerCase().includes(query.toLowerCase()))
              .map((g) => (
                <button
                  key={g.id}
                  onClick={() => {
                    onPick(g.id)
                    document.getElementById("all-projects")?.hidePopover()
                  }}
                >
                  <i style={{ background: colourOf(g.name, g.colour) }} />
                  {g.name}
                  <small>{g.count}</small>
                </button>
              ))}
          </div>
        </div>
        <span className="mx-0.5 h-6 w-px shrink-0 bg-line" />

        <Tile
          hold={() => {}}
          project={null}
          colour="transparent"
          on={false}
          lit={false}
          hot={false}
          down={false}
          still={!!still}
          label="Новий проєкт"
          onDown={() => {}}
          onClick={onNew}
        >
          <FolderPlus size={16} className="text-faint" />
        </Tile>

        {/* One name for the whole row. It slides to the tile under the cursor
            and stretches to the new name rather than fading out and back in,
            which is what makes the dock feel like an instrument. */}
        <motion.div
          aria-hidden
          style={{ x, width, opacity: shown, y: rise }}
          className="pointer-events-none absolute bottom-[calc(100%+9px)] left-0 h-[34px] -translate-x-1/2 overflow-hidden rounded-[11px] border border-line bg-raised/95 shadow-[0_8px_18px_rgba(0,0,0,0.4)] backdrop-blur"
        >
          <span
            style={{ background: item?.colour ?? "transparent" }}
            className="absolute inset-x-0 bottom-0 h-[2.5px] transition-colors duration-200"
          />
        </motion.div>
        <motion.span
          ref={label}
          style={{ x, opacity: shown, y: rise }}
          className="pointer-events-none absolute bottom-[calc(100%+9px)] left-0 flex h-[34px] -translate-x-1/2 items-center gap-2 whitespace-nowrap px-3.5 text-[11.5px]"
        >
          {over !== null && <span className="text-faint">Перенести в</span>}
          <span className="font-medium">{item?.name ?? ""}</span>
          {item?.note && over === null && (
            <span className="text-faint tabular-nums">{item.note}</span>
          )}
        </motion.span>
      </motion.div>
    </div>
  )
}

function Tile({
  hold,
  project,
  colour,
  on,
  lit,
  hot,
  down,
  still,
  label,
  divider,
  children,
  onDown,
  onClick,
  onFocus,
  onBlur,
  onKeyDown,
}: {
  hold: (el: HTMLButtonElement | null) => void
  /** What a meeting dropped here is filed under. null is the "all" tile. */
  project: number | null
  colour: string
  on: boolean
  /** A meeting is being dragged over this one. */
  lit: boolean
  /** The cursor is nearest this one. */
  hot: boolean
  down: boolean
  still: boolean
  label: string
  divider?: boolean
  children: React.ReactNode
  onDown: () => void
  onClick: () => void
  onFocus?: (e: React.FocusEvent<HTMLButtonElement>) => void
  onBlur?: () => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLButtonElement>) => void
}) {
  return (
    <>
      {divider && <span className="mx-0.5 h-6 w-px shrink-0 bg-line" />}
      <button
        ref={hold}
        data-project={project ?? undefined}
        onPointerDown={onDown}
        onClick={onClick}
        onFocus={onFocus}
        onBlur={onBlur}
        onKeyDown={onKeyDown}
        aria-label={label}
        // A ring only for the keyboard. Clicking a tile left the browser's own
        // focus ring drawn around it, so the "all" tile sat there looking
        // permanently pointed at from the moment anybody pressed it.
        className="relative size-[38px] shrink-0 rounded-[9px] outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[3px] focus-visible:outline-accent"
      >
        <motion.span
          animate={{
            y: (hot || lit ? -6 : 0) + (down ? 2 : 0),
            scale: 1 + (lit ? 0.14 : hot ? 0.085 : 0) - (down ? 0.025 : 0),
          }}
          transition={
            still
              ? { duration: 0 }
              : { type: "spring", stiffness: 420, damping: 41 }
          }
          style={{ background: colour, transformOrigin: "center bottom" }}
          className={`absolute inset-0 grid place-items-center overflow-hidden rounded-[9px] border border-text/10 shadow-[0_2px_3px_rgba(0,0,0,0.15)] ${
            hot || lit
              ? "brightness-110 shadow-[0_7px_12px_rgba(0,0,0,0.3)]"
              : ""
          } ${lit ? "ring-2 ring-text ring-offset-2 ring-offset-ink" : ""}`}
        >
          {children}
        </motion.span>
        <motion.i
          animate={{ opacity: on ? 1 : 0 }}
          transition={{ duration: 0.2 }}
          style={{ background: colour }}
          className="absolute -bottom-[5px] left-1/2 h-[3px] w-1 -translate-x-1/2 rounded-full"
        />
      </button>
    </>
  )
}
