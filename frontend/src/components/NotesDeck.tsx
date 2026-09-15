import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { AnimatePresence, motion } from "motion/react"
import { Check, NotebookPen, Plus, Trash2, X } from "lucide-react"
import { Meetings, type Sticky } from "../api"
const colors = ["lime", "blue", "pink", "orange"]
export default function NotesDeck({
  recording = 0,
  project = 0,
  legacy = "",
  seed = "",
  onSeedUsed,
}: {
  recording?: number
  project?: number
  legacy?: string
  seed?: string
  onSeedUsed?: () => void
}) {
  const [notes, setNotes] = useState<Sticky[]>([]),
    [open, setOpen] = useState(false),
    [active, setActive] = useState(0),
    [editing, setEditing] = useState<number | null>(null),
    [status, setStatus] = useState(""),
    [removed, setRemoved] = useState<Sticky | null>(null),
    [point, setPoint] = useState({ left: 0, top: 0 })
  const trigger = useRef<HTMLButtonElement>(null),
    panel = useRef<HTMLDivElement>(null),
    leaving = useRef(0),
    saveTimer = useRef(0),
    restoring = useRef(false),
    queue = useRef(Promise.resolve()),
    latest = useRef(notes)
  latest.current = notes
  useEffect(() => {
    let alive = true
    Meetings.Notes(recording, project)
      .then((n) => {
        if (alive)
          setNotes([
            ...(legacy
              ? [
                  {
                    id: -recording,
                    recording,
                    project: 0,
                    text: legacy,
                    colour: "orange",
                    at: 0,
                  },
                ]
              : []),
            ...(n as Sticky[]),
          ])
      })
      .catch((e) => setStatus(String(e)))
    return () => {
      alive = false
    }
  }, [recording, project])
  const show = () => {
    clearTimeout(leaving.current)
    const r = trigger.current?.getBoundingClientRect()
    if (r)
      setPoint({
        left: Math.max(8, Math.min(r.left, window.innerWidth - 480)),
        top: Math.min(r.bottom + 8, window.innerHeight - 300),
      })
    setOpen(true)
  }
  const save = (n: Sticky) => {
    setStatus("Зберігаю…")
    queue.current = queue.current
      .catch(() => {})
      .then(async () => {
        if (n.id < 0) await Meetings.SaveNote(recording, n.text)
        else await Meetings.PutNote(n as never)
        setStatus("Збережено")
      })
      .catch((e) => setStatus("Не збережено: " + String(e)))
  }
  const change = (id: number, text: string, colour?: string) => {
    const updated = latest.current.map((n) =>
      n.id === id ? { ...n, text, colour: colour ?? n.colour } : n,
    )
    latest.current = updated
    setNotes(updated)
    clearTimeout(saveTimer.current)
    setStatus("Зберігаю…")
    saveTimer.current = window.setTimeout(
      () => save(updated.find((n) => n.id === id)!),
      400,
    )
  }
  const add = async (text = "") => {
    show()
    try {
      const n = (await Meetings.PutNote({
        id: 0,
        recording,
        project,
        text,
        colour: colors[notes.length % 4],
        at: 0,
      } as never)) as Sticky
      setNotes((ns) => [...ns, n])
      setActive(Math.min(notes.length, 5))
      setEditing(n.id)
      setStatus("Збережено")
    } catch (e) {
      setStatus(String(e))
    }
  }
  useEffect(() => {
    if (seed) {
      void add(seed)
      onSeedUsed?.()
    }
  }, [seed])
  useEffect(() => {
    if (!open) return
    const down = (e: PointerEvent) => {
      if (
        !panel.current?.contains(e.target as Node) &&
        !trigger.current?.contains(e.target as Node)
      ) {
        setOpen(false)
        setEditing(null)
      }
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopImmediatePropagation()
        setOpen(false)
        setEditing(null)
        restoring.current = true
        trigger.current?.focus()
        restoring.current = false
      }
    }
    document.addEventListener("pointerdown", down)
    window.addEventListener("keydown", key, true)
    return () => {
      document.removeEventListener("pointerdown", down)
      window.removeEventListener("keydown", key, true)
    }
  }, [open])
  useEffect(
    () => () => {
      clearTimeout(leaving.current)
      clearTimeout(saveTimer.current)
      const n = latest.current.find((n) => n.id === editing)
      if (n) save(n)
    },
    [editing],
  )
  const selected = notes.find((n) => n.id === editing)
  return (
    <>
      <button
        ref={trigger}
        className="notes-trigger"
        aria-label={`Нотатки · ${notes.length}`}
        aria-expanded={open}
        onPointerEnter={show}
        onPointerLeave={() => {
          if (editing === null)
            leaving.current = window.setTimeout(() => setOpen(false), 240)
        }}
        onFocus={() => !restoring.current && show()}
        onClick={() => {
          show()
          if (!notes.length) void add()
          else setEditing(notes[Math.min(active, notes.length - 1)].id)
        }}
      >
        <span className="mini-deck" aria-hidden>
          {(notes.length
            ? notes.slice(0, 4)
            : colors.slice(0, 3).map((colour) => ({ colour }))
          ).map((n, i) => (
            <i
              key={i}
              className={n.colour}
              style={{ rotate: `${i * 9 - 12}deg`, left: i * 5 }}
            />
          ))}
        </span>
        <span>
          Нотатки <b>{notes.length || "+"}</b>
        </span>
      </button>
      {createPortal(
        <AnimatePresence>
          {open && (
            <motion.div
              ref={panel}
              className="notes-surface"
              style={point}
              initial={{ opacity: 0, y: -6, scale: 0.97 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -4, scale: 0.97 }}
              transition={{ duration: 0.23, ease: [0.22, 1, 0.36, 1] }}
              onPointerEnter={() => clearTimeout(leaving.current)}
              onPointerLeave={() => {
                if (editing === null)
                  leaving.current = window.setTimeout(() => setOpen(false), 240)
              }}
            >
              <header>
                <span>
                  <NotebookPen size={14} /> Ваші нотатки <b>{notes.length}</b>
                </span>
                <button
                  className="ui-icon"
                  aria-label="Додати нотатку"
                  onClick={() => void add()}
                >
                  <Plus size={15} />
                </button>
                <button
                  className="ui-icon"
                  aria-label="Закрити нотатки"
                  onClick={() => {
                    setOpen(false)
                    setEditing(null)
                  }}
                >
                  <X size={15} />
                </button>
              </header>
              {selected ? (
                <div className={`note-editor ${selected.colour}`}>
                  <textarea
                    autoFocus
                    aria-label="Текст нотатки"
                    value={selected.text}
                    placeholder="Запишіть думку…"
                    onChange={(e) => change(selected.id, e.target.value)}
                    onBlur={() => {
                      clearTimeout(saveTimer.current)
                      save(latest.current.find((n) => n.id === selected.id)!)
                    }}
                  />
                  <footer>
                    <div className="note-colors">
                      {colors.map((c) => (
                        <button
                          key={c}
                          className={c}
                          aria-label={`Колір ${c}`}
                          onClick={() => change(selected.id, selected.text, c)}
                        />
                      ))}
                    </div>
                    <button
                      aria-label="Видалити нотатку"
                      onClick={async () => {
                        try {
                          clearTimeout(saveTimer.current)
                          await queue.current
                          if (selected.id < 0)
                            await Meetings.SaveNote(recording, "")
                          else await Meetings.RemoveNote(selected.id)
                          latest.current = latest.current.filter(
                            (n) => n.id !== selected.id,
                          )
                          setNotes(latest.current)
                          setActive(Math.max(0, latest.current.length - 1))
                          setRemoved(selected)
                          setStatus("Нотатку видалено")
                          setEditing(null)
                        } catch (e) {
                          setStatus(String(e))
                        }
                      }}
                    >
                      <Trash2 size={14} />
                    </button>
                    <button onClick={() => setEditing(null)}>
                      <Check size={14} /> До колоди
                    </button>
                  </footer>
                </div>
              ) : (
                <div className="note-fan">
                  {notes.length ? (
                    notes.slice(0, 6).map((n, i) => (
                      <button
                        key={n.id}
                        className={`note-card ${n.colour} ${active === i ? "active" : ""}`}
                        style={{
                          left:
                            i <= active
                              ? i * 38
                              : active * 38 + 181 + (i - active - 1) * 38,
                          zIndex: active === i ? 20 : i,
                          transform: `translateY(${active === i ? -6 : 8}px) rotate(${active === i ? 0 : i % 2 ? 3 : -3}deg)`,
                        }}
                        onPointerEnter={() => setActive(i)}
                        onFocus={() => setActive(i)}
                        onClick={() => setEditing(n.id)}
                      >
                        <span>{n.text || "Нова думка…"}</span>
                        <small>{String(i + 1).padStart(2, "0")}</small>
                      </button>
                    ))
                  ) : (
                    <button className="notes-empty" onClick={() => void add()}>
                      <Plus size={22} />
                      <span>
                        Залиште власну думку
                        <br />
                        <small>до зустрічі або проєкту</small>
                      </span>
                    </button>
                  )}
                </div>
              )}
              {notes.length > 6 && editing === null && (
                <select
                  aria-label="Усі нотатки"
                  value=""
                  onChange={(e) => setEditing(+e.target.value)}
                >
                  <option value="">Усі {notes.length} нотаток…</option>
                  {notes.map((n) => (
                    <option value={n.id} key={n.id}>
                      {n.text.slice(0, 55) || "Нова нотатка"}
                    </option>
                  ))}
                </select>
              )}
              <p className={status.startsWith("Не") ? "error" : "note-status"}>
                {status || "Наведіть, щоб розкласти · натисніть, щоб писати"}
                {status.startsWith("Не збережено") && selected && (
                  <button className="ui-chip" onClick={() => save(selected)}>
                    Повторити збереження
                  </button>
                )}
                {removed && (
                  <button
                    className="ui-chip"
                    onClick={async () => {
                      try {
                        const restored =
                          removed.id < 0
                            ? (await Meetings.SaveNote(recording, removed.text),
                              removed)
                            : ((await Meetings.PutNote({
                                ...removed,
                                id: 0,
                              } as never)) as Sticky)
                        setNotes((ns) => [...ns, restored])
                        setActive(Math.min(notes.length, 5))
                        setRemoved(null)
                        setStatus("Відновлено")
                      } catch (e) {
                        setStatus(String(e))
                      }
                    }}
                  >
                    Скасувати видалення
                  </button>
                )}
              </p>
            </motion.div>
          )}
        </AnimatePresence>,
        document.body,
      )}
    </>
  )
}
