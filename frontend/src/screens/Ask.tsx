import { useState } from "react"
import { ArrowUpRight, Sparkles } from "lucide-react"
import Head from "../components/Head"
import KnowledgeSource from "../components/KnowledgeSource"
import { Meetings, type KnowledgeAnswer } from "../api"
import NeedsKey from "../components/NeedsKey"
const saved = { question: "", answer: null as KnowledgeAnswer | null }
export default function Ask({
  onOpen,
  onProject,
}: {
  onOpen: (id: number, at?: number) => void
  onProject?: (id: number) => void
}) {
  const [question, setQuestion] = useState(saved.question),
    [answer, setAnswer] = useState(saved.answer),
    [busy, setBusy] = useState(false),
    [problem, setProblem] = useState("")
  const send = async () => {
    if (busy || !question.trim()) return
    setBusy(true)
    setProblem("")
    setAnswer(null)
    saved.answer = null
    try {
      const result = (await Meetings.AskKnowledge(question)) as KnowledgeAnswer
      setAnswer(result)
      saved.answer = result
    } catch (e) {
      setProblem(String(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="knowledge-screen">
      <Head title="Запитати архів" />
      <div className="knowledge-query">
        <NeedsKey what="Відповіді з архіву" />
        <p className="knowledge-intro">
          Пов’яжіть сказане на зустрічах із рішеннями проєктів і власними
          нотатками.
        </p>
        <div className="knowledge-query-line">
          <Sparkles size={15} />
          <input
            aria-label="Питання до архіву"
            value={question}
            onChange={(e) => {
              setQuestion(e.target.value)
              saved.question = e.target.value
            }}
            onKeyDown={(e) => e.key === "Enter" && void send()}
            placeholder="Що змінилося і чому?"
          />
          <button
            className="ui-primary"
            disabled={busy || !question.trim()}
            onClick={() => void send()}
          >
            {busy ? "Зіставляю…" : "Запитати"}
            <ArrowUpRight size={13} />
          </button>
        </div>
        <div className="knowledge-coverage">
          <span>Увесь архів</span>
          <small>
            Зустрічі · розшифровки · підсумки · проєкти · нотатки · домовленості
          </small>
        </div>
      </div>
      <div className="knowledge-results">
        {busy && (
          <div className="search-working">
            Знаходжу джерела та формую відповідь…
            <i />
          </div>
        )}
        {problem && (
          <p role="alert" className="error">
            {problem}
            <button className="ui-chip" onClick={() => void send()}>
              Повторити
            </button>
          </p>
        )}
        {answer && !busy && (
          <>
            <p className="knowledge-answer">{answer.text}</p>
            <h3 className="result-count">
              Джерела відповіді · {answer.sources?.length || 0}
            </h3>
            {answer.sources?.map((h) => (
              <KnowledgeSource
                key={h.key}
                hit={h}
                onOpen={onOpen}
                onProject={onProject}
              />
            ))}
          </>
        )}
      </div>
    </div>
  )
}
