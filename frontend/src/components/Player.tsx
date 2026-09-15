import {
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from "react"
import { Pause, Play, RotateCcw, Volume2, AlertCircle } from "lucide-react"
import { Meetings, clock } from "../api"
/** go moves the audio. Playing from there is the caller's separate wish — the
 * one thing it never does is leave the audio where it was, which is how the
 * highlight and the sound came to disagree. */
export type Controls = { go: (seconds: number, play?: boolean) => void }
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
    [playing, setPlaying] = useState(false),
    [at, setAt] = useState(0),
    [duration, setDuration] = useState(0),
    [rate, setRate] = useState(1),
    [error, setError] = useState(false),
    [shape, setShape] = useState<number[]>([])
  useEffect(() => {
    Meetings.Waveform(id)
      .then((s) => setShape(s ?? []))
      .catch(() => {})
    return () => {
      void Meetings.Playing(false).catch(() => {})
    }
  }, [id])
  const play = () => audio.current?.play().catch(() => setError(true))
  const toggle = () => (audio.current?.paused ? play() : audio.current?.pause())
  useImperativeHandle(ref, () => ({
    go(seconds, start = true) {
      if (!audio.current) return
      audio.current.currentTime = seconds
      setAt(seconds)
      onTime(seconds)
      if (start) void play()
    },
  }))
  useEffect(() => {
    if (audio.current) audio.current.playbackRate = rate
  }, [rate])
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
  return (
    <div className={`floating-audio ${error ? "audio-error" : ""}`}>
      <audio
        ref={audio}
        src={`/audio/${encodeURIComponent(file)}`}
        preload="metadata"
        onPlay={() => {
          setPlaying(true)
          void Meetings.Playing(true)
        }}
        onPause={() => {
          setPlaying(false)
          void Meetings.Playing(false)
        }}
        onEnded={() => {
          setPlaying(false)
          void Meetings.Playing(false)
        }}
        onError={() => {
          setError(true)
          setPlaying(false)
          void Meetings.Playing(false)
        }}
        onLoadedMetadata={(e) => {
          setDuration(
            Number.isFinite(e.currentTarget.duration)
              ? e.currentTarget.duration
              : 0,
          )
          setError(false)
        }}
        onTimeUpdate={(e) => {
          setAt(e.currentTarget.currentTime)
          onTime(e.currentTarget.currentTime)
        }}
      />
      <button
        className="audio-play"
        aria-label={
          error
            ? "Повторити завантаження аудіо"
            : playing
              ? "Пауза"
              : "Відтворити"
        }
        onClick={() => {
          if (error) {
            setError(false)
            audio.current?.load()
          } else toggle()
        }}
      >
        {error ? (
          <AlertCircle size={15} />
        ) : playing ? (
          <Pause size={14} fill="currentColor" />
        ) : (
          <Play size={14} fill="currentColor" />
        )}
      </button>
      <span className="audio-clock">
        {error ? "Аудіо недоступне" : clock(at)}
      </span>
      <div className="audio-expanded">
        <button
          className="ui-icon"
          aria-label="Назад на 10 секунд"
          onClick={() => {
            if (audio.current) audio.current.currentTime = Math.max(0, at - 10)
          }}
        >
          <RotateCcw size={13} />
        </button>
        <div className="audio-wave">
          <div aria-hidden>
            {(shape.length ? shape : Array(45).fill(0.18)).map((v, i, a) => (
              <i
                key={i}
                style={{
                  height: Math.max(v * 100, 8) + "%",
                  background:
                    i / a.length < at / (duration || 1)
                      ? "var(--color-accent)"
                      : undefined,
                }}
              />
            ))}
          </div>
          <input
            aria-label="Позиція відтворення"
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
        <span className="audio-clock">{clock(duration)}</span>
        <button
          className="audio-rate"
          aria-label="Швидкість відтворення"
          onClick={() => setRate(rate === 1 ? 1.5 : rate === 1.5 ? 2 : 1)}
        >
          <Volume2 size={12} />
          {rate}×
        </button>
      </div>
    </div>
  )
}
