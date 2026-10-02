import { useEffect, useMemo, useRef, useState } from "react"
import { AnimatePresence, motion } from "motion/react"
import {
  AudioLines,
  CornerDownLeft,
  FolderOpen,
  Search,
  Sparkles,
  UserRound,
} from "lucide-react"
import {
  Meetings as Api,
  many,
  when,
  type Group,
  type Person,
  type Recording,
} from "../api"
import { colourOf } from "../colours"
import { lang, t, type Key } from "../i18n"

type Hit = {
  kind: "meeting" | "project" | "person" | "do"
  id: number
  label: string
  note?: string
  colour?: string
  run: () => void
}

const ICON = {
  meeting: AudioLines,
  project: FolderOpen,
  person: UserRound,
  do: Sparkles,
}
const GROUP: Record<keyof typeof ICON, Key> = {
  meeting: "Наради",
  project: "Проєкти",
  person: "Люди",
  do: "Дії",
}

/** Command navigation lives in the title bar and opens with ⌘K. */
export default function Palette({
  onOpenMeeting,
  onOpenProject,
  onScreen,
}: {
  onOpenMeeting: (id: number) => void
  onOpenProject: (id: number) => void
  onScreen: (screen: "today" | "search" | "todo" | "ask" | "settings") => void
}) {
  const [query, setQuery] = useState("")
  const [at, setAt] = useState(0)
  const [rows, setRows] = useState<Recording[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [people, setPeople] = useState<Person[]>([])
  const field = useRef<HTMLInputElement>(null)

  const refresh = () => {
    void Api.Recent(10000)
      .then((r) => setRows((r as Recording[]) ?? []))
      .catch(() => {})
    void Api.Groups()
      .then((g) => setGroups((g as Group[]) ?? []))
      .catch(() => {})
    void Api.People()
      .then((p) => setPeople((p as Person[]) ?? []))
      .catch(() => {})
  }
  useEffect(refresh, [])

  useEffect(() => {
    const press = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        refresh()
        field.current?.focus()
        field.current?.select()
      }
    }
    window.addEventListener("keydown", press)
    return () => window.removeEventListener("keydown", press)
  }, [])

  const hits = useMemo<Hit[]>(() => {
    const q = query.trim().toLowerCase()
    if (!q) return []
    const has = (s: string) => s.toLowerCase().includes(q)

    const commands: Hit[] = [
      {
        kind: "do",
        id: -1,
        label: t("Почати або зупинити запис"),
        run: () => Api.Record(),
      },
      {
        kind: "do",
        id: -2,
        label: t("Шукати в усьому архіві"),
        run: () => onScreen("search"),
      },
      {
        kind: "do",
        id: -3,
        label: t("Запитати про весь архів"),
        run: () => onScreen("ask"),
      },
      {
        kind: "do",
        id: -4,
        label: t("Зобовʼязання"),
        run: () => onScreen("todo"),
      },
      {
        kind: "do",
        id: -5,
        label: t("Налаштування"),
        run: () => onScreen("settings"),
      },
      {
        kind: "do",
        id: -6,
        label: t("Сьогодні"),
        run: () => onScreen("today"),
      },
    ]

    return [
      ...commands.filter((c) => has(c.label)),
      ...groups
        .filter((g) => has(g.name))
        .map<Hit>((g) => ({
          kind: "project",
          id: g.id,
          label: g.name,
          note: `${g.count} ${many(g.count, "нарада", "наради", "нарад")}`,
          colour: colourOf(g.name, g.colour),
          run: () => onOpenProject(g.id),
        })),
      ...people
        .filter((p) => has(p.name))
        .map<Hit>((p) => ({
          kind: "person",
          id: p.id,
          label: p.name,
          note: `${p.meetings} ${many(p.meetings, "нарада", "наради", "нарад")}`,
          colour: colourOf(p.name, p.colour),
          run: () => onScreen("settings"),
        })),
      ...rows
        .filter((r) => has(r.title))
        .slice(0, 8)
        .map<Hit>((r) => ({
          kind: "meeting",
          id: r.id,
          label: r.title,
          note: when(r.started),
          run: () => onOpenMeeting(r.id),
        })),
    ]
  }, [query, rows, groups, people, onOpenMeeting, onOpenProject, onScreen, lang()])

  const go = (hit?: Hit) => {
    hit?.run()
    setQuery("")
    field.current?.blur()
  }

  return (
    <div className="no-drag relative w-[300px]">
      <div className="flex h-[26px] items-center gap-2 rounded-md border border-line/70 bg-surface/70 px-2.5 transition-colors focus-within:border-accent/60">
        <Search size={12} className="shrink-0 text-faint" />
        <input
          ref={field}
          onFocus={refresh}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setAt(0)
          }}
          onKeyDown={(e) => {
            e.stopPropagation()
            if (e.key === "ArrowDown")
              setAt((i) => Math.min(i + 1, hits.length - 1))
            if (e.key === "ArrowUp") setAt((i) => Math.max(i - 1, 0))
            if (e.key === "Enter") go(hits[at])
            if (e.key === "Escape") {
              setQuery("")
              e.currentTarget.blur()
            }
          }}
          placeholder={t("Нарада, проєкт, людина, дія…")}
          className="min-w-0 flex-1 bg-transparent text-[11.5px] outline-none placeholder:text-faint/80"
        />
        <kbd className="shrink-0 text-[9px] tabular-nums text-faint/70">⌘K</kbd>
      </div>

      <AnimatePresence>
        {hits.length > 0 && (
          <motion.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
            className="absolute inset-x-0 top-[calc(100%+6px)] z-50 max-h-[60vh] overflow-y-auto rounded-xl border border-line bg-raised/95 py-1 shadow-[0_20px_50px_-16px_rgba(0,0,0,0.8)] backdrop-blur-xl"
          >
            {hits.map((hit, i) => {
              const Icon = ICON[hit.kind]
              const first = i === 0 || hits[i - 1].kind !== hit.kind
              return (
                <div key={`${hit.kind}${hit.id}`}>
                  {first && (
                    <p className="px-3 pb-1 pt-2 text-[9px] uppercase tracking-[0.14em] text-faint">
                      {t(GROUP[hit.kind])}
                    </p>
                  )}
                  <button
                    onMouseEnter={() => setAt(i)}
                    onMouseDown={(e) => {
                      e.preventDefault()
                      go(hit)
                    }}
                    className={`flex w-full items-center gap-2.5 px-3 py-1.5 text-left transition-colors ${
                      at === i ? "bg-surface" : ""
                    }`}
                  >
                    <Icon
                      size={12}
                      style={{ color: hit.colour }}
                      className={hit.colour ? "" : "text-faint"}
                    />
                    <span className="min-w-0 flex-1 truncate text-[12px]">
                      {hit.label}
                    </span>
                    {hit.note && (
                      <span className="shrink-0 text-[10px] text-faint">
                        {hit.note}
                      </span>
                    )}
                    {at === i && (
                      <CornerDownLeft
                        size={11}
                        className="shrink-0 text-faint"
                      />
                    )}
                  </button>
                </div>
              )
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
