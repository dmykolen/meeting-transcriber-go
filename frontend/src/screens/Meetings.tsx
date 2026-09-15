import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Dialogs } from "@wailsio/runtime"
import { motion } from "motion/react"
import {
  Check,
  FileAudio,
  FolderInput,
  Import,
  Mic,
  Trash,
  Trash2,
} from "lucide-react"
import {
  Meetings as Api,
  length,
  when,
  type Group,
  type Listening,
  type Mark,
  type Recording,
} from "../api"
import { colourOf } from "../colours"
import Timeline from "../components/EdgeTimeline"
import Drawer, { useNarrow } from "../components/Drawer"
import Dock from "../components/Dock"
import Confirm from "../components/Confirm"
import MeetingPeek from "../components/MeetingPeek"
import LiveNow from "../components/LiveNow"
import Head, { Verb } from "../components/Head"
import Undo from "../components/Undo"
import Transcript from "./Transcript"
import Project from "./Project"

const day = (iso: string) => iso.slice(0, 10)

/**
 * Everything the app has heard, as one workspace.
 *
 * This replaced a screen that was a list of cards and a screen that was one
 * meeting. They are the same screen: a list you scan and a thing you read, side
 * by side, so that following an argument across three meetings does not mean
 * leaving and coming back twice.
 *
 * Three instruments, and no filter panel anywhere: the timeline picks a
 * stretch of time, the dock picks a project, and the search box picks words.
 */
export default function Workspace({
  pick,
  onPicked,
  project,
  onProject,
  pickAt,
  onReturn,
  returnLabel,
}: {
  pickAt?: number
  onReturn?: () => void
  returnLabel?: string
  pick: number | null
  onPicked: () => void
  /** Held above, so the palette in the title bar can open a project too. */
  project: number | null
  onProject: (id: number | null) => void
}) {
  const narrow = useNarrow(1049)
  const [listOpen, setListOpen] = useState(false)
  const [focused, setFocused] = useState(false)
  const [problem, setProblem] = useState("")
  const [confirmBin, setConfirmBin] = useState(false)
  const [rows, setRows] = useState<Recording[] | null>(null)
  const [marks, setMarks] = useState<Mark[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [live, setLive] = useState<Listening | null>(null)

  const [range, setRange] = useState<[string, string] | null>(null)
  const [open, setOpen] = useState<number | null>(null)
  /** A second to open the meeting at, when it was reached from a line of a
      project document rather than from the list. */
  const [moment, setMoment] = useState<number | undefined>()
  // Something is open in the reader. On a narrow window it owns the width;
  // wide, the list keeps its place beside it.
  const reading = open !== null || project !== null
  const [binned, setBinned] = useState(false)
  /** The recording under the pointer, whichever pane it is in. The timeline and
      the list are two views of the same thing, so pointing at one shows the
      other where it is. */
  const [lit, setLit] = useState<number | null>(null)

  const [naming, setNaming] = useState(false)
  const [fresh, setFresh] = useState("")
  const [undone, setUndone] = useState<{ id: number; title: string } | null>(
    null,
  )

  // A meeting opened from Today, Search or To do lands here already selected.
  useEffect(() => {
    if (pick !== null) {
      setOpen(pick)
      setMoment(pickAt)
      setListOpen(false)
      onPicked()
    }
  }, [pick, onPicked, pickAt])

  const load = useCallback(async () => {
    try {
      setRows((await (binned ? Api.Bin() : Api.Recent(10000))) as Recording[])
    } catch (e) {
      setProblem(String(e))
    }
    Api.Groups().then((g) => setGroups((g as Group[]) ?? []))
    Api.Span(365).then((m) => setMarks((m as Mark[]) ?? []))
  }, [binned])

  useEffect(() => {
    const tick = () => {
      load()
      Api.Listening()
        .then((s) => setLive(s as Listening))
        .catch(() => {})
    }
    tick()
    const timer = setInterval(tick, 4000)
    return () => clearInterval(timer)
  }, [load])

  // The three instruments compose: a project, a stretch of time, and the bin.
  const shown = useMemo(
    () =>
      (rows ?? []).filter(
        (r) =>
          (project === null || r.group === project) &&
          (!range ||
            (day(r.started) >= range[0] && day(r.started) <= range[1])),
      ),
    [rows, project, range],
  )

  const file = useCallback(
    async (recording: number, group: number) => {
      await Api.File(recording, group)
      load()
    },
    [load],
  )

  const drag = useDrag(file)

  // Pointing at a mark in the timeline brings its row into view. Without this,
  // lighting a meeting three hundred rows down is a highlight nobody sees.
  useEffect(() => {
    if (lit === null) return
    document
      .querySelector(`[data-rec="${lit}"]`)
      ?.scrollIntoView({ block: "nearest", behavior: "smooth" })
  }, [lit])

  // j and k walk the list, Enter opens what is under the cursor. The list is
  // long and the pointer is a slow way through four hundred meetings.
  useEffect(() => {
    const press = (e: KeyboardEvent) => {
      const on = document.activeElement?.tagName
      if (on === "INPUT" || on === "TEXTAREA" || e.metaKey || e.ctrlKey) return
      const step = e.key === "j" ? 1 : e.key === "k" ? -1 : 0
      if (step) {
        e.preventDefault()
        const at = shown.findIndex((r) => r.id === (lit ?? open))
        const next = shown[Math.min(Math.max(at + step, 0), shown.length - 1)]
        if (next) setLit(next.id)
      }
      if (e.key === "Enter" && lit !== null) {
        e.preventDefault()
        setOpen(lit)
      }
    }
    window.addEventListener("keydown", press)
    return () => window.removeEventListener("keydown", press)
  }, [shown, lit, open])

  const list = (
    <motion.div
      key={`${project}-${range?.join("") ?? ""}-${binned}`}
      initial={{ opacity: 0.45 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.18 }}
      className="min-h-0 flex-1 overflow-y-auto pb-[calc(var(--spacing-dock)+0.75rem)]"
    >
      {live && (
        <div className="px-3 pt-2">
          <LiveNow state={live} />
        </div>
      )}
      {rows === null ? null : shown.length === 0 ? (
        <p className="px-4 py-10 text-center text-[11.5px] leading-relaxed text-faint">
          {binned
            ? "У кошику порожньо."
            : project !== null
              ? "У цьому проєкті ще нічого немає. Перетягніть нараду на його плитку внизу."
              : range
                ? "За цей проміжок нічого. Візьміть ширший на смузі вгорі."
                : "Тут порожньо."}
        </p>
      ) : (
        group(shown).map(([date, held]) => (
          <section key={date}>
            <h2 className="sticky top-0 z-10 flex h-[30px] items-center justify-between border-b border-line/50 bg-surface/90 px-3.5 text-[9.5px] text-faint backdrop-blur">
              {when(held[0].started)}
              <span className="tabular-nums">{held.length}</span>
            </h2>
            {held.map((r) => (
              <Row
                key={r.id}
                recording={r}
                groups={groups}
                on={open === r.id}
                lit={lit === r.id}
                binned={binned}
                onLight={setLit}
                onOpen={() => {
                  setMoment(undefined)
                  setOpen(r.id)
                  setListOpen(false)
                }}
                onLift={(e) => drag.lift(r, e)}
                onFile={(g) => file(r.id, g)}
                onDelete={async () => {
                  await Api.Delete(r.id)
                  setUndone({ id: r.id, title: r.title })
                  if (open === r.id) setOpen(null)
                  load()
                }}
                onRestore={async () => {
                  await Api.Restore(r.id)
                  load()
                }}
              />
            ))}
          </section>
        ))
      )}
    </motion.div>
  )

  return (
    <div
      className={`workspace-shell @container/work relative flex h-full min-h-0 flex-col ${focused ? "workspace-focus" : ""}`}
    >
      <Head title="Записи" count={shown.length}>
        {onReturn && open === null && (
          <button className="ui-chip" onClick={onReturn}>
            ← {returnLabel || "Назад"}
          </button>
        )}
        {range && (
          <button
            onClick={() => setRange(null)}
            className="mr-1 rounded-md bg-raised px-2 py-0.5 text-[10px] text-soft transition-colors hover:text-text"
          >
            {range[0]} — {range[1]} ✕
          </button>
        )}
        <Verb on={binned} onClick={() => setBinned((b) => !b)} Icon={Trash2}>
          Кошик
        </Verb>
        {binned && shown.length > 0 && (
          <Verb
            onClick={async () => {
              setConfirmBin(true)
            }}
            Icon={Trash}
          >
            Спорожнити
          </Verb>
        )}
        <Verb
          onClick={async () => {
            const path = await pickFile()
            if (path) {
              await Api.Import(path)
              load()
            }
          }}
          Icon={Import}
        >
          Додати файл
        </Verb>
      </Head>

      <Timeline
        marks={marks}
        groups={groups}
        range={range}
        picked={project}
        lit={lit}
        onRange={setRange}
        onPick={onProject}
        onLight={setLit}
      />

      {/* The layout answers to the window, not to a number picked once.
          Narrow: one pane — the list, or the reader when something is open.
          Normal: a list that grows with the window between 240 and 340.
          Wide: the same, because a list wider than that is not easier to read. */}
      <div className={`workspace-split ${narrow || focused ? "one-pane" : ""}`}>
        {!focused &&
          (narrow ? (
            !reading ? (
              <aside className="meeting-list">{list}</aside>
            ) : (
              <Drawer
                side="left"
                title="Зустрічі"
                open={listOpen}
                onClose={() => setListOpen(false)}
              >
                {list}
              </Drawer>
            )
          ) : (
            <aside className="meeting-list">{list}</aside>
          ))}

        {/* The reader arrives from the side rather than appearing. Which of
            the two it is — a meeting or a project — is what changes, so that is
            what the key follows. */}
        <motion.div
          key={
            open !== null
              ? `m${open}`
              : project !== null
                ? `p${project}`
                : "empty"
          }
          initial={{ opacity: 0, x: 10 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          className={`workspace-reader min-h-0 overflow-hidden ${reading || !narrow ? "" : "hidden"}`}
        >
          {open !== null ? (
            <Transcript
              id={open}
              groups={groups}
              at={moment}
              focused={focused}
              onFocus={() => setFocused((v) => !v)}
              onList={narrow ? () => setListOpen(true) : undefined}
              onReturn={onReturn}
              returnLabel={returnLabel}
              onBack={() => setOpen(null)}
              onChanged={load}
            />
          ) : project !== null ? (
            <Project
              id={project}
              groups={groups}
              onChanged={load}
              onOpen={(id, at = 0) => {
                setMoment(at)
                setOpen(id)
              }}
            />
          ) : (
            <Empty />
          )}
        </motion.div>
      </div>

      {naming && (
        <NameProject
          onDone={async (name) => {
            if (name) await Api.NewGroup(name)
            setNaming(false)
            setFresh("")
            load()
          }}
          value={fresh}
          onChange={setFresh}
        />
      )}

      <Dock
        groups={groups}
        picked={project}
        over={drag.over}
        dragging={drag.lifted}
        onPick={(id) => {
          onProject(id)
          setOpen(null)
        }}
        onNew={() => setNaming(true)}
      />

      {problem && (
        <p role="alert" className="action-toast">
          {problem}
          <button onClick={() => setProblem("")}>Закрити</button>
        </p>
      )}
      {confirmBin && (
        <Confirm title="Очистити кошик" onClose={() => setConfirmBin(false)}>
          <section className="confirm-panel">
            <h2>
              Видалити назавжди всі {rows?.length ?? 0} записів із кошика?
            </h2>
            <p>
              Дія стосується всього кошика, незалежно від фільтра проєкту або
              дати.
            </p>
            <footer>
              <button className="ui-chip" onClick={() => setConfirmBin(false)}>
                Залишити
              </button>
              <button
                className="ui-primary"
                onClick={async () => {
                  try {
                    await Api.EmptyBin()
                    setConfirmBin(false)
                    load()
                  } catch (e) {
                    setProblem(String(e))
                  }
                }}
              >
                Видалити назавжди
              </button>
            </footer>
          </section>
        </Confirm>
      )}
      {drag.card}

      <Undo
        what={undone?.title ?? null}
        onUndo={async () => {
          if (undone) await Api.Restore(undone.id)
          setUndone(null)
          load()
        }}
        onGone={() => setUndone(null)}
      />
    </div>
  )
}

/** Recordings by calendar day, newest first, as the list is grouped. */
function group(rows: Recording[]) {
  const by = new Map<string, Recording[]>()
  for (const r of rows) {
    const k = day(r.started)
    by.get(k)?.push(r) ?? by.set(k, [r])
  }
  return [...by.entries()]
}

/**
 * Carrying a meeting to the dock.
 *
 * Pointer events rather than the HTML5 drag API, which the WebView this ships
 * inside never fires for page elements. Which tile is underneath is asked of
 * the document, so the dock and the list know nothing about each other.
 */
function useDrag(onDrop: (recording: number, group: number) => void) {
  const [held, setHeld] = useState<{
    id: number
    title: string
    x: number
    y: number
    from: number
    over: number | null
    moved: boolean
  } | null>(null)
  const live = useRef(held)
  live.current = held
  const lifted = held !== null

  useEffect(() => {
    if (!lifted) return
    const move = (e: PointerEvent) =>
      setHeld((d) => {
        if (!d) return d
        const moved =
          d.moved || Math.hypot(e.clientX - d.x, e.clientY - d.y) > 5
        const tile = moved
          ? document
              .elementFromPoint(e.clientX, e.clientY)
              ?.closest("[data-project]")
          : null
        return {
          ...d,
          x: e.clientX,
          y: e.clientY,
          moved,
          over: tile ? Number(tile.getAttribute("data-project")) : null,
        }
      })
    const drop = () => {
      const what = live.current
      setHeld(null)
      if (what?.moved && what.over !== null && what.over !== what.from)
        onDrop(what.id, what.over)
    }
    window.addEventListener("pointermove", move)
    window.addEventListener("pointerup", drop)
    return () => {
      window.removeEventListener("pointermove", move)
      window.removeEventListener("pointerup", drop)
    }
  }, [lifted, onDrop])

  return {
    over: held?.moved ? held.over : null,
    /** A card is in the air — the dock rises to say it can take it. */
    lifted: !!held?.moved,
    lift: (r: Recording, e: React.PointerEvent) =>
      e.button === 0 &&
      setHeld({
        id: r.id,
        title: r.title,
        x: e.clientX,
        y: e.clientY,
        from: r.group,
        over: null,
        moved: false,
      }),
    card: held?.moved ? (
      <div
        style={{ left: held.x, top: held.y }}
        className="pointer-events-none fixed z-50 -translate-x-1/2 -translate-y-1/2 -rotate-2 rounded-lg border border-line bg-raised px-3 py-1.5 text-[11.5px] shadow-[0_16px_40px_rgba(0,0,0,0.6)]"
      >
        {held.title}
      </div>
    ) : null,
  }
}

function Row({
  recording,
  groups,
  on,
  lit,
  binned,
  onLight,
  onOpen,
  onLift,
  onFile,
  onDelete,
  onRestore,
}: {
  recording: Recording
  groups: Group[]
  on: boolean
  lit: boolean
  binned: boolean
  onLight: (id: number | null) => void
  onOpen: () => void
  onLift: (e: React.PointerEvent) => void
  onFile: (group: number) => void
  onDelete: () => void
  onRestore: () => void
}) {
  const busy = recording.status !== "done" && recording.status !== "failed"
  const filed = groups.find((g) => g.id === recording.group)
  const Kind = recording.kind === "note" ? Mic : FileAudio

  return (
    <MeetingPeek recording={recording} onOpen={onOpen} disabled={on || binned}>
      <div
        onPointerDown={(e) =>
          !busy &&
          !binned &&
          !(e.target as HTMLElement).closest("[popover], [data-row-action]") &&
          onLift(e)
        }
        data-rec={recording.id}
        onMouseEnter={() => onLight(recording.id)}
        onMouseLeave={() => onLight(null)}
        // Reserved action width keeps the title still as controls reveal.
        className={`meeting-row group grid grid-cols-[minmax(0,1fr)_56px] border-b border-line/40 transition-[grid-template-columns,background-color] duration-[220ms] ease-[cubic-bezier(.22,1,.36,1)] ${
          binned ? "" : ""
        } ${on ? "bg-raised/60" : lit ? "bg-raised/35" : "hover:bg-raised/25"}`}
      >
        <button
          onClick={onOpen}
          disabled={false}
          className="block w-full px-4 py-2.5 text-left disabled:cursor-default"
        >
          <span className="flex items-start gap-2">
            <Kind
              size={11}
              className="mt-[3px] shrink-0"
              style={{
                color: filed ? colourOf(filed.name, filed.colour) : undefined,
              }}
            />
            <span
              className={`text-[12px] leading-snug ${on ? "text-text" : "text-soft"}`}
            >
              {recording.title}
            </span>
          </span>
          <span className="mt-1 flex items-center gap-1.5 truncate pl-[19px] text-[9px] text-faint">
            <span className="tabular-nums">{clockOf(recording.started)}</span>
            <span>·</span>
            <span className="tabular-nums">{length(recording.duration)}</span>
            {busy && (
              <span className="text-accent">
                ·{" "}
                {{
                  done: "готово",
                  failed: "помилка",
                  queued: "у черзі",
                  transcribing: "розшифрування",
                  summarising: "підсумок",
                }[recording.status] ?? recording.status}
              </span>
            )}
            {recording.speakers?.length ? (
              <>
                <span>·</span>
                <span className="truncate">
                  {recording.speakers.join(", ")}
                </span>
              </>
            ) : null}
          </span>
        </button>

        <span className="overflow-hidden" data-row-action>
          {binned && (
            <button
              className="ui-icon"
              title="Відновити"
              aria-label="Відновити"
              onClick={onRestore}
            >
              <Import size={14} />
            </button>
          )}
          {!binned && (
            <span className="flex h-full items-center gap-px pr-2 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
              <Filing recording={recording} groups={groups} onFile={onFile} />
              <Tool onClick={onDelete} title="У кошик" danger>
                <Trash2 size={12} />
              </Tool>
            </span>
          )}
        </span>
      </div>
    </MeetingPeek>
  )
}

/**
 * Which project this meeting belongs to.
 *
 * This button used to toggle between "no project" and groups[0] — the first
 * project in the list, whichever that happened to be — so every meeting filed
 * from the list landed in the same one no matter what anybody wanted. Dragging
 * onto a tile is still the quick way; this is the way that works without aiming.
 *
 * A native popover, so it lives in the top layer, closes on Escape or a click
 * outside, and is positioned by CSS against the button rather than by measuring
 * anything.
 */
function Filing({
  recording,
  groups,
  onFile,
}: {
  recording: Recording
  groups: Group[]
  onFile: (group: number) => void
}) {
  const id = `file-${recording.id}`
  const shut = (e: React.MouseEvent) =>
    (e.currentTarget.closest("[popover]") as HTMLElement | null)?.hidePopover()

  return (
    <>
      <button
        popoverTarget={id}
        style={{ anchorName: `--${id}` } as React.CSSProperties}
        title="У проєкт"
        className="grid size-[22px] place-items-center rounded text-faint transition-colors hover:bg-surface hover:text-text"
      >
        <FolderInput size={12} />
      </button>
      <div
        popover="auto"
        id={id}
        style={
          {
            positionAnchor: `--${id}`,
            positionArea: "bottom span-left",
            positionTryFallbacks: "flip-block",
            marginTop: "4px",
          } as React.CSSProperties
        }
        className="w-[200px] rounded-xl border border-line bg-raised/95 p-1 shadow-[0_20px_50px_-16px_rgba(0,0,0,0.8)] backdrop-blur-xl [&:popover-open]:animate-[rise_.16s_cubic-bezier(.22,1,.36,1)]"
      >
        {groups.map((g) => (
          <button
            key={g.id}
            onClick={(e) => {
              shut(e)
              onFile(g.id)
            }}
            className={`flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-[12px] transition-colors hover:bg-surface ${
              recording.group === g.id ? "text-text" : "text-soft"
            }`}
          >
            <span
              className="size-2.5 shrink-0 rounded-[3px]"
              style={{ background: colourOf(g.name, g.colour) }}
            />
            <span className="min-w-0 flex-1 truncate">{g.name}</span>
            {recording.group === g.id && (
              <Check size={12} className="shrink-0 text-faint" />
            )}
          </button>
        ))}
        {groups.length > 0 && recording.group > 0 && (
          <>
            <span className="my-1 block h-px bg-line" />
            <button
              onClick={(e) => {
                shut(e)
                onFile(0)
              }}
              className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-[12px] text-soft transition-colors hover:bg-surface hover:text-text"
            >
              <span className="size-2.5 shrink-0 rounded-[3px] border border-line" />
              Поза проєктами
            </button>
          </>
        )}
        {groups.length === 0 && (
          <p className="px-2.5 py-2 text-[11.5px] leading-snug text-faint">
            Проєктів ще немає. Створіть перший плиткою «+» у доку внизу.
          </p>
        )}
      </div>
    </>
  )
}

const clockOf = (iso: string) =>
  new Date(iso).toLocaleTimeString("uk", { hour: "2-digit", minute: "2-digit" })

function Tool({
  onClick,
  title,
  danger,
  children,
}: {
  onClick: () => void
  title: string
  danger?: boolean
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      className={`grid size-[22px] place-items-center rounded text-faint transition-colors hover:bg-surface ${
        danger ? "hover:text-warn" : "hover:text-text"
      }`}
    >
      {children}
    </button>
  )
}

function NameProject({
  value,
  onChange,
  onDone,
}: {
  value: string
  onChange: (v: string) => void
  onDone: (name: string) => void
}) {
  return (
    <div className="absolute inset-x-0 bottom-[calc(var(--spacing-dock)+1.25rem)] z-40 flex justify-center">
      <input
        autoFocus
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onDone(value.trim())
          if (e.key === "Escape") onDone("")
        }}
        onBlur={() => onDone("")}
        placeholder="Назва проєкту"
        className="w-56 rounded-lg border border-accent/60 bg-raised px-3 py-2 text-[12px] outline-none placeholder:text-faint"
      />
    </div>
  )
}

function Empty() {
  return (
    <div className="grid h-full place-content-center gap-2 px-8 text-center text-[11.5px] leading-relaxed text-faint">
      <p>Оберіть запис ліворуч, щоб прочитати його.</p>
      <p>Проєкт унизу — щоб побачити, як він стоїть.</p>
    </div>
  )
}

/** The file picker is the host's job rather than the page's. */
async function pickFile(): Promise<string | null> {
  try {
    const chosen = await Dialogs.OpenFile({
      Title: "Додати запис",
      CanChooseFiles: true,
      Filters: [
        {
          DisplayName: "Аудіо та відео",
          Pattern: "*.wav;*.m4a;*.mp3;*.mp4;*.mov;*.webm;*.aac;*.flac;*.ogg",
        },
      ],
    })
    return typeof chosen === "string" && chosen ? chosen : null
  } catch {
    // No bridge (design mode) or the person cancelled. Neither is an error.
    return null
  }
}
