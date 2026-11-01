import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import {
  Check,
  Copy,
  FileText,
  MoreHorizontal,
  Play,
  RotateCcw,
  ScrollText,
  Search as SearchIcon,
  Sparkles,
  Trash2,
  X,
} from "lucide-react"
import {
  Meetings,
  clock,
  copyText,
  due,
  late,
  length,
  when,
  why,
  type Group,
  type Listening,
  type Summary,
  type Turn,
  type VoiceNote,
} from "../api"
import { colourOf } from "../colours"
import { draft, drop, useDrafts, useNow, working, type Draft } from "../drafts"
import Head from "../components/Head"
import Filing from "../components/Filing"
import LiveNow from "../components/LiveNow"
import Player, { type Controls } from "../components/Player"
import Undo from "../components/Undo"
import { locale, t, tr } from "../i18n"

/**
 * What was said to nobody but the microphone, written out as its speaker would
 * have typed it. A note is a card, not a meeting's page: its text, its to-dos
 * with their dates, the voice behind it, and the project it is filed under.
 * Newest first, as many columns as the window holds: grouped by day, a wide
 * window held two cards a row and showed the rest of itself empty.
 */
export default function Notes({
  pick,
  onPicked,
  onOpen,
}: {
  pick: number | null
  onPicked: () => void
  /** The recording's full page in Library: chapters, decisions, its notes. */
  onOpen: (id: number) => void
}) {
  const [notes, setNotes] = useState<VoiceNote[] | null>(null)
  const [groups, setGroups] = useState<Group[]>([])
  const [live, setLive] = useState<Listening | null>(null)
  const [ai, setAi] = useState(false)
  const [query, setQuery] = useState("")
  const [problem, setProblem] = useState("")
  const [lit, setLit] = useState<number | null>(null)
  const [playing, setPlaying] = useState<{ id: number; file: string; at: number } | null>(null)
  const [binned, setBinned] = useState<VoiceNote | null>(null)
  const [replaced, setReplaced] = useState<{ id: number; before: Summary; after: Summary } | null>(null)
  const drafts = useDrafts()
  const player = useRef<Controls>(null)
  const known = useRef<Set<number> | null>(null)

  const load = useCallback(async () => {
    try {
      setNotes((await Meetings.VoiceNotes()) as VoiceNote[])
    } catch (e) {
      setProblem(why(e))
    }
    Meetings.Groups()
      .then((g) => setGroups((g as Group[]) ?? []))
      .catch(() => {})
  }, [])
  useEffect(() => {
    load()
    Meetings.Summaries()
      .then(setAi)
      .catch(() => {})
    const timer = setInterval(load, 3000)
    return () => clearInterval(timer)
  }, [load])
  useEffect(() => {
    const read = () =>
      Meetings.Listening()
        .then((s) => setLive(s as Listening))
        .catch(() => {})
    read()
    const timer = setInterval(read, 1000)
    return () => clearInterval(timer)
  }, [])

  // A note that arrives while the screen is open, the one just dictated, is
  // lit and brought into view, as is one opened from elsewhere.
  useEffect(() => {
    if (!notes) return
    const fresh = known.current && notes.find((n) => !known.current!.has(n.id))
    known.current = new Set(notes.map((n) => n.id))
    if (fresh) setLit(fresh.id)
  }, [notes])
  useEffect(() => {
    if (pick === null) return
    setLit(pick)
    onPicked()
  }, [pick, onPicked])
  useEffect(() => {
    if (lit === null) return
    const timer = setTimeout(() => setLit(null), 2600)
    return () => clearTimeout(timer)
  }, [lit])
  const ready = notes !== null
  useEffect(() => {
    if (lit !== null && ready)
      document.querySelector(`[data-note="${lit}"]`)?.scrollIntoView({ block: "nearest", behavior: "smooth" })
  }, [lit, ready])
  useEffect(() => {
    if (playing) player.current?.go(playing.at)
  }, [playing])
  useEffect(() => {
    if (!replaced) return
    const timer = setTimeout(() => setReplaced(null), 10_000)
    return () => clearTimeout(timer)
  }, [replaced])

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (notes ?? []).filter(
      (n) =>
        !q ||
        [n.title, n.summary?.overview, ...(n.summary?.action_items ?? []).map((a) => a.task), n.said]
          .join("\n")
          .toLowerCase()
          .includes(q),
    )
  }, [notes, query])
  const missed = !!live?.missed && Date.now() - Date.parse(live.missed) < 5000

  const accept = async (n: VoiceNote, d: Draft) => {
    try {
      const before = n.summary ?? null
      await Meetings.AcceptSummary(n.id, before as never, d.summary as never)
      drop(n.id)
      await load()
      if (before) setReplaced({ id: n.id, before, after: d.summary! })
    } catch (e) {
      setProblem(why(e))
    }
  }

  return (
    <div className="memos">
      <Head title={t("Нотатки")} count={notes ? shown.length : undefined}>
        <label className="memos-find">
          <SearchIcon size={12} aria-hidden />
          <input
            type="search"
            aria-label={t("Шукати в нотатках")}
            placeholder={t("Шукати в нотатках")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Escape" && setQuery("")}
          />
        </label>
      </Head>
      <div className="memos-feed">
        {live?.dictating && <LiveNow state={live} />}
        {missed && (
          <p role="status" className="memos-missed">
            {t("Нічого не почув: нотатку не збережено")}
          </p>
        )}
        {problem && (
          <p role="alert" className="reader-error">
            {problem}
            <button onClick={() => setProblem("")} aria-label={t("Закрити повідомлення")}>
              <X size={13} />
            </button>
          </p>
        )}
        {notes && !notes.length && !live?.dictating && (
          <p className="memos-empty">
            {t("Нотаток ще немає. Натисніть «Нотатка» внизу ліворуч, скажіть, що не хочете забути, і натисніть ще раз.")}
          </p>
        )}
        {notes && !!notes.length && !shown.length && <p className="memos-empty">{t("Нічого не знайшов.")}</p>}
        <div className="memos-grid">
          {shown.map((n) => (
            <Card
              key={n.id}
              note={n}
              groups={groups}
              lit={lit === n.id}
              ai={ai}
              draft={drafts.find((d) => d.id === n.id)}
              onPlay={(at) => n.audio && setPlaying({ id: n.id, file: n.audio, at })}
              onOpen={() => onOpen(n.id)}
              onChanged={load}
              onProblem={setProblem}
              onAccept={(d) => accept(n, d)}
              onDelete={async () => {
                try {
                  await Meetings.Delete(n.id)
                  setBinned(n)
                  if (playing?.id === n.id) setPlaying(null)
                  await load()
                } catch (e) {
                  setProblem(why(e))
                }
              }}
            />
          ))}
        </div>
      </div>
      {playing && <Player key={playing.id} id={playing.id} file={playing.file} onTime={() => {}} ref={player} />}
      <Undo
        what={binned ? title(binned) : null}
        onUndo={async () => {
          if (binned) await Meetings.Restore(binned.id)
          setBinned(null)
          load()
        }}
        onGone={() => setBinned(null)}
      />
      {replaced && (
        <div className="action-toast" role="status">
          {t("Текст замінено")}
          <button
            onClick={async () => {
              try {
                await Meetings.AcceptSummary(replaced.id, replaced.after as never, replaced.before as never)
                setReplaced(null)
                await load()
              } catch (e) {
                setProblem(why(e))
              }
            }}
          >
            <RotateCcw size={12} />
            {t("Скасувати||undo")}
          </button>
          <button onClick={() => setReplaced(null)} aria-label={t("Закрити повідомлення")}>
            <X size={12} />
          </button>
        </div>
      )}
    </div>
  )
}

/** One note. */
function Card({
  note: n,
  groups,
  lit,
  ai,
  draft: d,
  onPlay,
  onOpen,
  onChanged,
  onProblem,
  onAccept,
  onDelete,
}: {
  note: VoiceNote
  groups: Group[]
  lit: boolean
  ai: boolean
  draft?: Draft
  onPlay: (at: number) => void
  onOpen: () => void
  onChanged: () => void
  onProblem: (why: string) => void
  onAccept: (d: Draft) => void
  onDelete: () => void
}) {
  const [said, setSaid] = useState<Turn[] | null>(null)
  const now = useNow(working(d))
  const busy = n.status !== "done" && n.status !== "failed"
  const proposal = d?.summary
  const s = proposal ?? n.summary
  const filed = groups.find((g) => g.id === n.group)
  const menu = `note-menu-${n.id}`
  const shut = (e: React.MouseEvent) =>
    (e.currentTarget.closest("[popover]") as HTMLElement | null)?.hidePopover()
  const write = () => draft(n.id, title(n), !n.summary, true)
  const act = (f: () => Promise<unknown>) => f().then(onChanged, (e) => onProblem(why(e)))

  return (
    <article data-note={n.id} className={`memo ${lit ? "lit" : ""} ${proposal ? "proposal" : ""}`}>
      <header>
        <h3>{proposal?.title || title(n)}</h3>
        <time dateTime={n.started}>
          {when(n.started)}
          {n.duration > 0 && ` · ${n.duration < 60 ? t("{n} с", { n: Math.round(n.duration) }) : length(n.duration)}`}
        </time>
      </header>

      {n.status === "failed" ? (
        <p className="memo-state failed">
          {tr(n.problem ?? "") || t("Не вдалося розшифрувати")}
          <button onClick={() => act(() => Meetings.Again(n.id))}>{t("Спробувати ще раз")}</button>
        </p>
      ) : n.status === "queued" ? (
        <p className="memo-state">
          {n.wait === "time"
            ? t(
                new Date(n.until).toDateString() === new Date().toDateString()
                  ? "Почнеться о {time}."
                  : "Почнеться завтра о {time}.",
                { time: new Date(n.until).toLocaleTimeString(locale(), { hour: "2-digit", minute: "2-digit" }) },
              )
            : {
                "": t("Чекає своєї черги."),
                models: t("Почнеться, щойно завантажаться моделі."),
                recording: t("Почнеться, коли закінчиться поточний запис."),
                next: t("Розшифрується наступною."),
                user: t("Почнеться, коли ви 5 хвилин не користуватиметесь Mac."),
                busy: t("Почнеться, коли Mac звільниться від іншої роботи."),
                turn: t("Чекає своєї черги."),
              }[n.wait]}
          {n.wait !== "next" && n.wait !== "models" && (
            <button onClick={() => act(() => Meetings.Rush(n.id))}>{t("Розшифрувати зараз")}</button>
          )}
        </p>
      ) : n.status === "transcribing" ? (
        <p className="memo-state">
          {t("Розшифровую · {n}%", { n: Math.round(n.progress * 100) })}
          <i className="memo-progress" style={{ "--p": n.progress } as React.CSSProperties} />
        </p>
      ) : null}

      {proposal && <p className="memo-proposal">{t("Новий текст")}</p>}
      {s?.overview ? (
        <Prose text={s.overview} />
      ) : n.said ? (
        <Prose text={n.said} raw />
      ) : n.status === "done" && !n.turns ? (
        <p className="memo-state">{t("Нічого не розібрав у записі.")}</p>
      ) : null}

      {(n.status === "summarising" || working(d)) && (
        <p className="memo-state writing">
          <Sparkles size={12} className="animate-pulse" />
          {working(d) ? `${t("Пишу текст")} · ${clock((now - d!.began) / 1000)}` : t("Пишу текст")}
        </p>
      )}

      {!!s?.action_items?.length && (
        <ul className="memo-todos" aria-label={t("Справи")}>
          {s.action_items.map((a, i) => (
            <li key={i} className={a.done ? "done" : ""}>
              <button
                role="checkbox"
                aria-checked={a.done}
                aria-label={t("Виконано: {task}", { task: a.task })}
                disabled={!!proposal}
                onClick={() => act(() => Meetings.Tick(n.id, i, !a.done))}
              >
                {a.done && <Check size={11} />}
              </button>
              <span>{a.task}</span>
              {a.due && <time className={!a.done && late(a.due) ? "late" : ""}>{due(a.due)}</time>}
            </li>
          ))}
        </ul>
      )}

      {d?.problem && (
        <p className="reader-error" role="alert">
          {d.problem}
          <button onClick={write}>{t("Спробувати ще раз")}</button>
          <button onClick={() => drop(n.id)} aria-label={t("Закрити повідомлення")}>
            <X size={13} />
          </button>
        </p>
      )}

      {said && (
        <div className="memo-said">
          {said.map((turn, i) => (
            <p key={i}>
              <button onClick={() => onPlay(turn.start)} disabled={!n.audio}>
                {clock(turn.start)}
              </button>
              {turn.text}
            </p>
          ))}
        </div>
      )}

      {proposal ? (
        <footer>
          <button className="ui-primary" onClick={() => onAccept(d!)}>
            {t("Прийняти")}
          </button>
          <button className="ui-chip" onClick={() => drop(n.id)}>
            {n.summary ? t("Залишити як було") : t("Відкинути")}
          </button>
        </footer>
      ) : (
        <footer>
          {n.audio && (
            <button className="memo-play" onClick={() => onPlay(0)} aria-label={t("Слухати")} title={t("Слухати")}>
              <Play size={11} fill="currentColor" />
            </button>
          )}
          {!busy && (
            <span className="memo-filing">
              <Filing recording={n} groups={groups} onFile={(g) => act(() => Meetings.File(n.id, g))} />
              {filed && (
                <span style={{ color: colourOf(filed.name, filed.colour) }} className="truncate">
                  {filed.name}
                </span>
              )}
            </span>
          )}
          <button
            className="ui-icon memo-more"
            popoverTarget={menu}
            style={{ anchorName: `--${menu}` } as React.CSSProperties}
            aria-label={t("Дії нотатки")}
            title={t("Дії нотатки")}
          >
            <MoreHorizontal size={14} />
          </button>
          <div
            popover="auto"
            id={menu}
            className="memo-menu"
            style={{ positionAnchor: `--${menu}` } as React.CSSProperties}
          >
            {n.turns > 0 && (
              <button
                onClick={async (e) => {
                  shut(e)
                  if (said) return setSaid(null)
                  try {
                    setSaid(((await Meetings.Open(n.id))?.transcript as Turn[]) ?? [])
                  } catch (err) {
                    onProblem(why(err))
                  }
                }}
              >
                <ScrollText size={13} />
                {said ? t("Сховати розшифровку") : t("Розшифровка")}
              </button>
            )}
            {ai && n.turns > 0 && !busy && !working(d) && (
              <button
                onClick={(e) => {
                  shut(e)
                  write()
                }}
              >
                <Sparkles size={13} />
                {n.summary ? t("Переписати текст") : t("Написати текст")}
              </button>
            )}
            <button
              onClick={(e) => {
                shut(e)
                onOpen()
              }}
            >
              <FileText size={13} />
              {t("Відкрити сторінку запису")}
            </button>
            {!!(s?.overview || n.said) && (
              <button
                onClick={(e) => {
                  shut(e)
                  void copyText(plain(n, s)).catch((err) => onProblem(why(err)))
                }}
              >
                <Copy size={13} />
                {t("Копіювати текст")}
              </button>
            )}
            <button
              className="danger"
              onClick={(e) => {
                shut(e)
                onDelete()
              }}
            >
              <Trash2 size={13} />
              {t("У кошик")}
            </button>
          </div>
        </footer>
      )}
    </article>
  )
}

/**
 * A note's text: paragraphs, and "- " lines as a list. A long one is folded to
 * its first lines, so one hour of thinking aloud does not stretch a row of
 * cards.
 */
function Prose({ text, raw }: { text: string; raw?: boolean }) {
  const box = useRef<HTMLDivElement>(null)
  const [long, setLong] = useState(false)
  const [open, setOpen] = useState(false)
  const blocks: (string | string[])[] = []
  for (const line of text.split("\n").map((l) => l.trim()).filter(Boolean)) {
    const item = /^[-•]\s+(.*)/.exec(line)?.[1]
    const last = blocks.at(-1)
    if (item === undefined) blocks.push(line)
    else if (Array.isArray(last)) last.push(item)
    else blocks.push([item])
  }
  // Measured again when the card's width changes: a narrower window makes the
  // same text taller than the fold.
  useLayoutEffect(() => {
    const el = box.current
    if (!el || open) return
    const measure = () => setLong(el.scrollHeight > el.clientHeight + 4)
    measure()
    const watch = new ResizeObserver(measure)
    watch.observe(el)
    return () => watch.disconnect()
  }, [text, open])
  return (
    <>
      <div ref={box} className={`memo-text ${raw ? "raw" : ""} ${open ? "" : "folded"} ${long && !open ? "cut" : ""}`}>
        {blocks.map((b, i) =>
          Array.isArray(b) ? (
            <ul key={i}>
              {b.map((x, j) => (
                <li key={j}>{x}</li>
              ))}
            </ul>
          ) : (
            <p key={i}>{b}</p>
          ),
        )}
      </div>
      {long && (
        <button className="memo-unfold" onClick={() => setOpen(!open)}>
          {open ? t("Згорнути") : t("Повністю")}
        </button>
      )}
    </>
  )
}

/** A note's name; one not written out yet is named by the file, which says nothing. */
const title = (n: VoiceNote) => (n.title.endsWith(".wav") ? t("Нотатка") : n.title)

/** The note as text for the clipboard. */
function plain(n: VoiceNote, s?: Summary | null) {
  const todos = (s?.action_items ?? []).map((a) => `- [${a.done ? "x" : " "}] ${a.task}${a.due ? ` (${due(a.due)})` : ""}`)
  return [title(n), "", s?.overview || n.said, ...(todos.length ? ["", ...todos] : [])].join("\n")
}
