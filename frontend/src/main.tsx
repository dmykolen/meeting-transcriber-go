import { StrictMode, useSyncExternalStore } from "react"
import { createRoot } from "react-dom/client"
import { MotionConfig } from "motion/react"
import App from "./App"
import Strip from "./components/Strip"
import { Meetings } from "./api"
import { lang, onLang, setLang } from "./i18n"
import "./index.css"

// The same bundle serves the recording strip, a second window of its own.
const strip = location.hash === "#strip"
document.documentElement.classList.toggle("strip-window", strip)

// Changing the language redraws everything below, in place: the screen and
// whatever is open on it stay as they were.
function Root() {
  useSyncExternalStore(onLang, lang)
  return (
    <MotionConfig reducedMotion="user">{strip ? <Strip /> : <App />}</MotionConfig>
  )
}

// The language is read before the first frame, so nothing flashes in the other.
Meetings.Settings()
  .then((s) => setLang(s.uiLanguage))
  .catch(() => {})
  .finally(() =>
    createRoot(document.getElementById("root")!).render(
      <StrictMode>
        <Root />
      </StrictMode>,
    ),
  )
