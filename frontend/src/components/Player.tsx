import {
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type Ref,
} from "react"
import { Pause, Play, RotateCcw, AlertCircle } from "lucide-react"
import { Meetings, clock } from "../api"
import { t } from "../i18n"
/** go moves the audio. Playing from there is the caller's separate wish — the
 * one thing it never does is leave the audio where it was, which is how the
 * highlight and the sound came to disagree. */
export type Controls = { go: (seconds: number, play?: boolean) => void }

/** The envelope has 300 points; the wave has room for this many bars. Drawn
 * one bar per point, they came out zero pixels wide and the wave was empty. */
const BARS = 56

/**
 * The meeting's audio, in the corner. The play button never moves: the rest
 * opens to its left while the pointer or focus is on it and while it plays.
 * It used to grow from the button's side, so the button slid out from under
 * the pointer that came to press it.
 *
 * Position and loudness reach the styles as --p and --level, set once a frame
 * while playing, so the wave and the ring move smoothly without React
 * re-rendering anything.
 */
export default function Player({
  id,
  file,
  onTime,
  ref,
}: {
  id: number
  file: string
  onTime: (s: number) => void
  ref: Ref<Controls>
}) {
  const audio = useRef<HTMLAudioElement>(null),
    root = useRef<HTMLDivElement>(null),
    leave = useRef(0),
    [playing, setPlaying] = useState(false),
    [at, setAt] = useState(0),
    [duration, setDuration] = useState(0),
    [rate, setRate] = useState(1),
    [error, setError] = useState(false),
    [shape, setShape] = useState<number[]>([]),
    [pointer, setPointer] = useState(false),
    [focus, setFocus] = useState(false),
    /** The point under the pointer on the wave: its share of the length, and
        how far it is from the player's left edge, for the time shown above. */
    [aim, setAim] = useState<{ share: number; x: number } | null>(null),
    [pulse, setPulse] = useState(0)
  const open = !error && (pointer || focus || playing)
  useEffect(() => {
    Meetings.Waveform(id)
      .then((s) => setShape(s ?? []))
      .catch(() => {})
    return () => {
      clearTimeout(leave.current)
      void Meetings.Playing(false).catch(() => {})
    }
  }, [id])
  const bars = useMemo(
    () =>
      Array.from({ length: BARS }, (_, i) => {
        if (!shape.length) return 0.16
        const from = Math.floor((i * shape.length) / BARS)
        return Math.max(
          ...shape.slice(from, Math.max(from + 1, Math.floor(((i + 1) * shape.length) / BARS))),
        )
      }),
    [shape],
  )
  const paint = (seconds: number) =>
    root.current?.style.setProperty(
      "--p",
      String(audio.current?.duration ? seconds / audio.current.duration : 0),
    )
  const play = () => audio.current?.play().catch(() => setError(true))
  const toggle = () => (audio.current?.paused ? play() : audio.current?.pause())
  useImperativeHandle(ref, () => ({
    go(seconds, start = true) {
      if (!audio.current) return
      audio.current.currentTime = seconds
      setAt(seconds)
      paint(seconds)
      onTime(seconds)
      if (start) void play()
    },
  }))
  useEffect(() => {
    if (audio.current) audio.current.playbackRate = rate
  }, [rate])
  // While it plays: the position every frame, and the loudness under it, eased,
  // for the glow around the button.
  useEffect(() => {
    if (!playing) return
    const still = matchMedia("(prefers-reduced-motion: reduce)").matches
    let frame = 0,
      level = 0
    const tick = () => {
      const a = audio.current
      if (a?.duration) {
        paint(a.currentTime)
        if (!still && shape.length) {
          const loud = shape[Math.min(shape.length - 1, Math.floor((a.currentTime / a.duration) * shape.length))]
          level += (loud - level) * 0.16
          root.current?.style.setProperty("--level", level.toFixed(3))
        }
      }
      frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(frame)
      root.current?.style.setProperty("--level", "0")
    }
  }, [playing, shape])
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (
        e.code === "Space" &&
        !(e.target as HTMLElement).closest(
          'button,input,textarea,select,[role="dialog"],dialog',
        )
      ) {
        e.preventDefault()
        toggle()
      }
    }
    window.addEventListener("keydown", key)
    return () => window.removeEventListener("keydown", key)
  }, [])
  const stopped = () => {
    setPlaying(false)
    void Meetings.Playing(false)
  }
  return (
    <div
      ref={root}
      className={`floating-audio ${open ? "open" : ""} ${playing ? "playing" : ""} ${error ? "audio-error" : ""}`}
      onPointerEnter={() => {
        clearTimeout(leave.current)
        setPointer(true)
      }}
      onPointerLeave={() => {
        leave.current = window.setTimeout(() => setPointer(false), 650)
      }}
      onFocus={() => setFocus(true)}
      onBlur={(e) =>
        !root.current?.contains(e.relatedTarget as Node) && setFocus(false)
      }
    >
      <audio
        ref={audio}
        src={`/audio/${encodeURIComponent(file)}`}
        preload="metadata"
        onPlay={() => {
          setPlaying(true)
          setPulse((n) => n + 1)
          void Meetings.Playing(true)
        }}
        onPause={stopped}
        onEnded={stopped}
        onError={() => {
          setError(true)
          stopped()
        }}
        onLoadedMetadata={(e) => {
          setDuration(
            Number.isFinite(e.currentTarget.duration)
              ? e.currentTarget.duration
              : 0,
          )
          paint(e.currentTarget.currentTime)
          setError(false)
        }}
        onTimeUpdate={(e) => {
          setAt(e.currentTarget.currentTime)
          paint(e.currentTarget.currentTime)
          onTime(e.currentTarget.currentTime)
        }}
      />
      <div className="audio-more" inert={!open}>
        <div>
          <button
            className="ui-icon"
            aria-label={t("Назад на 10 секунд")}
            title={t("Назад на 10 секунд")}
            onClick={() => {
              if (audio.current)
                audio.current.currentTime = Math.max(0, audio.current.currentTime - 10)
            }}
          >
            <RotateCcw size={13} />
          </button>
          <button
            className="audio-rate"
            aria-label={t("Швидкість відтворення")}
            title={t("Швидкість відтворення")}
            onClick={() => setRate(rate === 1 ? 1.5 : rate === 1.5 ? 2 : 1)}
          >
            {rate}×
          </button>
          <div
            className="audio-wave"
            onPointerMove={(e) => {
              const box = e.currentTarget.getBoundingClientRect()
              setAim({
                share: Math.min(1, Math.max(0, (e.clientX - box.left) / box.width)),
                x: e.clientX - (root.current?.getBoundingClientRect().left ?? 0),
              })
            }}
            onPointerLeave={() => setAim(null)}
          >
            {[false, true].map((lit) => (
              <div key={String(lit)} className={lit ? "lit" : ""} aria-hidden>
                {bars.map((v, i) => (
                  <i
                    key={i}
                    style={{ "--h": Math.max(v, 0.1), "--i": i } as CSSProperties}
                  />
                ))}
              </div>
            ))}
            <span className="audio-head" aria-hidden />
            {aim && duration > 0 && (
              <span
                className="audio-aim"
                style={{ "--a": aim.share } as CSSProperties}
                aria-hidden
              />
            )}
            <input
              aria-label={t("Позиція відтворення")}
              type="range"
              min="0"
              max={duration || 1}
              step="0.1"
              value={at}
              onChange={(e) => {
                if (audio.current) audio.current.currentTime = +e.target.value
              }}
            />
          </div>
        </div>
      </div>
      {/* Outside the part that opens, which clips whatever leaves its box. */}
      {open && aim && duration > 0 && (
        <b className="audio-tip" style={{ left: aim.x }} aria-hidden>
          {clock(aim.share * duration)}
        </b>
      )}
      <span className="audio-clock">
        {error ? (
          t("Аудіо недоступне")
        ) : (
          <>
            {clock(at)}
            {duration > 0 && <small> / {clock(duration)}</small>}
          </>
        )}
      </span>
      <button
        className="audio-play"
        aria-label={
          error
            ? t("Повторити завантаження аудіо")
            : playing
              ? t("Пауза")
              : t("Відтворити")
        }
        onClick={() => {
          if (error) {
            setError(false)
            audio.current?.load()
          } else toggle()
        }}
      >
        <svg className="audio-ring" viewBox="0 0 36 36" aria-hidden>
          <circle cx="18" cy="18" r="16.5" pathLength={1} />
        </svg>
        {pulse > 0 && <span key={pulse} className="audio-pulse" aria-hidden />}
        {error ? (
          <AlertCircle size={15} />
        ) : (
          <>
            <Play size={14} fill="currentColor" className="audio-icon-play" />
            <Pause size={14} fill="currentColor" className="audio-icon-pause" />
          </>
        )}
      </button>
    </div>
  )
}
