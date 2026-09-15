import { useEffect, useRef } from "react"
import { AnimatePresence, motion } from "motion/react"
import { Undo2 } from "lucide-react"

/**
 * What replaced "Delete for ever".
 *
 * A confirmation dialog asks somebody to be certain before they have seen the
 * result. Undo lets them find out and change their mind, which is the same
 * safety with one less decision — and it is why deleting here is a single
 * quiet click rather than a coral button that shouts.
 *
 * Ten seconds, then it goes. The recording is still in the bin either way;
 * this is only the fast way back.
 */
export default function Undo({
  what,
  onUndo,
  onGone,
}: {
  what: string | null
  onUndo: () => void
  onGone: () => void
}) {
  const gone = useRef(onGone)
  gone.current = onGone
  useEffect(() => {
    if (!what) return
    const timer = setTimeout(() => gone.current(), 10_000)
    return () => clearTimeout(timer)
  }, [what])

  return (
    <AnimatePresence>
      {what && (
        <motion.div
          initial={{ opacity: 0, y: 12, scale: 0.98 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 8, scale: 0.98 }}
          transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
          className="pointer-events-none fixed inset-x-0 bottom-5 z-50 flex justify-center px-6"
        >
          <div className="pointer-events-auto flex max-w-md items-center gap-3 rounded-full border border-line/70 bg-raised/95 py-2 pl-4 pr-2 shadow-[0_8px_32px_-8px_rgba(0,0,0,0.7)] backdrop-blur-xl">
            <span className="min-w-0 flex-1 truncate text-[12.5px] text-soft">
              У кошику: <span className="text-text">{what}</span>
            </span>
            <button
              onClick={onUndo}
              className="flex shrink-0 items-center gap-1.5 rounded-full px-3 py-1 text-[12px] font-medium text-accent transition-colors hover:bg-accent/12"
            >
              <Undo2 size={13} /> Скасувати
            </button>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
