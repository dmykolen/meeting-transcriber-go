// The generated bindings, gathered behind one import so that no component
// reaches into a path with a Go module name in it.
import * as Meetings from "../bindings/github.com/dmykolen/meeting-transcriber-go/internal/service/meetings"
import * as Status from "../bindings/github.com/dmykolen/meeting-transcriber-go/internal/service/status"

// Design mode: run the screens on sample data, for working on the interface
// without waiting for a gigabyte of models.
//
// Asked for explicitly with VITE_DESIGN rather than inferred from the absence
// of a Wails bridge — the runtime package defines `window._wails` the moment it
// is imported, host or no host, so that test was always true and this was
// always off. Both conditions are compile-time constants, so a production build
// drops the branch and the sample file with it.
const designing = import.meta.env.DEV && import.meta.env.VITE_DESIGN === "1"

async function design() {
  return (await import("./sample")).sample
}

const stub = <T extends object>(real: T): T =>
  designing
    ? (new Proxy(real, {
        get:
          (_t, key) =>
          async (...args: unknown[]) => {
            const s = await design()
            const fn = (s as Record<string, unknown>)[key as string]
            if (typeof fn !== "function")
              throw new Error(`no sample for ${String(key)}`)
            return (fn as (...a: unknown[]) => unknown)(...args)
          },
      }) as T)
    : real

const meetings = stub(Meetings)
const status = stub(Status)

export { meetings as Meetings, status as Status }

export type Stage = "downloading" | "loading" | "ready" | "broken"

export type SetupState = {
  stage: Stage
  what: string
  fraction: number
  done: number
  total: number
  problem?: string
}

export type MCPState = {
  status: "starting" | "running" | "failed" | "stopped"
  url: string
  command: string
  problem?: string
}

// The always-on recorder's state, straight from Go.
export type Listening = {
  phase:
    | "off"
    | "opening"
    | "listening"
    | "recording"
    | "wrapping up"
    | "paused"
    | "broken"
  kind: "meeting" | "note"
  elapsed: number
  quiet: number
  system: boolean
  problem: string
}

export type Action = { task: string; owner: string; due: string; done: boolean }
export type Chapter = { start: number; title: string; summary: string }

export type Summary = {
  title: string
  overview: string
  chapters: Chapter[] | null
  topics: string[] | null
  decisions: string[] | null
  action_items: Action[] | null
  open_questions: string[] | null
}

export type Recording = {
  id: number
  kind: "meeting" | "note"
  title: string
  started: string
  duration: number
  language: string
  audio: string // file name; empty once the audio has been deleted
  group: number // which group it is filed under, 0 for none
  status: "queued" | "transcribing" | "summarising" | "done" | "failed"
  progress: number
  problem?: string
  summary?: Summary
  speakers?: string[]
  turns: number
  note?: string
}

export type Turn = {
  start: number
  end: number
  speaker?: string
  text: string
}
export type Meeting = Recording & { transcript: Turn[] }
export type Hit = {
  recording: number
  title: string
  start: number
  speaker?: string
  text: string
}
export type Answer = { text: string; sources: Hit[] }

export type Outstanding = {
  recording: number
  title: string
  started: string
  index: number
  task: string
  owner: string
  due: string
  done: boolean
}

// One line of the meeting happening right now.
export type Line = { at: number; who: "you" | "them"; text: string }

export type Voice = {
  speaker: string
  seconds: number
  share: number
  turns: number
  longest: number
  words: number
  pace: number
  questions: number
}

export type Analytics = {
  speech: number
  silence: number
  overlap: number
  words: number
  pace: number
  speakers: Voice[]
  balance: number
  busiest: { at: number; words: number }[] | null
}

// `colour` is empty when nobody has chosen one; colourOf then derives it from
// the name, which is what keeps somebody the same colour everywhere.
export type Person = {
  id: number
  name: string
  samples: number
  meetings: number
  colour: string
  sources: Source[]
}
export type Source = {
  recording: number
  speaker: string
  title: string
  audio: string
  start: number
  finish: number
}
export type Group = { id: number; name: string; count: number; colour: string }

/** One recording as the timeline draws it — no transcript, no summary. */
export type Mark = {
  id: number
  kind: "meeting" | "note"
  started: string
  duration: number
  folder: number
}

/** One thing a project's meetings keep saying, and how often they said it. */
export type Thread = {
  /** Its id in the kept document, or 0 when this came from the mechanical fold. */
  item: number
  state: string
  by: string
  pinned: boolean
  text: string
  owner: string
  due: string
  done: boolean
  times: number
  from: number
  index: number
  when: string
}

export type Face = {
  name: string
  seconds: number
  meetings: number
  last: string
}

export type Standing = {
  meetings: number
  hours: number
  first: string
  last: string
  work: Thread[]
  decisions: Thread[]
  questions: Thread[]
  people: Face[]
  /** One paragraph on where the project stands, when a model has written one. */
  status: string
  written: boolean
  /** How many of these meetings the model has folded in; climbs on a rebuild. */
  folded: number
}

export type Said = {
  recording: number
  title: string
  started: string
  text: string
}
export type Nagging = { text: string; times: number; said: Said[] }

export type Briefing = {
  since: string
  meetings: Recording[]
  minutes: number
  decided: Said[]
  mine: Outstanding[]
  overdue: Outstanding[]
  nagging: Nagging[]
  voices: string[]
  skipped: number
  spared: number
}

export type Density = "compact" | "comfortable"

export type Settings = {
  language: string
  transcriber: "whisper" | "parakeet"
  openaiKey: string
  openaiModel: string
  summarise: "always" | "meetings" | "never"
  keepNotes: boolean
  density: Density
  listening: boolean
  /** Capture the machine's own audio alongside the microphone. */
  system: boolean
  startSpeech: number
  quietEnds: number
  preroll: number
  keepAudioDays: number
  folder: string
}

/** Seconds as a person says them: 4:07, or 1:12:30 for the long ones. */
export function clock(seconds: number): string {
  const s = Math.max(0, Math.round(seconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const rest = s % 60
  const pad = (n: number) => String(n).padStart(2, "0")
  return h > 0 ? `${h}:${pad(m)}:${pad(rest)}` : `${m}:${pad(rest)}`
}

/**
 * Ukrainian counts three ways — 1 нарада, 2 наради, 5 нарад — and the app said
 * "2 нарад" everywhere it counted anything. Intl knows which form a number
 * takes; the words are the only part worth writing down.
 */
const rule = new Intl.PluralRules("uk")
export const many = (
  n: number,
  one: string,
  few: string,
  rest: string,
): string => (({ one, few }) as Record<string, string>)[rule.select(n)] ?? rest

/** "18 minutes", "1 hr 4 min" — a length, not a timestamp. */
export function length(seconds: number): string {
  const m = Math.round(seconds / 60)
  if (m < 1) return "до хвилини"
  if (m < 60) return `${m} хв`
  return `${Math.floor(m / 60)} год ${m % 60} хв`
}

/** Today, Yesterday, or the date. Nobody wants a full timestamp in a list. */
export function when(iso: string): string {
  const at = new Date(iso)
  const now = new Date()
  const day = (d: Date) =>
    new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.round((day(now) - day(at)) / 86_400_000)
  const time = at.toLocaleTimeString("uk-UA", {
    hour: "2-digit",
    minute: "2-digit",
  })
  if (days === 0) return `Сьогодні ${time}`
  if (days === 1) return `Учора ${time}`
  if (days < 7)
    return at.toLocaleDateString("uk-UA", { weekday: "long" }) + ` ${time}`
  return (
    at.toLocaleDateString("uk-UA", { day: "numeric", month: "short" }) +
    ` ${time}`
  )
}

export function bytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ["KB", "MB", "GB"]
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}

export type Sticky = {
  id: number
  recording: number
  project: number
  text: string
  colour: string
  at: number
}
export type KnowledgeHit = {
  key: string
  kind: string
  recording: number
  project: number
  note: number
  title: string
  start: number
  text: string
}
export type KnowledgeAnswer = { text: string; sources: KnowledgeHit[] }
