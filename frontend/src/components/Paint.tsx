import { Pipette } from "lucide-react"
import { SWATCHES, hex } from "../colours"

/**
 * Giving something a colour.
 *
 * There were three of these — one for a person in the settings, one for a
 * project in the settings, one on the project page — and all three offered the
 * same nine swatches and nothing else. Nine is what the app can promise to keep
 * apart from each other and from its own warning and success colours; it is not
 * a limit anybody asked for. So the swatches stay as the quick answer and the
 * system colour panel sits beside them for any other answer at all.
 *
 * The panel is the platform's: an <input type="color"> in a WebView opens the
 * macOS colour picker, with its wheel, its sliders, its eyedropper and its
 * saved palettes. Nothing here reimplements any of that.
 */
export default function Paint({
  colour,
  derived,
  chosen,
  onPick,
}: {
  /** What is painted right now. */
  colour: string
  /** What the name alone would give it, for the reset. */
  derived: string
  /** Whether somebody has actually chosen, rather than inherited. */
  chosen: boolean
  /** A colour, or "" to go back to the one the name gives. */
  onPick: (colour: string) => void
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {SWATCHES.map((swatch) => (
        <button
          key={swatch}
          onClick={() => onPick(swatch)}
          aria-label={swatch}
          style={{ background: swatch }}
          className={`size-4 rounded-full transition-transform hover:scale-125 ${
            colour === swatch ? "ring-2 ring-text ring-offset-2 ring-offset-surface" : ""
          }`}
        />
      ))}

      {/* Any colour at all. The input is the whole control — it is laid over
          the swatch so the swatch is what you press, and the system panel is
          what opens. */}
      <label
        title="Будь-який інший колір"
        className="relative ml-0.5 grid size-4 cursor-pointer place-items-center rounded-full border border-line text-faint transition-colors hover:border-soft hover:text-soft"
        style={{ background: chosen && !SWATCHES.includes(colour) ? colour : undefined }}
      >
        {!(chosen && !SWATCHES.includes(colour)) && <Pipette size={9} />}
        <input
          type="color"
          value={hex(colour)}
          onChange={(e) => onPick(e.target.value)}
          className="absolute inset-0 cursor-pointer opacity-0"
        />
      </label>

      {chosen && (
        <button
          onClick={() => onPick("")}
          className="ml-1 inline-flex items-center gap-1 rounded px-1.5 py-1 text-[10px] text-faint transition-colors hover:bg-raised hover:text-text"
        >
          <span className="size-2 rounded-full" style={{ background: derived }} />
          Скинути
        </button>
      )}
    </div>
  )
}
