import { useEffect, useRef, type ReactNode } from "react"
import { createPortal } from "react-dom"
export default function Confirm({
  children,
  onClose,
  title,
}: {
  children: ReactNode
  onClose: () => void
  title: string
}) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const previous = document.activeElement as HTMLElement
    ref.current?.showModal()
    return () => previous?.focus({ preventScroll: true })
  }, [])
  return createPortal(
    <dialog
      ref={ref}
      className="edit-dialog"
      aria-label={title}
      onCancel={onClose}
      onClick={(e) => e.currentTarget === e.target && onClose()}
    >
      {children}
    </dialog>,
    document.body,
  )
}
