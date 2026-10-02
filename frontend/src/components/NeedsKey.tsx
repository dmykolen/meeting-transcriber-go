import { useEffect, useState } from "react"
import { KeyRound } from "lucide-react"
import { Meetings } from "../api"
import { t } from "../i18n"

/**
 * Summaries, the to-do list and Ask all need an AI; transcription and speakers
 * never do. Without one those screens used to sit there looking broken, which
 * is the worst of both worlds — say so instead.
 *
 * Returns null while the answer is unknown and when there is a key, so callers
 * can drop it in without a condition of their own.
 */
export default function NeedsKey({ what }: { what: string }) {
  const [missing, setMissing] = useState(false)

  useEffect(() => {
    Meetings.Summaries()
      .then((on) => setMissing(!on))
      .catch(() => {})
  }, [])

  if (!missing) return null
  return (
    <div className="flex items-start gap-2.5 rounded-panel border border-line/60 bg-surface/50 px-4 py-3">
      <KeyRound size={14} className="mt-0.5 shrink-0 text-faint" />
      <p className="text-[12.5px] leading-relaxed text-soft">
        {t(
          "{what}: оберіть AI у параметрах. Розшифровка й розпізнавання учасників працюють локально.",
          { what },
        )}
      </p>
    </div>
  )
}
