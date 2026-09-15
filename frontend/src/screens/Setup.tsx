import { motion } from "motion/react"
import { bytes, type SetupState } from "../api"

/**
 * The first run.
 *
 * Half a gigabyte of models is not a spinner. It gets a screen that says what
 * is being fetched, how far it has got, and — the part most downloads leave
 * out — that this happens once.
 */
export default function Setup({ state }: { state: SetupState | null }) {
  const stage = state?.stage ?? "loading"
  const fraction = state?.fraction ?? 0

  return (
    <div className="flex h-full items-center justify-center px-10">
      <div className="w-full max-w-md">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        >
          <Mark stage={stage} />

          <h1 className="mt-7 text-[22px] font-semibold tracking-[-0.01em]">
            {stage === "broken" ? "Something went wrong" : "Getting ready"}
          </h1>

          <p className="mt-2 text-[13px] leading-relaxed text-soft">
            {stage === "broken" ? (
              state?.problem
            ) : stage === "loading" ? (
              "Opening the models. A few seconds."
            ) : (
              <>
                Downloading the models that transcribe and recognise voices. This
                happens once — after today the app starts instantly, and works
                with no internet at all.
              </>
            )}
          </p>

          {stage !== "broken" && (
            <>
              <div className="mt-7 h-1 overflow-hidden rounded-full bg-raised">
                <motion.div
                  className="h-full rounded-full bg-accent"
                  initial={{ width: 0 }}
                  animate={{ width: `${Math.max(fraction * 100, stage === "loading" ? 100 : 2)}%` }}
                  transition={{ ease: "easeOut", duration: 0.4 }}
                />
              </div>

              <div className="mt-3 flex items-baseline justify-between text-[12px]">
                <span className="text-soft">{state?.what}</span>
                {state && state.total > 0 && (
                  <span className="tabular-nums text-faint">
                    {bytes(state.done)} / {bytes(state.total)}
                  </span>
                )}
              </div>
            </>
          )}

          {stage === "broken" && (
            <p className="mt-6 text-[12px] leading-relaxed text-faint">
              Check the connection and reopen the app — it carries on from where
              it stopped rather than starting again.
            </p>
          )}
        </motion.div>
      </div>
    </div>
  )
}

/** A soft pulse while it works, and a still red ring when it cannot. */
function Mark({ stage }: { stage: string }) {
  const broken = stage === "broken"
  return (
    <div className="relative size-14">
      {!broken && <div className="breathe absolute inset-0 rounded-2xl bg-accent/25" />}
      <div
        className={`relative flex size-14 items-center justify-center rounded-2xl border ${
          broken ? "border-warn/40 bg-warn/10" : "border-accent/30 bg-accent-soft/40"
        }`}
      >
        <svg viewBox="0 0 24 24" fill="none" strokeWidth={1.6} strokeLinecap="round" className="size-6">
          {broken ? (
            <path d="M12 8v5m0 3h.01M12 3l9 16H3l9-16Z" className="stroke-warn" />
          ) : (
            <path
              d="M12 4v9m0 0a3 3 0 0 0 3-3V7a3 3 0 0 0-6 0v3a3 3 0 0 0 3 3Zm-6 0a6 6 0 0 0 12 0M12 16v4"
              className="stroke-accent"
            />
          )}
        </svg>
      </div>
    </div>
  )
}
