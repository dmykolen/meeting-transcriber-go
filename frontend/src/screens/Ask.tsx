import { useState } from "react"
import { ArrowUpRight, Sparkles } from "lucide-react"
import Head from "../components/Head"
import KnowledgeSource from "../components/KnowledgeSource"
import { Meetings, why, type KnowledgeAnswer } from "../api"
import NeedsKey from "../components/NeedsKey"
import { t } from "../i18n"
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
      setProblem(why(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="knowledge-screen">
      <Head title={t("Запитати архів")} />
      <div className="knowledge-query">
        <NeedsKey what={t("Відповіді з архіву")} />
        <p className="knowledge-intro">
          {t(
            "Пов’яжіть сказане на зустрічах із рішеннями проєктів і власними нотатками.",
          )}
        </p>
        <div className="knowledge-query-line">
          <Sparkles size={15} />
          <input
            aria-label={t("Питання до архіву")}
            value={question}
            onChange={(e) => {
              setQuestion(e.target.value)
              saved.question = e.target.value
            }}
            onKeyDown={(e) => e.key === "Enter" && void send()}
            placeholder={t("Що змінилося і чому?")}
          />
          <button
            className="ui-primary"
            disabled={busy || !question.trim()}
            onClick={() => void send()}
          >
            {busy ? t("Зіставляю…") : t("Запитати")}
            <ArrowUpRight size={13} />
          </button>
        </div>
        <div className="knowledge-coverage">
          <span>{t("Увесь архів")}</span>
          <small>
            {t(
              "Зустрічі · розшифровки · підсумки · проєкти · нотатки · домовленості",
            )}
          </small>
        </div>
      </div>
      <div className="knowledge-results">
        {busy && (
          <div className="search-working">
            {t("Знаходжу джерела та формую відповідь…")}
            <i />
          </div>
        )}
        {problem && (
          <p role="alert" className="error">
            {problem}
            <button className="ui-chip" onClick={() => void send()}>
              {t("Повторити")}
            </button>
          </p>
        )}
        {answer && !busy && (
          <>
            <p className="knowledge-answer">{answer.text}</p>
            <h3 className="result-count">
              {t("Джерела відповіді · {n}", { n: answer.sources?.length || 0 })}
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
