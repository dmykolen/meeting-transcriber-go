/** Stable, saturated defaults for people and projects. Explicit user colours
 * remain untouched; collisions within a meeting use the next free hue. */
const WHEEL = [
  [60, 78], // gold
  [245, 58], // deep blue
  [170, 78], // mint
  [275, 58], // deep periwinkle
  [130, 78], // green
  [60, 58], // bronze
  [245, 78], // blue
  [130, 58], // deep green
  [275, 78], // periwinkle
  [170, 58], // deep mint
]

const CHROMA = 0.23

/**
 * tone is the colour a name always has.
 *
 * FNV-1a because it spreads short strings — "Marta" and "Olha" differ in one
 * letter and must not land on one hue, which a naive sum of character codes
 * does constantly.
 */
export function tone(name: string): string {
  let hash = 0x811c9dc5
  for (let i = 0; i < name.length; i++) {
    hash ^= name.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  const [hue, light] = WHEEL[hash % WHEEL.length]
  return `oklch(${light}% ${CHROMA} ${hue})`
}

/** The wheel as pickable swatches, for the one place somebody overrules it. */
export const SWATCHES = WHEEL.map(
  ([hue, light]) => `oklch(${light}% ${CHROMA} ${hue})`,
)

/** The hue behind a swatch, which is the part the eye actually separates. */
const HUES = WHEEL.map(([hue]) => hue)

/**
 * What to actually paint: somebody's own choice, or the colour their name gives
 * them.
 *
 * A choice used to count only while it was a colour from the wheel above, so
 * that changing the wheel repainted everything already painted. That was the
 * right answer while the wheel was the only way to choose; it is the wrong one
 * now that any colour can be picked from the system panel, because it would
 * quietly throw away exactly the choices somebody went to the trouble of
 * making. A colour that was chosen is kept, whatever it is.
 */
export const colourOf = (name: string, chosen?: string) =>
  picked(chosen) ? chosen!.trim() : tone(name)

/** Whether somebody chose this, rather than inheriting it from their name. */
export const picked = (colour?: string) => !!colour && colour.trim() !== ""

/**
 * Any CSS colour as the #rrggbb an <input type="color"> insists on.
 *
 * The wheel is written in oklch and a chosen colour comes back as hex, so the
 * two have to meet somewhere. The browser already knows how to convert between
 * every colour space it supports; asking it is shorter and more correct than
 * carrying the conversion around.
 */
export function hex(colour: string): string {
  if (/^#[0-9a-f]{6}$/i.test(colour.trim())) return colour.trim()
  const canvas = document.createElement("canvas")
  canvas.width = canvas.height = 1
  const context = canvas.getContext("2d", { willReadFrequently: true })
  if (!context) return "#808080"
  context.fillStyle = colour
  context.fillRect(0, 0, 1, 1)
  const data = context.getImageData(0, 0, 1, 1).data
  return (
    "#" +
    Array.from(data.slice(0, 3), (v) => v.toString(16).padStart(2, "0")).join(
      "",
    )
  )
}

/**
 * A wash of the same colour, for a background that has to sit under text.
 * color-mix keeps it in the same hue rather than adding a second, dimmer
 * palette to maintain.
 */
export const wash = (colour: string, percent = 14) =>
  `color-mix(in oklch, ${colour} ${percent}%, transparent)`

/**
 * Text that stays readable on a tile of any of these.
 *
 * The dock painted every initial in --color-ink, which is right on a light
 * tile and unreadable on a dark one; half the wheel is now dark. Safari 26
 * works out which of black or white wins, so nothing here has to guess.
 */
export const on = (colour: string) => `contrast-color(${colour})`

/**
 * Colours for a group of names that will be looked at together.
 *
 * The wheel gives a name the same colour everywhere, which is what makes a
 * person recognisable across meetings — but it cannot promise that two people
 * in the *same* meeting get different ones, and with ten slots and six speakers
 * that promise is the one that matters. Measured on a real meeting: Marta and
 * the owner both landed on hue 130, one at 78% lightness and one at 58%, and in
 * a seven-pixel bar those are the same green.
 *
 * So: everyone keeps the colour their name gives them, and anyone who would
 * collide with somebody already placed is walked to the next free slot. The
 * common case — one or two people in view — is untouched, and a crowded meeting
 * can no longer show one colour twice. Order decides who keeps their own, so
 * pass the names in the order they are drawn.
 */
export function palette(
  names: string[],
  chosen?: Map<string, string>,
): Map<string, string> {
  const out = new Map<string, string>()
  const usedHue = new Set<number>()
  const usedSlot = new Set<number>()

  const take = (i: number) => {
    usedHue.add(HUES[i])
    usedSlot.add(i)
    return SWATCHES[i]
  }

  for (const name of names) {
    const own = chosen?.get(name)
    // A colour somebody picked by hand is theirs whatever else is on screen.
    if (picked(own)) {
      const at = SWATCHES.indexOf(own!.trim())
      out.set(name, at < 0 ? own!.trim() : take(at))
      continue
    }
    const wanted = SWATCHES.indexOf(tone(name))
    // Separate on hue, not on the whole colour. Two slots can share a hue at
    // two lightnesses, and 78% against 58% of one green is a difference nobody
    // reads in a seven-pixel bar — which is exactly how the owner and Marta
    // ended up as the same lime in a six-person meeting.
    if (!usedHue.has(HUES[wanted])) {
      out.set(name, take(wanted))
      continue
    }
    const free = SWATCHES.findIndex((_, i) => !usedHue.has(HUES[i]))
    // Only once every hue is spoken for does the second weight earn its place.
    const spare = SWATCHES.findIndex((_, i) => !usedSlot.has(i))
    out.set(name, take(free >= 0 ? free : spare >= 0 ? spare : wanted))
  }
  return out
}
