import { useEffect, useRef, useState, type ReactNode } from "react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import {
  Circle,
  EarOff,
  Loader,
  Pause,
  Play,
  Square,
  TriangleAlert,
} from "lucide-react"
import { Meetings, clock, recording, type Listening } from "../api"
import { locale, t, tr } from "../i18n"

/** How long a pause for a private conversation lasts, in minutes. */
const pauses = [5, 15, 60]

const out = [0.22, 1, 0.36, 1] as const
// The rail's own spring, so the drawer moves like the rest of the rail.
const spring = { type: "spring", stiffness: 480, damping: 38 } as const

// The drawer is pulled out of the button: a beam leaves the button, the capsule
// unrolls from its edge, and the controls settle in one after another.
const beam = {
  hidden: { scaleX: 0, opacity: 0, transition: { duration: 0.12 } },
  shown: { scaleX: 1, opacity: 1, transition: { duration: 0.18, ease: out } },
}
const row = {
  hidden: { opacity: 0, x: -10, filter: "blur(6px)", transition: { duration: 0.12 } },
  shown: {
    opacity: 1,
    x: 0,
    filter: "blur(0px)",
    transition: { duration: 0.34, ease: out },
    // A leftover filter would soften the text in WebKit.
    transitionEnd: { filter: "none" },
  },
}

/**
 * The always-on part, made visible.
 *
 * This app's whole proposition is that it is listening when you are not
 * thinking about it, and a claim like that has to be checkable at a glance —
 * otherwise the honest reaction is to not trust it and press Record anyway.
 * So the state has a permanent home at the foot of the rail, and pressing it
 * starts or stops a recording by hand. Pointing at it, or reaching it from the
 * keyboard, pulls out a drawer with the rest: Record with its shortcut, and a
 * pause for a conversation that is nobody's business.
 */
export default function Ear() {
  const [state, setState] = useState<Listening | null>(null)
  const [open, setOpen] = useState(false)
  // The control the glowing plate sits under.
  const [lit, setLit] = useState<number | null>(null)
  const still = useReducedMotion()
  const button = useRef<HTMLButtonElement>(null)
  const closing = useRef(0)
  // After Escape or a choice, focus returning to the button does not reopen it.
  const dismissed = useRef(false)

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
  const paused = state.phase === "paused" && !!state.until
  const busy = recording(state)
  const left = paused ? Math.max(0, (Date.parse(state.until!) - Date.now()) / 1000) : 0
  // The button's own action: Record or Stop, or ending a pause.
  const press = () =>
    (paused ? Meetings.PauseFor(0) : Meetings.Record()).catch(() => {})
  const show = () => {
    window.clearTimeout(closing.current)
    if (!look.pressable || open) return
    setLit(null)
    setOpen(true)
  }
  const choose = (act: () => unknown) => {
    act()
    dismissed.current = true
    setOpen(false)
    button.current?.focus()
  }
  // Without motion the drawer is simply there, already full width.
  const drawer = {
    hidden: {
      width: still ? "auto" : 0,
      opacity: 0,
      transition: { duration: 0.18, ease: [0.4, 0, 1, 1] as const },
    },
    shown: {
      width: "auto",
      opacity: 1,
      transition: { ...spring, staggerChildren: 0.045, delayChildren: 0.08 },
    },
  }
  // One control of the drawer; the plate glides under whichever is pointed at.
  const control = (key: number, act: () => unknown, children: ReactNode) => (
    <motion.button
      key={key}
      variants={row}
      className="ear-item"
      onPointerEnter={() => setLit(key)}
      onFocus={() => setLit(key)}
      onClick={() => choose(act)}
    >
      {lit === key && (
        <motion.span layoutId="ear-lit" className="ear-lit" transition={spring} />
      )}
      {children}
    </motion.button>
  )

  return (
    <div
      className="relative mb-2 mt-auto w-full"
      onPointerEnter={show}
      onPointerLeave={() => {
        window.clearTimeout(closing.current)
        closing.current = window.setTimeout(() => setOpen(false), 200)
      }}
      onFocus={() => !dismissed.current && show()}
      onBlur={(e) => {
        if (e.currentTarget.contains(e.relatedTarget)) return
        dismissed.current = false
        setOpen(false)
      }}
      onKeyDown={(e) => {
        if (e.key !== "Escape" || !open) return
        dismissed.current = true
        setOpen(false)
        button.current?.focus()
      }}
    >
      <button
        ref={button}
        onClick={press}
        disabled={!look.pressable}
        title={look.title}
        className={`ear group flex w-full flex-col items-center gap-1 rounded-xl px-1 py-2.5 transition-colors enabled:hover:bg-raised disabled:cursor-default ${open && look.pressable ? "ear-open" : ""}`}
      >
        <span className="relative flex size-[18px] items-center justify-center">
          {look.halo && (
            <span
              className={`breathe absolute inset-0 rounded-full ${look.halo}`}
            />
          )}
          {paused && (
            // What is left of the pause, as a share of an hour.
            <svg viewBox="0 0 30 30" className="ear-ring" aria-hidden>
              <circle cx="15" cy="15" r="13" />
              <circle
                cx="15"
                cy="15"
                r="13"
                pathLength={1}
                strokeDasharray={1}
                strokeDashoffset={1 - Math.min(1, left / 3600)}
              />
            </svg>
          )}
          <look.Icon
            size={16}
            strokeWidth={1.9}
            className={`ear-icon relative z-10 ${look.tint} ${look.spin ? "animate-spin" : ""}`}
            fill={look.filled ? "currentColor" : "none"}
          />
        </span>
        <span className={`text-[10px] font-medium tabular-nums ${look.tint}`}>
          {look.label}
        </span>
      </button>
      <AnimatePresence>
        {open && look.pressable && (
          <motion.div
            className="ear-menu"
            initial="hidden"
            animate="shown"
            exit="hidden"
          >
            <motion.span className="ear-beam" variants={beam} />
            <motion.div
              className="ear-drawer"
              variants={drawer}
              onPointerMove={(e) => {
                const box = e.currentTarget.getBoundingClientRect()
                e.currentTarget.style.setProperty("--spot-x", `${e.clientX - box.left}px`)
                e.currentTarget.style.setProperty("--spot-y", `${e.clientY - box.top}px`)
              }}
              onPointerLeave={() => setLit(null)}
            >
              <div className="ear-row">
                {control(
                  0,
                  press,
                  <>
                    {paused ? (
                      <Play size={13} className="text-accent" />
                    ) : busy ? (
                      <Square size={11} fill="currentColor" className="text-warn" />
                    ) : (
                      <span className="ear-dot" />
                    )}
                    <span>
                      {paused
                        ? t("Слухати знову")
                        : busy
                          ? t("Зупинити запис")
                          : t("Записати зараз")}
                    </span>
                    {!paused && <kbd className="ear-key">⌘R</kbd>}
                  </>,
                )}
                <motion.span variants={row} className="ear-divider" />
                <motion.small variants={row} id="ear-pause" className="ear-caption">
                  {busy ? t("Зберегти запис і не записувати") : t("Не записувати")}
                </motion.small>
                <div role="group" aria-labelledby="ear-pause" className="flex gap-0.5">
                  {pauses.map((m) =>
                    control(
                      m,
                      () => Meetings.PauseFor(m).catch(() => {}),
                      <>
                        <Arc minutes={m} />
                        <span>
                          {m < 60 ? t("{m} хв", { m }) : t("{h} год", { h: m / 60 })}
                        </span>
                      </>,
                    ),
                  )}
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

/** A pause's length as a share of an hour, drawn as an arc that fills in. */
function Arc({ minutes }: { minutes: number }) {
  return (
    <svg viewBox="0 0 16 16" className="ear-arc" aria-hidden>
      <circle cx="8" cy="8" r="6" />
      <motion.circle
        cx="8"
        cy="8"
        r="6"
        variants={{
          hidden: { pathLength: 0 },
          shown: {
            pathLength: minutes / 60,
            transition: { delay: 0.22, duration: 0.7, ease: out },
          },
        }}
      />
    </svg>
  )
}

/** One place that turns a phase into what a person sees. */
function describe(s: Listening) {
  if (s.phase === "paused" && s.until) {
    const until = new Date(s.until)
    return {
      Icon: EarOff,
      filled: false,
      tint: "text-soft",
      halo: "",
      spin: false,
      pressable: true,
      label: clock(Math.max(0, (until.getTime() - Date.now()) / 1000)),
      title: t("Нічого не записується до {time}. Натисніть, щоб слухати знову.", {
        time: until.toLocaleTimeString(locale(), {
          hour: "2-digit",
          minute: "2-digit",
        }),
      }),
    }
  }
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
        title:
          s.kind === "note"
            ? t("Триває запис нотатки — ⌘R зупинити")
            : t("Триває запис зустрічі — ⌘R зупинити"),
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
        title: t("Тиша вже {n} с — можливо, зустріч завершилась", { n: s.quiet }),
      }
    case "held":
      return {
        Icon: Pause,
        filled: true,
        tint: "text-soft",
        halo: "",
        spin: false,
        pressable: true,
        label: clock(s.elapsed),
        title: t("Запис на паузі: сказане зараз не записується — ⌘R зупинити"),
      }
    case "listening":
      return {
        Icon: Circle,
        filled: false,
        tint: "text-faint group-hover:text-soft",
        halo: "",
        spin: false,
        pressable: true,
        label: t("Запис"),
        title: s.system
          ? t("Слухаю. Почати запис — ⌘R")
          : t("Слухаю лише вас: звук співрозмовників не захоплюється"),
      }
    case "opening":
      return {
        Icon: Loader,
        filled: false,
        tint: "text-faint",
        halo: "",
        spin: true,
        pressable: false,
        label: t("Запуск"),
        title: t("Відкриваю мікрофон. macOS може запитати дозвіл."),
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
        label: t("Вимкнено"),
        title: t("Слухання вимкнено. Увімкніть його в параметрах."),
      }
    default:
      return {
        Icon: TriangleAlert,
        filled: false,
        tint: "text-warn",
        halo: "",
        spin: false,
        pressable: false,
        label: t("Помилка"),
        title: s.problem ? tr(s.problem) : t("Не вдалося відкрити мікрофон"),
      }
  }
}
