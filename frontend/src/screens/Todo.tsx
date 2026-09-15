import { useCallback, useEffect, useState } from "react"
import { AnimatePresence, motion } from "motion/react"
import { Check, MoreHorizontal, RotateCcw } from "lucide-react"
import Head from "../components/Head"
import ActionEditor from "../components/ActionEditor"
import { Meetings, when, type Outstanding } from "../api"
export default function Todo({ onOpen }: { onOpen: (id: number) => void }) {
  const [items, setItems] = useState<Outstanding[]>([]),
    [done, setDone] = useState(false),
    [editing, setEditing] = useState<Outstanding | null>(null),
    [error, setError] = useState(""),
    [undo, setUndo] = useState<Outstanding | null>(null)
  const load = useCallback(async () => {
    try {
      setItems((await Meetings.Actions(done)) as Outstanding[])
    } catch (e) {
      setError(String(e))
    }
  }, [done])
  useEffect(() => {
    void load()
  }, [load])
  useEffect(() => {
    if (!undo) return
    const t = setTimeout(() => setUndo(null), 8000)
    return () => clearTimeout(t)
  }, [undo])
  return (
    <div className="knowledge-screen">
      <Head title="Домовленості">
        <button
          className="ui-chip"
          aria-pressed={done}
          onClick={() => setDone(!done)}
        >
          {done ? "Сховати виконані" : "Показати виконані"}
        </button>
      </Head>
      <div className="knowledge-results">
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <p className="result-count">
          {items.length} домовленостей · {done ? "усі" : "відкриті"}
        </p>
        {!items.length && (
          <p className="search-empty">
            {done
              ? "Домовленостей ще немає. Додайте їх у документі зустрічі."
              : "Відкритих домовленостей немає."}
          </p>
        )}
        <AnimatePresence initial={false}>
          {items.map((a) => (
            <motion.div
              layout
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, x: 16, height: 0 }}
              transition={{ duration: 0.2 }}
              className={`commitment ${a.done ? "done" : ""}`}
              key={`${a.recording}-${a.index}`}
            >
              <button
                role="checkbox"
                aria-label={`Виконано: ${a.task}`}
                aria-checked={a.done}
                onClick={async () => {
                  try {
                    await Meetings.Tick(a.recording, a.index, !a.done)
                    setUndo(a)
                    await load()
                  } catch (e) {
                    setError(String(e))
                  }
                }}
              >
                {a.done && <Check size={12} />}
              </button>
              <div>
                <p>{a.task}</p>
                <div className="task-links">
                  <button
                    className="commitment-meta"
                    onClick={() => setEditing(a)}
                  >
                    {a.owner || "Призначити"}{" "}
                    <span>· {a.due || "Без строку"}</span>
                  </button>
                  <button onClick={() => onOpen(a.recording)}>{a.title}</button>
                  <small>{when(a.started)}</small>
                </div>
              </div>
              <button
                className="context-action ui-icon"
                aria-label="Редагувати домовленість"
                onClick={() => setEditing(a)}
              >
                <MoreHorizontal size={14} />
              </button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      {editing && (
        <ActionEditor
          recording={editing.recording}
          index={editing.index}
          action={editing}
          onClose={() => setEditing(null)}
          onSaved={() => void load()}
        />
      )}
      {undo && (
        <div className="action-toast" role="status">
          Домовленість оновлено
          <button
            onClick={async () => {
              try {
                await Meetings.Tick(undo.recording, undo.index, undo.done)
                setUndo(null)
                await load()
              } catch (e) {
                setError(String(e))
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
