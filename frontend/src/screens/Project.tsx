import { useEffect, useState } from "react"
import {
  Check,
  CircleHelp,
  Gavel,
  ListChecks,
  RefreshCw,
  Sparkles,
  Trash2,
} from "lucide-react"
import {
  Meetings as Api,
  length,
  many,
  when,
  type Group,
  type Thread,
  type Standing,
} from "../api"
import NotesDeck from "../components/NotesDeck"
import Paint from "../components/Paint"
import { colourOf, picked, tone } from "../colours"

/** Model-maintained project state, with deterministic summary fallback. */
export default function Project({
  id,
  groups,
  onChanged,
  onOpen,
}: {
  id: number
  groups: Group[]
  onChanged: () => void
  onOpen: (recording: number, at?: number) => void
}) {
  const [state, setState] = useState<Standing | null>(null)
  const [rows, setRows] = useState<
    { id: number; title: string; started: string; duration: number }[]
  >([])
  const [name, setName] = useState("")
  const [picking, setPicking] = useState(false)
  const [busy, setBusy] = useState(false)
  const [seed, setSeed] = useState("")
  const [problem, setProblem] = useState("")

  const group = groups.find((g) => g.id === id)
  const colour = group
    ? colourOf(group.name, group.colour)
    : "var(--color-accent)"

  useEffect(() => {
    setState(null)
    Api.Standing(id)
      .then((s) => setState(s as Standing))
      .catch((e) => setProblem(String(e)))
    Api.InGroup(id)
      .then((r) => setRows((r as typeof rows) ?? []))
      .catch((e) => setProblem(String(e)))
    setName(group?.name ?? "")
    setPicking(false)
    // The name follows the project, not the render: retyping it on every poll
    // would fight whoever is editing it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  // Rebuilding puts every meeting back through the model, which is slow. The
  // document counts what it has read as it goes, so the wait is a number
  // climbing rather than a spinner standing for an unknown number of minutes.
  const redo = async () => {
    setBusy(true)
    const read = () => Api.Standing(id).then((s) => setState(s as Standing))
    const watch = setInterval(read, 900)
    try {
      await Api.RebuildProject(id)
    } catch (e) {
      setProblem(String(e))
    } finally {
      clearInterval(watch)
      await read()
      setBusy(false)
    }
  }

  if (!group || !state)
    return <p className="reader-loading">{problem || "Відкриваю проєкт…"}</p>
  const quiet = new Date(Date.now() - 21 * 86400000)

  return (
    <div
      className="@container/proj flex h-full min-h-0 flex-col"
      style={{ ["--tone" as string]: colour }}
    >
      <header className="project-toolbar flex h-bar flex-none items-center gap-2 border-b border-line/70 px-5">
        <button
          onClick={() => setPicking((p) => !p)}
          title="Колір проєкту"
          style={{ background: colour }}
          className="size-3 shrink-0 rounded-[3px] transition-transform hover:scale-125"
        />
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            e.stopPropagation()
            if (e.key === "Enter") e.currentTarget.blur()
            if (e.key === "Escape") {
              setName(group.name)
              e.currentTarget.blur()
            }
          }}
          onBlur={async () => {
            const to = name.trim()
            if (!to || to === group.name) return setName(group.name)
            await Api.RenameGroup(id, to)
            onChanged()
          }}
          spellCheck={false}
          className="-mx-1.5 min-w-0 flex-1 rounded-md bg-transparent px-1.5 py-0.5 font-[inherit] text-[13px] font-medium outline-none transition-colors hover:bg-raised/50 focus:bg-raised"
        />
        <button
          onClick={async () => {
            await Api.DropGroup(id)
            onChanged()
          }}
          title="Розформувати. Наради лишаться, без проєкту."
          className="grid size-6 place-items-center rounded text-faint transition-colors hover:bg-raised hover:text-warn"
        >
          <Trash2 size={13} />
        </button>
        <NotesDeck
          key={id}
          project={id}
          seed={seed}
          onSeedUsed={() => setSeed("")}
        />
        <button
          className="ui-chip"
          disabled={!state.questions?.some((q) => !q.done)}
          onClick={() =>
            setSeed(
              "До наступної зустрічі\n\n" +
                state.questions
                  .filter((q) => !q.done)
                  .map((q) => "• " + q.text)
                  .join("\n"),
            )
          }
        >
          + Порядок денний
        </button>
      </header>
      {problem && (
        <p className="reader-error" role="alert">
          {problem}
        </p>
      )}

      {picking && (
        <div className="flex-none border-b border-line/50 px-5 py-2.5">
          <Paint
            colour={colour}
            derived={tone(group.name)}
            chosen={picked(group.colour)}
            onPick={async (c) => {
              await Api.Paint(id, c)
              onChanged()
            }}
          />
        </div>
      )}

      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)] @min-[600px]/proj:grid-cols-[minmax(0,1fr)_clamp(190px,26cqw,360px)]">
        <div className="min-h-0 overflow-y-auto px-5 pb-8 pt-4">
          <dl className="flex flex-wrap gap-x-7 gap-y-2">
            {[
              ["нарад", String(state.meetings)],
              ["годин", state.hours.toFixed(1)],
              ["людей", String(state.people.length)],
              ["незакрито", String(state.work.filter((w) => !w.done).length)],
              // A short date: the others are two or three characters, and a
              // full "Today 04:41 PM" here breaks the rhythm of the row.
              [
                "востаннє",
                state.last
                  ? new Date(state.last).toLocaleDateString("uk", {
                      day: "numeric",
                      month: "short",
                    })
                  : "—",
              ],
            ].map(([k, v]) => (
              <div key={k}>
                <dt className="text-[9px] uppercase tracking-[0.14em] text-faint">
                  {k}
                </dt>
                <dd className="mt-0.5 text-[17px] font-light tabular-nums">
                  {v}
                </dd>
              </div>
            ))}
          </dl>

          {state.meetings > 0 && (
            <section className="mt-5">
              <h3 className="mb-2 flex items-center gap-2 text-[10px] uppercase tracking-[0.14em] text-faint">
                <Sparkles size={11} /> Де воно стоїть
                {/* The line saying how this paragraph was written is also the
                    button that writes it again. Nothing that says where
                    something came from should need a second control beside it
                    to fix it. */}
                <button
                  onClick={redo}
                  disabled={busy}
                  title="Перечитати всі наради з першої і зібрати документ заново"
                  className="group ml-1 inline-flex items-center gap-1.5 rounded-full border border-dashed border-line px-2 py-0.5 text-[9px] normal-case tracking-normal transition-colors hover:border-[var(--tone)] hover:text-text disabled:cursor-progress"
                >
                  <RefreshCw
                    size={9}
                    className={
                      busy
                        ? "animate-spin"
                        : "transition-transform duration-500 group-hover:-rotate-180"
                    }
                  />
                  {busy
                    ? `перечитую — ${state.folded} з ${state.meetings}`
                    : state.written
                      ? `зібрано моделлю з ${state.meetings} ${many(state.meetings, "наради", "нарад", "нарад")}`
                      : `зібрано з підсумків — зібрати моделлю`}
                </button>
              </h3>
              {/* The largest thing on the page. Somebody opening a project wants
                  a sentence they can repeat to their manager, not a metric grid. */}
              {state.status ? (
                <p
                  className={`max-w-[70ch] text-[14.5px] font-light leading-[1.75] text-soft transition-opacity duration-500 ${
                    busy ? "opacity-40" : ""
                  }`}
                >
                  {state.status}
                </p>
              ) : (
                busy && (
                  <div className="max-w-[70ch] space-y-2 pt-0.5">
                    {[100, 96, 58].map((w) => (
                      <div
                        key={w}
                        className="thinking h-[13px] rounded-sm"
                        style={{ width: `${w}%` }}
                      />
                    ))}
                  </div>
                )
              )}
            </section>
          )}

          <Block title="Робота" Icon={ListChecks} note={`${state.work.length}`}>
            {state.work.map((w) => (
              <Task
                key={w.text}
                line={w}
                onOpen={async () =>
                  onOpen(w.from, (await Api.Moment(w.from, w.text)) as number)
                }
                onTick={async () => {
                  // The document owns the line once the model keeps one; before
                  // that the tick belongs to the meeting it came from.
                  if (w.item > 0) await Api.TickItem(id, w.item, !w.done)
                  else if (w.index >= 0)
                    await Api.Tick(w.from, w.index, !w.done)
                  Api.Standing(id).then((s) => setState(s as Standing))
                }}
                onEdit={
                  w.item > 0
                    ? async (text) => {
                        await Api.PinItem(id, w.item, text, w.owner, w.due)
                        Api.Standing(id).then((s) => setState(s as Standing))
                      }
                    : undefined
                }
              />
            ))}
          </Block>

          <Block
            title="Вирішено"
            Icon={Gavel}
            note={`${state.decisions.length}`}
          >
            {state.decisions.map((d) => (
              <Said
                key={d.text}
                line={d}
                onOpen={async () =>
                  onOpen(d.from, (await Api.Moment(d.from, d.text)) as number)
                }
              />
            ))}
          </Block>

          <Block
            title="Досі без відповіді"
            Icon={CircleHelp}
            note={`${state.questions.length}`}
          >
            {state.questions.map((q) => (
              <Said
                key={q.text}
                line={q}
                onOpen={async () =>
                  onOpen(q.from, (await Api.Moment(q.from, q.text)) as number)
                }
              />
            ))}
          </Block>

          <Block title="Наради" Icon={Sparkles} note={`${rows.length}`}>
            {rows.map((r) => (
              <button
                key={r.id}
                onClick={() => onOpen(r.id)}
                className="flex w-full items-baseline gap-3 rounded-md px-1.5 py-1.5 text-left transition-colors hover:bg-raised/50"
              >
                <span className="w-20 shrink-0 text-[10px] tabular-nums text-faint">
                  {when(r.started)}
                </span>
                <span className="min-w-0 flex-1 truncate text-[12px] text-soft">
                  {r.title}
                </span>
                <span className="shrink-0 text-[10px] tabular-nums text-faint">
                  {length(r.duration)}
                </span>
              </button>
            ))}
          </Block>
        </div>

        <aside className="min-h-0 overflow-y-auto border-t border-line/70 px-4 pb-8 @min-[600px]/proj:border-l @min-[600px]/proj:border-t-0">
          <h3 className="sticky top-0 z-10 flex items-baseline justify-between bg-surface/90 py-3 text-[10.5px] backdrop-blur">
            Хто тут був
            <span className="text-[9px] tabular-nums text-faint">
              {state.people.length}
            </span>
          </h3>
          {state.people.map((p) => {
            const share = p.seconds / (state.people[0]?.seconds || 1)
            const gone = new Date(p.last) < quiet
            return (
              <div
                key={p.name}
                className={`border-b border-line/40 py-2.5 ${gone ? "opacity-50" : ""}`}
              >
                <div className="flex items-baseline justify-between gap-2">
                  <span className="flex min-w-0 items-center gap-1.5">
                    <span
                      className="size-[5px] shrink-0 rounded-full"
                      style={{ background: colourOf(p.name) }}
                    />
                    <span className="truncate text-[11px]">{p.name}</span>
                  </span>
                  <span
                    className="shrink-0 text-[10px] tabular-nums"
                    style={{ color: colourOf(p.name) }}
                  >
                    {(p.seconds / 3600).toFixed(1)} год
                  </span>
                </div>
                <span className="mt-1.5 block h-[3px] rounded-full bg-raised">
                  <span
                    className="block h-full rounded-full"
                    style={{
                      width: `${share * 100}%`,
                      background: colourOf(p.name),
                    }}
                  />
                </span>
                <span className="mt-1 block text-[9px] text-faint">
                  {p.meetings} {many(p.meetings, "нарада", "наради", "нарад")}
                  {gone && ` · востаннє ${when(p.last)}`}
                </span>
              </div>
            )
          })}
        </aside>
      </div>
    </div>
  )
}

function Block({
  title,
  Icon,
  note,
  children,
}: {
  title: string
  Icon: typeof Check
  note: string
  children: React.ReactNode
}) {
  if (!Array.isArray(children) || children.length === 0) return null
  return (
    <section className="mt-6 border-t border-line/50 pt-3.5">
      <h3 className="mb-1.5 flex items-center gap-2 text-[10px] uppercase tracking-[0.14em] text-faint">
        <Icon size={11} /> {title}
        <span className="ml-auto tabular-nums normal-case tracking-normal">
          {note}
        </span>
      </h3>
      {children}
    </section>
  )
}

/** How many meetings said it — the honest signal that nobody is doing it. */
function Times({ n }: { n: number }) {
  if (n < 2) return null
  return <span className="shrink-0 text-[9.5px] text-faint">казано ×{n}</span>
}

function Task({
  line,
  onTick,
  onOpen,
  onEdit,
}: {
  line: Thread
  onTick: () => void
  onOpen: () => void
  /** Present once the line lives in the kept document, where editing pins it. */
  onEdit?: (text: string) => void
}) {
  const [text, setText] = useState(line.text)
  useEffect(() => setText(line.text), [line.text])

  return (
    <div className="group flex items-baseline gap-2.5 rounded-md px-1.5 py-1.5 transition-colors hover:bg-raised/50">
      <button
        onClick={onTick}
        className={`mt-[3px] grid size-3.5 shrink-0 place-items-center rounded border transition-colors ${
          line.done
            ? "border-[--tone] bg-[--tone] text-ink"
            : "border-line hover:border-soft"
        }`}
      >
        {line.done && <Check size={9} strokeWidth={3} />}
      </button>
      <span className="min-w-0 flex-1">
        {/* Editable in place once the document owns it. Typing here pins the
            line: from then on the model may close it but never reword it. */}
        {onEdit ? (
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              e.stopPropagation()
              if (e.key === "Enter") e.currentTarget.blur()
              if (e.key === "Escape") {
                setText(line.text)
                e.currentTarget.blur()
              }
            }}
            onBlur={() => {
              const to = text.trim()
              if (!to || to === line.text) return setText(line.text)
              onEdit(to)
            }}
            spellCheck={false}
            className={`-mx-1 w-[calc(100%+0.5rem)] rounded bg-transparent px-1 font-[inherit] text-[12.5px] leading-snug outline-none transition-colors hover:bg-raised/60 focus:bg-raised ${
              line.done ? "text-faint line-through" : "text-text"
            }`}
          />
        ) : (
          <button onClick={onOpen} className="block w-full text-left">
            <span
              className={`text-[12.5px] leading-snug ${line.done ? "text-faint line-through" : ""}`}
            >
              {line.text}
            </span>
          </button>
        )}
        {(line.owner || line.due || line.pinned) && (
          <span className="mt-0.5 block text-[9.5px] text-faint">
            {[line.owner, line.due, line.pinned ? "ваше формулювання" : ""]
              .filter(Boolean)
              .join(" · ")}
          </span>
        )}
      </span>
      <button
        onClick={onOpen}
        title="Відкрити нараду, де це сказали"
        className="shrink-0"
      >
        <Times n={line.times} />
      </button>
    </div>
  )
}

function Said({ line, onOpen }: { line: Thread; onOpen: () => void }) {
  const gone = line.state === "overturned" || line.state === "answered"
  return (
    <button
      onClick={onOpen}
      className="flex w-full items-baseline gap-2.5 rounded-md px-1.5 py-1.5 text-left transition-colors hover:bg-raised/50"
    >
      <span
        className={`mt-[6px] size-[4px] shrink-0 rounded-full bg-[--tone] ${gone ? "opacity-40" : ""}`}
      />
      <span className="min-w-0 flex-1 text-[12.5px] leading-snug">
        <span
          className={
            gone ? "text-faint line-through decoration-1" : "text-soft"
          }
        >
          {line.text}
        </span>
        {/* What replaced it. A project's history of reversals is the most
            expensive thing in it to reconstruct, so it is never thrown away. */}
        {line.by && (
          <span className="mt-0.5 block text-[12px] text-soft no-underline">
            → {line.by}
          </span>
        )}
      </span>
      <Times n={line.times} />
      <span className="shrink-0 text-[9.5px] tabular-nums text-faint">
        {when(line.when)}
      </span>
    </button>
  )
}
