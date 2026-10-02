import type en from "./en"

// The interface is written in Ukrainian, and the Ukrainian text is the key:
// a component reads as the person sees it, and English is one lookup away.
// A string missing from en.ts does not compile. The same Ukrainian with two
// meanings carries one after "||" ("Скасувати||undo"), which Ukrainian drops.

export type Lang = "uk" | "en"
export type Key = keyof typeof en

let current: Lang = "uk"
// Loaded the first time English is chosen; a Ukrainian session never fetches it.
let english: Record<string, string> = {}
const listeners = new Set<() => void>()

export const lang = () => current

/** The locale dates, times and numbers are written in. */
export const locale = () => (current === "en" ? "en-GB" : "uk-UA")

export const onLang = (redraw: () => void) => {
  listeners.add(redraw)
  return () => void listeners.delete(redraw)
}

export async function setLang(chosen: string) {
  if (chosen === "en") english = (await import("./en")).default
  current = chosen === "en" ? "en" : "uk"
  document.documentElement.lang = current
  listeners.forEach((redraw) => redraw())
}

/** Interface text in the chosen language; {name} takes a value from vars. */
export function t(text: Key, vars?: Record<string, string | number>): string {
  const said = current === "en" ? english[text] : text.split("||")[0]
  return vars
    ? said.replace(/\{(\w+)\}/g, (_, name: string) => String(vars[name]))
    : said
}

/**
 * A message the Go side wrote in Ukrainian, in the chosen language when it is
 * known. "Known part: detail" keeps the detail, which is usually a system error.
 */
export function tr(text: string): string {
  if (current === "uk") return text
  if (text in english) return english[text]
  const cut = text.indexOf(": ")
  return cut > 0 && text.slice(0, cut) in english
    ? english[text.slice(0, cut)] + text.slice(cut)
    : text
}
