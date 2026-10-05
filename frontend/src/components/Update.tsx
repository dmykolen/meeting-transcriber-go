import { useEffect, useState } from "react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { Browser } from "@wailsio/runtime"
import { Meetings, why, type UpdateState } from "../api"
import { t, tr } from "../i18n"

const dismissedKey = "mt.update.dismissed"

const remembered = () => {
  try {
    return localStorage.getItem(dismissedKey)
  } catch {
    return null
  }
}

/** The app's update state, polled; and the version that was put off. */
export function useUpdate() {
  const [state, setState] = useState<UpdateState | null>(null)
  const [later, setLater] = useState(remembered)

  useEffect(() => {
    let alive = true
    const read = () =>
      Meetings.Update()
        .then((s) => alive && setState(s as UpdateState))
        .catch(() => {})
    read()
    // A check runs on Go's own clock every hour; this only notices its result.
    const timer = setInterval(read, 5000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  const version = state?.available?.version ?? null
  return {
    state,
    open: !!version && later !== version,
    postpone: () => {
      setLater(version)
      try {
        localStorage.setItem(dismissedKey, version ?? "")
      } catch {
        // Putting it off just lasts until the next launch.
      }
    },
    reopen: () => setLater(null),
  }
}

/** The version at the foot of the rail; it lights up when there is a newer one. */
export function Version({
  update,
}: {
  update: ReturnType<typeof useUpdate>
}) {
  const { state, open, reopen } = update
  if (!state) return null
  const newer = state.available
  return (
    <button
      onClick={reopen}
      disabled={!newer || open}
      title={
        newer
          ? t("Доступна версія {version}", { version: newer.version })
          : t("Версія {version}", { version: state.current })
      }
      className="relative mb-2 mt-1 w-full truncate text-center text-[9.5px] tabular-nums text-faint enabled:text-accent enabled:hover:text-text"
    >
      v{state.current}
      {newer && (
        <span className="absolute right-1.5 top-0.5 size-1.5 rounded-full bg-accent" />
      )}
    </button>
  )
}

/** Release notes are Markdown with a heading and bullets; shown as plain lines. */
function Notes({ text }: { text: string }) {
  const lines = text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && !/^the app isn.t notarized/i.test(l))
  return (
    <div className="max-h-36 space-y-1 overflow-y-auto pr-1 text-[11.5px] leading-snug text-soft">
      {lines.map((l, i) =>
        l.startsWith("#") ? (
          <p key={i} className="pt-1 text-[10px] font-semibold uppercase tracking-wider text-faint">
            {l.replace(/^#+\s*/, "")}
          </p>
        ) : (
          <p key={i}>{l.replace(/^[-*]\s*/, "").replace(/`/g, "")}</p>
        ),
      )}
    </div>
  )
}

/**
 * The offer to update: a card in the corner, not a dialog over the work. It
 * says what is new, downloads behind the window, and restarts only when asked.
 */
export function UpdateCard({ update }: { update: ReturnType<typeof useUpdate> }) {
  const { state, open, postpone } = update
  const [said, setSaid] = useState("")
  const still = useReducedMotion()
  const next = state?.available
  const button =
    "rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[11.5px] text-soft transition-colors hover:border-accent/40 hover:text-text"
  const go = (job: Promise<unknown>) => job.catch((e) => setSaid(why(e)))

  return (
    <AnimatePresence>
      {open && state && next && (
        <motion.aside
          role="status"
          aria-label={t("Доступна версія {version}", { version: next.version })}
          initial={{ opacity: 0, y: still ? 0 : 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: still ? 0 : 12 }}
          transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
          className="no-drag fixed bottom-4 right-4 z-50 w-[320px] rounded-panel border border-line/70 bg-surface p-3.5 shadow-2xl"
        >
          <h2 className="text-[13px] font-medium">
            {t("Доступна версія {version}", { version: next.version })}
          </h2>
          <p className="mb-2 mt-0.5 text-[11px] text-faint">
            {t("Зараз {version}", { version: state.current })}
          </p>
          {next.notes && <Notes text={next.notes} />}

          {state.stage === "downloading" && (
            <div className="mt-3 space-y-1">
              <p className="text-[11.5px] text-soft">
                {t("Завантажую оновлення")} ·{" "}
                <span className="tabular-nums">{Math.round(state.fraction * 100)}%</span>
              </p>
              <div className="h-1 overflow-hidden rounded-full bg-raised">
                <div
                  className="h-full rounded-full bg-accent transition-[width] duration-500"
                  style={{ width: `${state.fraction * 100}%` }}
                />
              </div>
            </div>
          )}
          {state.stage === "ready" && (
            <p className="mt-3 text-[11.5px] text-soft">
              {t("Оновлення завантажено й перевірено. Застосунок перезапуститься.")}
            </p>
          )}
          {(said || state.problem) && (
            <p className="mt-3 text-[11.5px] text-warn">{tr(said || state.problem)}</p>
          )}

          <div className="mt-3 flex items-center gap-2">
            {state.stage === "ready" ? (
              <button className={button} onClick={() => go(Meetings.RestartToUpdate())}>
                {t("Перезапустити")}
              </button>
            ) : state.installable ? (
              <button
                className={button}
                disabled={state.stage === "downloading"}
                onClick={() => go(Meetings.InstallUpdate())}
              >
                {t("Оновити")}
              </button>
            ) : (
              <button className={button} onClick={() => go(Browser.OpenURL(next.page))}>
                {t("Сторінка випуску")}
              </button>
            )}
            <button className={`${button} border-transparent bg-transparent`} onClick={postpone}>
              {t("Пізніше")}
            </button>
          </div>
        </motion.aside>
      )}
    </AnimatePresence>
  )
}
