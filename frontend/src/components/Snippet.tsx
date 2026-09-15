import { useEffect, useRef, useState } from "react"
import { Pause, Play } from "lucide-react"
import { clock } from "../api"
import type { Source } from "../api"

/**
 * One saved voice sample, played back.
 *
 * The app claims it knows what somebody sounds like, and until now that claim
 * was 512 numbers nobody could examine. This is the evidence: the longest thing
 * that person said in the meeting the sample was taken from. If it turns out to
 * be somebody else, that is worth finding out by ear rather than by wondering
 * why the wrong name keeps appearing.
 *
 * Nothing is preloaded — a person with ten samples would otherwise open ten
 * connections to the audio route on every visit to Settings.
 */
export default function Snippet({ source, colour }: { source: Source; colour: string }) {
  const audio = useRef<HTMLAudioElement | null>(null)
  const [playing, setPlaying] = useState(false)

  // Leaving Settings mid-sample should stop the sound, not leave it playing
  // from a screen nobody is looking at.
  useEffect(() => () => audio.current?.pause(), [])

  const heard = source.finish > source.start
  const play = () => {
    if (playing) {
      audio.current?.pause()
      return
    }
    if (!audio.current) {
      const el = new Audio(`/audio/${encodeURIComponent(source.audio)}`)
      el.onplay = () => setPlaying(true)
      el.onpause = () => setPlaying(false)
      el.onended = () => setPlaying(false)
      // Stop where the turn stops rather than running on into the next
      // speaker, which would make the sample sound like the wrong person.
      el.ontimeupdate = () => heard && el.currentTime >= source.finish && el.pause()
      el.onloadedmetadata = () => (el.currentTime = source.start)
      audio.current = el
    }
    const el = audio.current
    if (el.readyState > 0 && (el.currentTime < source.start || el.currentTime >= source.finish)) {
      el.currentTime = source.start
    }
    el.play().catch(() => setPlaying(false))
  }

  return (
    <button
      onClick={play}
      disabled={!source.audio}
      className="group/s flex w-full items-center gap-2.5 rounded-lg py-1 pl-1 pr-2 text-left transition-colors hover:bg-surface disabled:cursor-default disabled:opacity-40"
    >
      <span
        style={{ color: playing ? colour : undefined }}
        className="flex size-5 shrink-0 items-center justify-center rounded-full border border-line/70 text-faint transition-colors group-hover/s:border-line group-hover/s:text-text"
      >
        {playing ? <Pause size={9} fill="currentColor" /> : <Play size={9} fill="currentColor" className="ml-px" />}
      </span>
      <span className="min-w-0 flex-1 truncate text-[11.5px] text-soft">{source.title}</span>
      {heard && (
        <span className="shrink-0 text-[10.5px] tabular-nums text-faint">
          {clock(source.start)}
          <span className="mx-1 text-faint/50">–</span>
          {clock(source.finish)}
        </span>
      )}
    </button>
  )
}
