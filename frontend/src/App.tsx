import { useEffect, useState, lazy, Suspense } from "react"
import { motion } from "motion/react"
import { Status, type SetupState } from "./api"
import Palette from "./components/Palette"
import Setup from "./screens/Setup"
import Today from "./screens/Today"
import Workspace from "./screens/Meetings"
import Search from "./screens/Search"
import Todo from "./screens/Todo"
import Ask from "./screens/Ask"
import Rail, { type Screen } from "./components/Rail"
import { t } from "./i18n"

const SettingsScreen = lazy(() => import("./screens/Settings"))

export default function App() {
  const [setup, setSetup] = useState<SetupState | null>(null)
  const [screen, setScreen] = useState<Screen>("today")
  const [open, setOpen] = useState<number | null>(null)
  const [pickAt, setPickAt] = useState<number | undefined>()
  const [origin, setOrigin] = useState<Screen | null>(null)
  const [project, setProject] = useState<number | null>(null)

  // Polled rather than pushed: one small object once a second, and an event
  // channel for it would be more machinery than the thing it carries.
  useEffect(() => {
    let alive = true
    const read = async () => {
      try {
        const state = (await Status.State()) as SetupState
        if (alive) setSetup(state)
      } catch {
        // The window can outlive a reload in development; a failed poll is not
        // worth a red screen.
      }
    }
    read()
    const timer = setInterval(read, 1000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  const working = setup?.stage === "ready"

  // ⌘1..5 and ⌘, move between screens; the palette owns ⌘K. Every rail
  // button carries its own shortcut in the tooltip.
  useEffect(() => {
    if (!working) return
    const jump = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return
      const to: Record<string, Screen> = {
        "1": "today",
        "2": "library",
        "3": "search",
        "4": "todo",
        "5": "ask",
        ",": "settings",
      }
      const next = to[e.key.toLowerCase()]
      if (!next) return
      e.preventDefault()
      setOpen(null)
      setScreen(next)
    }
    window.addEventListener("keydown", jump)
    return () => window.removeEventListener("keydown", jump)
  }, [working])

  const show = (id: number, at?: number) => {
    setPickAt(at)
    setOrigin(screen === "library" ? null : screen)
    setOpen(id)
    setScreen("library")
  }

  const showProject = (id: number) => {
    setOrigin(screen)
    setOpen(null)
    setProject(id)
    setScreen("library")
  }

  return (
    <div className="flex h-full flex-col bg-ink/80">
      {/* The title bar was forty-two pixels of nothing but the traffic lights.
          The one line that reaches anything in the app lives there now, which is
          how a command palette exists here without a panel thrown over the
          middle of the screen. */}
      <div className="titlebar flex shrink-0 items-center gap-3 pl-[86px] pr-4">
        <Palette
          onOpenMeeting={show}
          onOpenProject={(id) => {
            setProject(id)
            setScreen("library")
          }}
          onScreen={(s) => {
            setOpen(null)
            setScreen(s)
          }}
        />
      </div>

      {/* No AnimatePresence around this one. It switches once per launch, and
          mode="wait" holds the incoming screen until every animation in the
          outgoing subtree has finished — which deadlocked here and left a black
          window with the app mounted behind an invisible setup screen. A plain
          conditional cannot do that. */}
      {!working ? (
        <div className="flex-1">
          <Setup state={setup} />
        </div>
      ) : (
        <motion.div
          className="flex min-h-0 flex-1"
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
        >
          <Rail
            screen={screen}
            onChange={(s) => {
              setOpen(null)
              setScreen(s)
            }}
          />
          {/* Keyed, but deliberately not wrapped in AnimatePresence. An exit
              animation here would mean mode="wait", and that holds the incoming
              screen until every animation in the outgoing one has finished —
              including the pulse on a recording that is still being processed,
              which repeats for ever. The screen simply stopped changing. The
              new screen animating in is the whole effect anyway. */}
          <main className="min-w-0 flex-1">
            <motion.div
              key={screen}
              className="h-full"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
            >
              {screen === "today" && <Today onOpen={show} />}
              {screen === "library" && (
                <Workspace
                  pick={open}
                  pickAt={pickAt}
                  onReturn={
                    origin
                      ? () => {
                          setScreen(origin)
                          setOrigin(null)
                        }
                      : undefined
                  }
                  returnLabel={
                    origin === "ask"
                      ? t("До відповіді")
                      : origin === "search"
                        ? t("До пошуку")
                        : t("Назад")
                  }
                  onPicked={() => setOpen(null)}
                  project={project}
                  onProject={setProject}
                />
              )}
              {screen === "search" && (
                <Search onOpen={show} onProject={showProject} />
              )}
              {screen === "todo" && <Todo onOpen={show} />}
              {screen === "ask" && (
                <Ask onOpen={show} onProject={showProject} />
              )}
              {screen === "settings" && (
                <Suspense
                  fallback={
                    <p className="reader-loading">{t("Відкриваю параметри…")}</p>
                  }
                >
                  <SettingsScreen />
                </Suspense>
              )}
            </motion.div>
          </main>
        </motion.div>
      )}
    </div>
  )
}
