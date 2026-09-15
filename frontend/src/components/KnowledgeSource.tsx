import { useState } from "react"
import {
  ArrowUpRight,
  FileText,
  NotebookPen,
  Folder,
  AudioLines,
} from "lucide-react"
import { clock, type KnowledgeHit } from "../api"
import Drawer from "./Drawer"
export const kindName: Record<string, string> = {
  transcript: "Розшифровка",
  summary: "Підсумок",
  note: "Нотатка",
  project: "Проєкт",
  action: "Домовленість",
  question: "Відкрите питання",
}
export default function KnowledgeSource({
  hit,
  onOpen,
  onProject,
}: {
  hit: KnowledgeHit
  onOpen: (id: number, at?: number) => void
  onProject?: (id: number) => void
}) {
  const [open, setOpen] = useState(false)
  const Icon =
    hit.kind === "note"
      ? NotebookPen
      : hit.kind === "project"
        ? Folder
        : hit.kind === "transcript"
          ? AudioLines
          : FileText
  const go = () => {
    setOpen(false)
    if (hit.project) onProject?.(hit.project)
    else
      onOpen(hit.recording, hit.kind === "transcript" ? hit.start : undefined)
  }
  return (
    <>
      <article className="knowledge-hit">
        <header>
          <Icon size={13} />
          <span>{kindName[hit.kind] || hit.kind}</span>
          <b>{hit.title}</b>
          {hit.kind === "transcript" && <time>{clock(hit.start)}</time>}
        </header>
        <button className="hit-text" onClick={go}>
          {hit.text}
        </button>
        <footer>
          <button onClick={go}>
            Відкрити {hit.kind === "transcript" ? "момент" : "документ"}
            <ArrowUpRight size={12} />
          </button>
          <button onClick={() => setOpen(true)}>Джерело поруч</button>
        </footer>
      </article>
      <Drawer
        open={open}
        onClose={() => setOpen(false)}
        title={kindName[hit.kind] || "Джерело"}
      >
        <div className="source-document">
          <h3>{hit.title}</h3>
          {hit.kind === "transcript" && <time>{clock(hit.start)}</time>}
          <p>{hit.text}</p>
          <button className="ui-chip" onClick={go}>
            Відкрити документ
            <ArrowUpRight size={13} />
          </button>
        </div>
      </Drawer>
    </>
  )
}
