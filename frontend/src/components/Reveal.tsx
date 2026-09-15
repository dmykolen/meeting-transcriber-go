import { useEffect, useRef, useState, type ReactNode } from "react"
import { createPortal } from "react-dom"
import { AnimatePresence, motion } from "motion/react"
export default function Reveal({
  label,
  children,
  className = "",
  hover = true,
}: {
  label: ReactNode
  children: ReactNode
  className?: string
  hover?: boolean
}) {
  const [open, setOpen] = useState(false),
    [pinned, setPinned] = useState(false),
    [box, setBox] = useState({ left: 0, top: 0 })
  const root = useRef<HTMLDivElement>(null),
    popup = useRef<HTMLDivElement>(null),
    timer = useRef(0),
    restoring = useRef(false),
    blockedUntil = useRef(0)
  const show = () => {
    clearTimeout(timer.current)
    if (Date.now() < blockedUntil.current) return
    const r = root.current?.getBoundingClientRect()
    if (r)
      setBox({
        left: Math.min(r.left, window.innerWidth - 470),
        top: Math.min(r.bottom + 6, window.innerHeight - 245),
      })
    setOpen(true)
  }
  const leave = () => {
    clearTimeout(timer.current)
    if (!pinned) timer.current = window.setTimeout(() => setOpen(false), 190)
  }
  useEffect(() => {
    if (!open) return
    const down = (e: PointerEvent) => {
      if (
        !root.current?.contains(e.target as Node) &&
        !popup.current?.contains(e.target as Node)
      ) {
        clearTimeout(timer.current)
        blockedUntil.current = Date.now() + 400
        setOpen(false)
        setPinned(false)
      }
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopImmediatePropagation()
        clearTimeout(timer.current)
        blockedUntil.current = Date.now() + 400
        setOpen(false)
        setPinned(false)
        restoring.current = true
        root.current?.querySelector("button")?.focus()
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
  useEffect(() => () => clearTimeout(timer.current), [])
  return (
    <div
      ref={root}
      className={`reveal-trigger ${className}`}
      onPointerEnter={() =>
        hover && (timer.current = window.setTimeout(show, 110))
      }
      onPointerLeave={leave}
    >
      <button
        className="ui-chip"
        aria-expanded={open}
        onFocus={() => hover && !restoring.current && show()}
        onClick={() => {
          clearTimeout(timer.current)
          if (pinned) {
            blockedUntil.current = Date.now() + 400
            setOpen(false)
            setPinned(false)
          } else {
            blockedUntil.current = 0
            show()
            setPinned(true)
          }
        }}
      >
        {label}
      </button>
      {createPortal(
        <AnimatePresence>
          {open && (
            <motion.div
              ref={popup}
              className={`reveal-surface ${className}`}
              style={{ left: Math.max(8, box.left), top: box.top }}
              initial={{ opacity: 0, y: -7, scale: 0.985 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -5, scale: 0.99 }}
              transition={{ duration: 0.19, ease: [0.22, 1, 0.36, 1] }}
              onClick={(e) => {
                if ((e.target as HTMLElement).closest("[data-close-reveal]")) {
                  setOpen(false)
                  setPinned(false)
                }
              }}
              onPointerEnter={() => clearTimeout(timer.current)}
              onPointerLeave={leave}
            >
              {children}
            </motion.div>
          )}
        </AnimatePresence>,
        document.body,
      )}
    </div>
  )
}
