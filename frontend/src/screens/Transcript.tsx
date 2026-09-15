import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  Check,
  ChevronLeft,
  Copy,
  ListChecks,
  MoreHorizontal,
  Search,
  Sparkles,
  Trash2,
  Users,
  X,
  PanelRight,
  Focus,
  Plus,
  ListTree,
  RotateCcw,
  ArrowDownToLine,
} from "lucide-react"
import {
  Meetings,
  clock,
  length,
  when,
  type Group,
  type Meeting,
  type Person,
  type Summary,
} from "../api"
import { colourOf, palette } from "../colours"
import Shape from "../components/Shape"
import Player, { type Controls } from "../components/Player"
import Voices from "../components/Voices"
import Reveal from "../components/Reveal"
import Drawer, { useNarrow } from "../components/Drawer"
import NotesDeck from "../components/NotesDeck"
import ActionEditor from "../components/ActionEditor"

export default function Transcript({
  id,
  groups,
  onBack,
  onChanged,
  at: openAt,
  onFocus,
  focused = false,
  onList,
  onReturn,
  returnLabel,
}: {
  id: number
  groups: Group[]
  onBack: () => void
  onChanged: () => void
  at?: number
  onFocus?: () => void
  focused?: boolean
  onList?: () => void
  onReturn?: () => void
  returnLabel?: string
}) {
  const [meeting, setMeeting] = useState<Meeting | null>(null),
    [people, setPeople] = useState<Person[]>([]),
    [title, setTitle] = useState(""),
    [tab, setTab] = useState<"summary" | "turns">(
      openAt !== undefined ? "turns" : "summary",
    ),
    [lit, setLit] = useState<string | null>(null),
    [needle, setNeedle] = useState(""),
    [finding, setFinding] = useState(false),
    [right, setRight] = useState(false),
    [problem, setProblem] = useState(""),
    [thinking, setThinking] = useState(false),
    [draft, setDraft] = useState<Summary | null>(null),
    [showDone, setShowDone] = useState(false),
    [editing, setEditing] = useState<number | null>(null),
    [undo, setUndo] = useState<null | (() => Promise<void>)>(null),
    [at, setAt] = useState(0),
    [seek, setSeek] = useState(0),
    [seed, setSeed] = useState(""),
    [selected, setSelected] = useState(""),
    [renaming, setRenaming] = useState<string | null>(null),
    [menu, setMenu] = useState(false)
  const scroll = useRef<HTMLDivElement>(null),
    player = useRef<Controls>(null),
    cancelTitle = useRef(false),
    narrow = useNarrow(1339)
  const load = useCallback(async () => {
    try {
      const m = (await Meetings.Open(id)) as Meeting
      setMeeting(m)
      setTitle(m.title)
      setPeople((await Meetings.People()) as Person[])
    } catch (e) {
      setProblem(String(e))
    }
  }, [id])
  useEffect(() => {
    void load()
  }, [load])
  useEffect(() => {
    if (!meeting || meeting.status === "done" || meeting.status === "failed")
      return
    const timer = setInterval(() => void load(), 1800)
    return () => clearInterval(timer)
  }, [meeting?.status, load])
  // Every jump moves the audio. It used to move only the playhead unless the
  // caller asked to play, so a chapter or a search hit lit up a row, set the
  // clock and slid the voice map — while the sound stayed at 0:00 and snapped
  // everything back the moment anybody pressed play.
  const jump = useCallback((seconds: number, play = false) => {
    setTab("turns")
    setNeedle("")
    setAt(seconds)
    setSeek((v) => v + 1)
    player.current?.go(seconds, play)
  }, [])
  useEffect(() => {
    if (openAt !== undefined && meeting) jump(openAt)
  }, [openAt, !!meeting, jump])
  useEffect(() => {
    if (tab !== "turns" || !meeting) return
    const timer = setTimeout(() => {
      const turns = meeting.transcript
      const nearest = turns.reduce(
        (best, t) =>
          Math.abs(t.start - at) < Math.abs(best.start - at) ? t : best,
        turns[0],
      )
      if (nearest)
        scroll.current
          ?.querySelector<HTMLElement>(`[data-at="${nearest.start}"]`)
          ?.scrollIntoView({ block: "center", behavior: "smooth" })
    }, 80)
    return () => clearTimeout(timer)
  }, [tab, seek, !!meeting])
  useEffect(() => {
    if (!undo) return
    const timer = setTimeout(() => setUndo(null), 8000)
    return () => clearTimeout(timer)
  }, [undo])
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      const typing = (e.target as HTMLElement).closest(
        "input,textarea,select,dialog",
      )
      if (e.key === "f" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setFinding((v) => !v)
      }
      if (typing) return
      if (e.key === "f" && !e.metaKey && !e.ctrlKey) onFocus?.()
      if (e.key === "Escape") {
        setMenu(false)
        setFinding(false)
        setLit(null)
      }
    }
    window.addEventListener("keydown", key)
    return () => window.removeEventListener("keydown", key)
  }, [onFocus])
  const colours = useMemo(
    () =>
      palette(
        meeting?.speakers ?? [],
        new Map(people.map((p) => [p.name, p.colour])),
      ),
    [meeting?.speakers, people],
  )
  if (!meeting)
    return (
      <div className="reader-loading">{problem || "Відкриваю документ…"}</div>
    )
  const s = meeting.summary,
    shown = meeting.transcript.filter((t) =>
      t.text.toLowerCase().includes(needle.toLowerCase()),
    ),
    filed = groups.find((g) => g.id === meeting.group)
  const tick = async (i: number) => {
    try {
      const previous = !!s?.action_items?.[i].done
      await Meetings.Tick(id, i, !previous)
      await load()
      setUndo(() => async () => {
        await Meetings.Tick(id, i, previous)
        await load()
      })
      onChanged()
    } catch (e) {
      setProblem(String(e))
    }
  }
  const preview = async () => {
    setTab("summary")
    setThinking(true)
    setProblem("")
    try {
      setDraft((await Meetings.PreviewSummary(id)) as Summary)
    } catch (e) {
      setProblem(String(e))
    } finally {
      setThinking(false)
    }
  }
  const analytics = (
    <>
      <Shape id={id} colours={colours} onJump={(t) => jump(t, true)} />
      <div className="speaker-editor">
        <span>Імена учасників</span>
        {meeting.speakers?.map((name) => (
          <SpeakerChip
            key={name}
            name={name}
            colour={colours.get(name)!}
            editing={renaming === name}
            onEdit={() => setRenaming(name)}
            onDone={async (to) => {
              setRenaming(null)
              if (to && to !== name) {
                await Meetings.Rename(id, name, to)
                await load()
                onChanged()
              }
            }}
          />
        ))}
      </div>
    </>
  )
  return (
    <div className={`meeting-reader ${focused ? "is-focused" : ""}`}>
      <header className="reader-toolbar">
        <button className="ui-chip" onClick={onList ?? onBack}>
          <ChevronLeft size={14} />
          {onList ? "Список зустрічей" : "Записи"}
        </button>
        {onReturn && (
          <button className="ui-chip accent" onClick={onReturn}>
            ← {returnLabel || "До пошуку"}
          </button>
        )}
        <span className="toolbar-spacer" />
        <button
          className="ui-icon"
          aria-label="Знайти в документі"
          onClick={() => setFinding((v) => !v)}
        >
          <Search size={15} />
        </button>
        <button
          className="ui-icon"
          aria-label="Зосереджене читання"
          aria-pressed={focused}
          onClick={onFocus}
        >
          <Focus size={16} />
        </button>
        {narrow && (
          <button
            className="ui-icon"
            aria-label="Голоси й ритм"
            onClick={() => setRight(true)}
          >
            <PanelRight size={16} />
          </button>
        )}
        <div className="relative">
          <button
            className="ui-icon"
            aria-label="Дії зустрічі"
            onClick={() => setMenu(!menu)}
          >
            <MoreHorizontal size={17} />
          </button>
          {menu && (
            <div className="reader-menu">
              <button
                onClick={async () => {
                  await navigator.clipboard.writeText(
                    await Meetings.Markdown(id),
                  )
                  setMenu(false)
                }}
              >
                <Copy size={14} />
                Копіювати Markdown
              </button>
              <button
                onClick={async () => {
                  try {
                    await Meetings.Again(id)
                    onChanged()
                    onBack()
                  } catch (e) {
                    setProblem(String(e))
                  }
                }}
              >
                <RotateCcw size={14} />
                Розшифрувати заново
              </button>
              <button
                onClick={async () => {
                  await Meetings.Delete(id)
                  onChanged()
                  onBack()
                }}
              >
                <Trash2 size={14} />У кошик
              </button>
            </div>
          )}
        </div>
      </header>
      <div className="reader-heading">
        <input
          aria-label="Назва зустрічі"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              cancelTitle.current = true
              setTitle(meeting.title)
              e.currentTarget.blur()
            }
            if (e.key === "Enter") e.currentTarget.blur()
          }}
          onBlur={async () => {
            if (cancelTitle.current) {
              cancelTitle.current = false
              return
            }
            if (title.trim() && title !== meeting.title) {
              try {
                await Meetings.Retitle(id, title.trim())
                await load()
                onChanged()
              } catch (e) {
                setProblem(String(e))
                setTitle(meeting.title)
              }
            }
          }}
        />
        <div className="reader-meta">
          <label className="date-edit">
            {when(meeting.started)}
            <input
              aria-label="Дата зустрічі"
              type="datetime-local"
              value={local(meeting.started)}
              onChange={async (e) => {
                if (e.target.value) {
                  await Meetings.Redate(
                    id,
                    new Date(e.target.value).toISOString(),
                  )
                  await load()
                  onChanged()
                }
              }}
            />
          </label>
          <span>·</span>
          <span>{length(meeting.duration)}</span>
          {filed && (
            <>
              <span>·</span>
              <b style={{ color: colourOf(filed.name, filed.colour) }}>
                {filed.name}
              </b>
            </>
          )}
        </div>
        <div className="reader-instruments">
          <Reveal
            className="voices-reveal"
            label={
              <>
                <Users size={13} />
                {meeting.speakers?.length || 1} голосів
              </>
            }
          >
            <Voices
              turns={meeting.transcript}
              colours={colours}
              at={at}
              lit={lit}
              length={meeting.duration}
              onJump={(t) => jump(t, true)}
              onLight={setLit}
            />
          </Reveal>
          {!!s?.chapters?.length && (
            <Reveal
              hover={false}
              label={
                <>
                  <ListTree size={13} />
                  Глави <small>{s.chapters.length}</small>
                </>
              }
            >
              {s.chapters.map((c, i) => (
                <button
                  data-close-reveal
                  className="chapter-option"
                  key={i}
                  onClick={() => jump(c.start)}
                >
                  <time>{clock(c.start)}</time>
                  <span>{c.title}</span>
                </button>
              ))}
            </Reveal>
          )}
          <NotesDeck
            recording={id}
            legacy={meeting.note}
            seed={seed}
            onSeedUsed={() => setSeed("")}
          />
          <button className="ui-chip" disabled={thinking} onClick={preview}>
            <Sparkles size={13} className={thinking ? "animate-pulse" : ""} />
            {thinking ? "Читаю…" : "Оновити підсумок"}
          </button>
        </div>
      </div>
      {meeting.status !== "done" && (
        <div className="processing-state" role="status">
          <div className="processing-steps">
            {["Аудіо", "Розшифровка", "Підсумок"].map((label, i) => (
              <span
                key={label}
                className={
                  i === 0 || (i === 1 && meeting.transcript.length > 0)
                    ? "complete"
                    : meeting.status === "failed"
                      ? "failed"
                      : "pending"
                }
              >
                {i === 0 || (i === 1 && meeting.transcript.length > 0) ? (
                  <Check size={12} />
                ) : (
                  <span className="step-dot" />
                )}
                {label}
              </span>
            ))}
            <b>
              {
                {
                  queued: "У черзі",
                  transcribing: "Розпізнаю мовлення й учасників",
                  summarising: "Готую підсумок",
                  failed: "Обробку зупинено",
                  done: "Готово",
                }[meeting.status]
              }
            </b>
          </div>
          {meeting.status === "failed" ? (
            <>
              <p>{meeting.problem}</p>
              <button
                className="ui-chip"
                onClick={async () => {
                  try {
                    await Meetings.Again(id)
                    await load()
                    onChanged()
                  } catch (e) {
                    setProblem(String(e))
                  }
                }}
              >
                Повторити обробку
              </button>
              {meeting.transcript.length > 0 && (
                <button className="ui-chip" onClick={preview}>
                  Лише новий підсумок
                </button>
              )}
            </>
          ) : (
            <progress
              aria-label="Обробка зустрічі"
              max={1}
              value={meeting.progress}
            />
          )}
        </div>
      )}
      {finding && (
        <div className="reader-find">
          <Search size={13} />
          <input
            autoFocus
            aria-label="Пошук у документі"
            value={needle}
            onChange={(e) => {
              setNeedle(e.target.value)
              setTab("turns")
            }}
            placeholder="Знайти у розшифровці…"
          />
          <button
            className="ui-icon"
            aria-label="Закрити пошук"
            onClick={() => {
              setFinding(false)
              setNeedle("")
            }}
          >
            <X size={14} />
          </button>
        </div>
      )}
      {problem && (
        <p className="reader-error" role="alert">
          {problem}
          <button
            onClick={() => setProblem("")}
            aria-label="Закрити повідомлення"
          >
            <X size={13} />
          </button>
        </p>
      )}
      <div className={`reader-columns ${narrow || focused ? "single" : ""}`}>
        <section className="document-column">
          <div className="document-tabs">
            <button
              className={tab === "summary" ? "active" : ""}
              onClick={() => setTab("summary")}
            >
              Підсумок
            </button>
            <button
              className={tab === "turns" ? "active" : ""}
              onClick={() => setTab("turns")}
            >
              Розшифровка
            </button>
            {lit && (
              <button className="voice-spot" onClick={() => setLit(null)}>
                {lit}
                <X size={11} />
              </button>
            )}
            {tab === "turns" && (
              <button
                className="follow-current"
                onClick={() => {
                  const target =
                    meeting.transcript.find(
                      (t) => at >= t.start && at < t.end,
                    ) ||
                    meeting.transcript.reduce(
                      (a, b) =>
                        Math.abs(a.start - at) < Math.abs(b.start - at) ? a : b,
                      meeting.transcript[0],
                    )
                  if (target)
                    scroll.current
                      ?.querySelector(`[data-at="${target.start}"]`)
                      ?.scrollIntoView({ block: "center", behavior: "smooth" })
                }}
              >
                <ArrowDownToLine size={12} />
                {clock(at)}
              </button>
            )}
          </div>
          <div
            ref={scroll}
            className="document-scroll"
            onMouseUp={() => {
              const selection = getSelection()
              if (
                selection &&
                selection.toString().trim().length > 4 &&
                scroll.current?.contains(selection.anchorNode)
              )
                setSelected(selection.toString())
              else setSelected("")
            }}
          >
            {tab === "summary" ? (
              <>
                {draft && (
                  <div className="summary-comparison">
                    <header>
                      <Sparkles size={14} />
                      Нова редакція · назва й ручні нотатки збережуться
                    </header>
                    <div className="comparison-columns">
                      <section>
                        <small>Зараз</small>
                        <SummaryReview summary={s} />
                      </section>
                      <section>
                        <small>Пропозиція</small>
                        <SummaryReview summary={draft} />
                      </section>
                    </div>
                    <footer>
                      <button
                        className="ui-primary"
                        onClick={async () => {
                          try {
                            const before = s ?? null,
                              after = draft
                            await Meetings.AcceptSummary(
                              id,
                              before as never,
                              after as never,
                            )
                            setDraft(null)
                            await load()
                            onChanged()
                            if (before)
                              setUndo(() => async () => {
                                await Meetings.AcceptSummary(
                                  id,
                                  after as never,
                                  before as never,
                                )
                                await load()
                              })
                          } catch (e) {
                            setProblem(String(e))
                          }
                        }}
                      >
                        Прийняти
                      </button>
                      <button
                        className="ui-chip"
                        onClick={() => setDraft(null)}
                      >
                        Залишити поточний
                      </button>
                    </footer>
                  </div>
                )}
                {s ? (
                  <>
                    <p className="summary-overview">{s.overview}</p>
                    {!!s.decisions?.length && (
                      <section className="doc-block">
                        <h3>
                          <Check size={13} />
                          Вирішено <small>{s.decisions.length}</small>
                        </h3>
                        <ul className="decision-list">
                          {s.decisions.map((d, i) => (
                            <li key={i}>{d}</li>
                          ))}
                        </ul>
                      </section>
                    )}
                    <section className="doc-block">
                      <h3>
                        <ListChecks size={13} />
                        Домовленості <span />
                        <button
                          aria-pressed={showDone}
                          onClick={() => setShowDone(!showDone)}
                        >
                          {showDone ? "Сховати виконані" : "Показати виконані"}
                        </button>
                        <button
                          className="ui-icon"
                          aria-label="Додати домовленість"
                          onClick={() => setEditing(-1)}
                        >
                          <Plus size={15} />
                        </button>
                      </h3>
                      {s.action_items?.map(
                        (a, i) =>
                          (showDone || !a.done) && (
                            <div
                              className={`commitment ${a.done ? "done" : ""}`}
                              key={i}
                            >
                              <button
                                role="checkbox"
                                aria-checked={a.done}
                                aria-label={`Виконано: ${a.task}`}
                                onClick={() => void tick(i)}
                              >
                                {a.done && <Check size={12} />}
                              </button>
                              <div>
                                <p>{a.task}</p>
                                <button
                                  className="commitment-meta"
                                  onClick={() => setEditing(i)}
                                >
                                  {a.owner || "Призначити"}{" "}
                                  <span>· {a.due || "Без строку"}</span>
                                </button>
                              </div>
                              <button
                                className="context-action ui-icon"
                                aria-label="Редагувати домовленість"
                                onClick={() => setEditing(i)}
                              >
                                <MoreHorizontal size={14} />
                              </button>
                            </div>
                          ),
                      )}
                    </section>
                    {!!s.open_questions?.length && (
                      <section className="doc-block">
                        <h3>
                          <Search size={13} />
                          Відкриті питання
                        </h3>
                        <ul className="decision-list">
                          {s.open_questions.map((q, i) => (
                            <li key={i}>
                              {q}
                              <button
                                className="context-action"
                                title="Зберегти в нотатках"
                                onClick={() => setSeed(q)}
                              >
                                <Plus size={13} />
                              </button>
                            </li>
                          ))}
                        </ul>
                      </section>
                    )}
                  </>
                ) : (
                  <div className="empty-reader">
                    <Sparkles size={20} />
                    <p>Підсумку ще немає. Розшифровка доступна поруч.</p>
                    <button
                      className="ui-primary"
                      onClick={preview}
                      disabled={thinking}
                    >
                      Створити підсумок
                    </button>
                  </div>
                )}
              </>
            ) : (
              <div className="transcript-turns">
                {shown.map((t, i) => (
                  <div
                    className={`transcript-turn ${at >= t.start && at < t.end ? "playing" : ""} ${lit && lit !== t.speaker ? "dimmed" : ""}`}
                    key={`${t.start}-${i}`}
                    data-at={t.start}
                  >
                    <button
                      className="turn-time"
                      onClick={() => player.current?.go(t.start)}
                      aria-label={`Програти з ${clock(t.start)}`}
                    >
                      {clock(t.start)}
                    </button>
                    <div>
                      <button
                        className="turn-speaker"
                        style={{ color: colours.get(t.speaker || "") }}
                        onClick={() =>
                          setLit(lit === t.speaker ? null : (t.speaker ?? null))
                        }
                      >
                        {t.speaker}
                      </button>
                      <p>{t.text}</p>
                    </div>
                    <button
                      className="context-action ui-icon"
                      aria-label="Копіювати репліку"
                      onClick={() => navigator.clipboard.writeText(t.text)}
                    >
                      <Copy size={12} />
                    </button>
                  </div>
                ))}
                {!shown.length && (
                  <p className="empty-reader">Збігів немає. Змініть запит.</p>
                )}
              </div>
            )}
          </div>
        </section>
        {!narrow && !focused && (
          <aside className="reader-inspector">{analytics}</aside>
        )}
      </div>
      {narrow && (
        <Drawer
          open={right}
          onClose={() => setRight(false)}
          title="Голоси й ритм"
        >
          {analytics}
        </Drawer>
      )}
      {meeting.audio && (
        <Player
          key={id}
          id={id}
          file={meeting.audio}
          onTime={setAt}
          ref={player}
        />
      )}
      {selected && (
        <div className="selection-actions">
          <button
            onClick={() => {
              navigator.clipboard.writeText(selected)
              setSelected("")
            }}
          >
            <Copy size={13} />
            Копіювати
          </button>
          <button
            onClick={() => {
              setSeed(selected)
              setSelected("")
            }}
          >
            + Нотатка
          </button>
          <button
            onClick={async () => {
              try {
                await Meetings.EditAction(id, -1, {
                  task: selected,
                  owner: "",
                  due: "",
                  done: false,
                } as never)
                setSelected("")
                await load()
              } catch (e) {
                setProblem(String(e))
              }
            }}
          >
            + Домовленість
          </button>
          <button onClick={() => setSelected("")} aria-label="Закрити">
            <X size={13} />
          </button>
        </div>
      )}
      {editing !== null && (
        <ActionEditor
          recording={id}
          index={editing}
          action={editing >= 0 ? s?.action_items?.[editing] : undefined}
          onClose={() => setEditing(null)}
          onSaved={() => {
            void load()
            onChanged()
          }}
        />
      )}
      {undo && (
        <div className="action-toast" role="status">
          Зміни збережено
          <button
            onClick={async () => {
              try {
                await undo()
                setUndo(null)
              } catch (e) {
                setProblem(String(e))
              }
            }}
          >
            <RotateCcw size={12} />
            Скасувати
          </button>
        </div>
      )}
    </div>
  )
}

/** What an <input type="datetime-local"> wants: local time, no zone, no seconds. */
function local(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function SpeakerChip({
  name,
  colour,
  editing,
  onEdit,
  onDone,
}: {
  name: string
  colour: string
  editing: boolean
  onEdit: () => void
  onDone: (to: string) => void
}) {
  const [value, setValue] = useState(name)
  useEffect(() => setValue(name), [name, editing])

  if (editing) {
    return (
      <input
        autoFocus
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => onDone(value.trim())}
        onKeyDown={(e) => {
          if (e.key === "Enter") onDone(value.trim())
          if (e.key === "Escape") onDone("")
        }}
        className="w-32 rounded-full border border-accent/50 bg-surface px-2.5 py-0.5 text-[11.5px] outline-none"
      />
    )
  }
  return (
    <button
      onClick={onEdit}
      title="Змінити ім’я учасника"
      className="flex items-center gap-1.5 rounded-full border border-line/60 bg-surface/60 px-2.5 py-0.5 text-[11.5px] text-soft transition-colors hover:border-accent/40 hover:text-text"
    >
      <span className="size-1.5 rounded-full" style={{ background: colour }} />
      {name}
    </button>
  )
}

function SummaryReview({ summary }: { summary?: Summary | null }) {
  if (!summary) return <p>Підсумку ще немає</p>
  return (
    <div className="summary-review">
      <p>{summary.overview}</p>
      {!!summary.decisions?.length && (
        <>
          <b>Рішення</b>
          <ul>
            {summary.decisions.map((d, i) => (
              <li key={i}>{d}</li>
            ))}
          </ul>
        </>
      )}
      {!!summary.action_items?.length && (
        <>
          <b>Домовленості</b>
          <ul>
            {summary.action_items.map((a, i) => (
              <li key={i}>
                {a.done ? "✓ " : ""}
                {a.task} · {a.owner} · {a.due || "без строку"}
              </li>
            ))}
          </ul>
        </>
      )}
      {!!summary.open_questions?.length && (
        <>
          <b>Питання</b>
          <ul>
            {summary.open_questions.map((q, i) => (
              <li key={i}>{q}</li>
            ))}
          </ul>
        </>
      )}
      {!!summary.chapters?.length && (
        <>
          <b>Глави</b>
          <ul>
            {summary.chapters.map((c, i) => (
              <li key={i}>
                {clock(c.start)} {c.title}
                {c.summary && <p>{c.summary}</p>}
              </li>
            ))}
          </ul>
        </>
      )}
      {!!summary.topics?.length && <small>{summary.topics.join(" · ")}</small>}
    </div>
  )
}
