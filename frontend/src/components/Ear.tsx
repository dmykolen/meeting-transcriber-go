import { useEffect, useState } from "react"
import { Circle, EarOff, Loader, Square, TriangleAlert } from "lucide-react"
import { Meetings, clock, type Listening } from "../api"

/**
 * The always-on part, made visible.
 *
 * This app's whole proposition is that it is listening when you are not
 * thinking about it, and a claim like that has to be checkable at a glance —
 * otherwise the honest reaction is to not trust it and press Record anyway.
 * So the state has a permanent home at the foot of the rail, and pressing it
 * starts or stops a recording by hand.
 */
export default function Ear() {
  const [state, setState] = useState<Listening | null>(null)

  useEffect(() => {
    let alive = true
    const read = () =>
      Meetings.Listening()
        .then((s) => alive && setState(s as Listening))
        .catch(() => {})
    read()
    const timer = setInterval(read, 1000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  // ⌘R, because this is the one action worth doing without reaching for a
  // mouse: a meeting has started and nothing is coming out of the speakers.
  useEffect(() => {
    const press = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "r") {
        e.preventDefault()
        Meetings.Record().catch(() => {})
      }
    }
    window.addEventListener("keydown", press)
    return () => window.removeEventListener("keydown", press)
  }, [])

  if (!state) return null
  const look = describe(state)

  return (
    <button
      onClick={() => Meetings.Record().catch(() => {})}
      disabled={!look.pressable}
      title={look.title}
      className="group mb-2 mt-auto flex w-full flex-col items-center gap-1 rounded-xl px-1 py-2.5 transition-colors enabled:hover:bg-raised disabled:cursor-default"
    >
      <span className="relative flex size-[18px] items-center justify-center">
        {look.halo && (
          <span
            className={`breathe absolute inset-0 rounded-full ${look.halo}`}
          />
        )}
        <look.Icon
          size={16}
          strokeWidth={1.9}
          className={`relative z-10 ${look.tint} ${look.spin ? "animate-spin" : ""}`}
          fill={look.filled ? "currentColor" : "none"}
        />
      </span>
      <span className={`text-[10px] font-medium tabular-nums ${look.tint}`}>
        {look.label}
      </span>
    </button>
  )
}

/** One place that turns a phase into what a person sees. */
function describe(s: Listening) {
  switch (s.phase) {
    case "recording":
      return {
        Icon: Square,
        filled: true,
        tint: "text-warn",
        halo: "bg-warn/30",
        spin: false,
        pressable: true,
        label: clock(s.elapsed),
        title: `Триває запис ${s.kind === "note" ? "нотатки" : "зустрічі"} — ⌘R зупинити`,
      }
    case "wrapping up":
      return {
        Icon: Square,
        filled: true,
        tint: "text-warn",
        halo: "",
        spin: false,
        pressable: true,
        label: clock(s.elapsed),
        title: `Тиша вже ${s.quiet} с — можливо, зустріч завершилась`,
      }
    case "listening":
      return {
        Icon: Circle,
        filled: false,
        tint: "text-faint group-hover:text-soft",
        halo: "",
        spin: false,
        pressable: true,
        label: "Запис",
        title: s.system
          ? "Слухаю. Почати запис — ⌘R"
          : "Слухаю лише вас: звук співрозмовників не захоплюється",
      }
    case "opening":
      return {
        Icon: Loader,
        filled: false,
        tint: "text-faint",
        halo: "",
        spin: true,
        pressable: false,
        label: "Запуск",
        title: "Відкриваю мікрофон. macOS може запитати дозвіл.",
      }
    case "paused":
    case "off":
      return {
        Icon: EarOff,
        filled: false,
        tint: "text-faint",
        halo: "",
        spin: false,
        pressable: false,
        label: "Вимкнено",
        title: "Слухання вимкнено. Увімкніть його в параметрах.",
      }
    default:
      return {
        Icon: TriangleAlert,
        filled: false,
        tint: "text-warn",
        halo: "",
        spin: false,
        pressable: false,
        label: "Помилка",
        title: s.problem || "Не вдалося відкрити мікрофон",
      }
  }
}
