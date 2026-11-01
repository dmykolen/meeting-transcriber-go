import { Check, FolderInput } from "lucide-react"
import type { Group, Recording } from "../api"
import { colourOf } from "../colours"
import { t } from "../i18n"

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
export default function Filing({
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
        title={t("У проєкт")}
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
              {t("Поза проєктами")}
            </button>
          </>
        )}
        {groups.length === 0 && (
          <p className="px-2.5 py-2 text-[11.5px] leading-snug text-faint">
            {t("Проєктів ще немає. Створіть перший плиткою «+» у доку внизу.")}
          </p>
        )}
      </div>
    </>
  )
}
