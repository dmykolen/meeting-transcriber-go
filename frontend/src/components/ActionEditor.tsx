import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { Meetings, why, type Action } from "../api"
export default function ActionEditor({
  recording,
  index,
  action,
  onClose,
  onSaved,
}: {
  recording: number
  index: number
  action?: Action
  onClose: () => void
  onSaved: () => void
}) {
  const [value, setValue] = useState<Action>(
      action ?? { task: "", owner: "", due: "", done: false },
    ),
    [error, setError] = useState(""),
    [saving, setSaving] = useState(false)
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    ref.current?.showModal()
  }, [])
  return createPortal(
    <dialog
      ref={ref}
      className="edit-dialog"
      onCancel={onClose}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault()
          setSaving(true)
          try {
            await Meetings.EditAction(recording, index, value as never)
            onSaved()
            onClose()
          } catch (e) {
            setError(why(e))
          } finally {
            setSaving(false)
          }
        }}
      >
        <header>
          <h2>{index < 0 ? "Нова домовленість" : "Домовленість"}</h2>
          <button
            type="button"
            className="ui-icon"
            aria-label="Закрити"
            onClick={onClose}
          >
            <X size={16} />
          </button>
        </header>
        <label>
          Що потрібно зробити
          <textarea
            autoFocus
            required
            value={value.task}
            onChange={(e) => setValue({ ...value, task: e.target.value })}
          />
        </label>
        <div className="field-pair">
          <label>
            Відповідальний
            <input
              value={value.owner}
              onChange={(e) => setValue({ ...value, owner: e.target.value })}
              placeholder="Ім’я"
            />
          </label>
          <label>
            Строк
            <input
              value={value.due}
              onChange={(e) => setValue({ ...value, due: e.target.value })}
              placeholder="Наприклад, 12 вересня"
            />
          </label>
        </div>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <footer>
          <button type="button" className="ui-chip" onClick={onClose}>
            Скасувати
          </button>
          <button
            className="ui-primary"
            disabled={saving || !value.task.trim()}
          >
            {saving ? "Зберігаю…" : "Зберегти"}
          </button>
        </footer>
      </form>
    </dialog>,
    document.body,
  )
}
