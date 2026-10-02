import { useEffect, useRef, useState, type ReactNode } from "react"
import { createPortal } from "react-dom"
import { AnimatePresence, motion } from "motion/react"
import { X } from "lucide-react"
import { t } from "../i18n"

let drawerCount = 0
let previousInert = false

export function useNarrow(width: number) {
  const [media] = useState(() => matchMedia(`(max-width: ${width}px)`))
  const [narrow, setNarrow] = useState(media.matches)
  useEffect(() => {
    const m = media
    const change = () => setNarrow(m.matches)
    m.addEventListener("change", change)
    return () => m.removeEventListener("change", change)
  }, [])
  return narrow
}

export default function Drawer({
  open,
  onClose,
  title,
  side = "right",
  children,
}: {
  open: boolean
  onClose: () => void
  title: string
  side?: "left" | "right"
  children: ReactNode
}) {
  const panel = useRef<HTMLDivElement>(null),
    close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    if (!open) return
    const app = document.getElementById("root")
    if (drawerCount++ === 0) previousInert = app?.inert ?? false
    if (app) app.inert = true
    const previous = document.activeElement as HTMLElement
    const timer = setTimeout(
      () => panel.current?.querySelector<HTMLButtonElement>("button")?.focus(),
      60,
    )
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault()
        e.stopImmediatePropagation()
        close.current()
      }
      if (e.key === "Tab") {
        const items = [
          ...panel.current!.querySelectorAll<HTMLElement>(
            'button,input,textarea,select,[tabindex="0"]',
          ),
        ].filter(
          (e) => e.getClientRects().length && !e.hasAttribute("disabled"),
        )
        const first = items[0],
          last = items.at(-1)
        if (
          e.shiftKey &&
          (document.activeElement === first ||
            !panel.current?.contains(document.activeElement))
        ) {
          e.preventDefault()
          last?.focus()
        } else if (
          !e.shiftKey &&
          (document.activeElement === last ||
            !panel.current?.contains(document.activeElement))
        ) {
          e.preventDefault()
          first?.focus()
        }
      }
    }
    window.addEventListener("keydown", key, true)
    return () => {
      clearTimeout(timer)
      window.removeEventListener("keydown", key, true)
      if (--drawerCount === 0 && app) app.inert = previousInert
      previous?.focus({ preventScroll: true })
    }
  }, [open])
  return createPortal(
    <AnimatePresence>
      {open && (
        <div className="drawer-layer">
          <motion.button
            aria-label={t("Закрити панель")}
            tabIndex={-1}
            className="drawer-scrim"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
          />
          <motion.div
            ref={panel}
            role="dialog"
            aria-modal="true"
            aria-label={title}
            className={`drawer-panel ${side}`}
            initial={{ x: side === "left" ? "-100%" : "100%" }}
            animate={{ x: 0 }}
            exit={{ x: side === "left" ? "-100%" : "100%" }}
            transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          >
            <header>
              <strong>{title}</strong>
              <button
                className="ui-icon"
                aria-label={t("Закрити")}
                onClick={onClose}
              >
                <X size={16} />
              </button>
            </header>
            {children}
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
