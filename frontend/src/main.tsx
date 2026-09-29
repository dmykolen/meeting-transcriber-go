import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { MotionConfig } from "motion/react"
import App from "./App"
import Strip from "./components/Strip"
import "./index.css"

// The same bundle serves the recording strip, a second window of its own.
const strip = location.hash === "#strip"
document.documentElement.classList.toggle("strip-window", strip)

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MotionConfig reducedMotion="user">{strip ? <Strip /> : <App />}</MotionConfig>
  </StrictMode>,
)
