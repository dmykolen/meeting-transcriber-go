import { useEffect, useRef, useState } from "react"
import { Search as SearchIcon, Sparkles, X } from "lucide-react"
import Head from "../components/Head"
import KnowledgeSource from "../components/KnowledgeSource"
import { Meetings, many, why, type KnowledgeHit } from "../api"
import { t } from "../i18n"
const saved = {
  query: "",
  semantic: false,
  hits: null as KnowledgeHit[] | null,
  scroll: 0,
}
export default function Search({
  onOpen,
  onProject,
}: {
  onOpen: (id: number, at?: number) => void
  onProject?: (id: number) => void
}) {
  const [query, setQuery] = useState(saved.query),
    [semantic, setSemantic] = useState(saved.semantic),
    [hits, setHits] = useState(saved.hits),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("")
  const version = useRef(0),
    scroll = useRef<HTMLDivElement>(null)
  const find = async (q: string, mode: boolean) => {
    const token = ++version.current
    setBusy(true)
    setError("")
    try {
      const rows = (await Meetings.SearchKnowledge(q, mode)) as KnowledgeHit[]
      if (token === version.current) {
        setHits(rows ?? [])
        saved.hits = rows ?? []
      }
    } catch (e) {
      if (token === version.current) {
        setError(why(e))
        setHits(null)
      }
    } finally {
      if (token === version.current) setBusy(false)
    }
  }
  useEffect(() => {
    saved.query = query
    saved.semantic = semantic
    const token = ++version.current
    if (query.trim().length < 2) {
      setHits(null)
      setBusy(false)
      return
    }
    if (semantic) {
      setBusy(false)
      return
    }
    const timer = setTimeout(() => void find(query, false), 180)
    return () => {
      clearTimeout(timer)
      if (version.current === token) version.current++
    }
  }, [query, semantic])
  useEffect(() => {
    if (scroll.current) scroll.current.scrollTop = saved.scroll
    return () => {
      version.current++
    }
  }, [])
  return (
    <div className="knowledge-screen">
      <Head title={t("Пошук в архіві")} />
      <div className="knowledge-query">
        <div className="knowledge-query-line">
          <SearchIcon size={15} />
          <input
            aria-label={t("Пошук в архіві")}
            autoFocus
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setHits(null)
              saved.hits = null
              setError("")
            }}
            onKeyDown={(e) => e.key === "Enter" && void find(query, semantic)}
            placeholder={t("Знайти сказане, записане або вирішене…")}
          />
          {query && (
            <button
              className="ui-icon"
              aria-label={t("Очистити запит")}
              onClick={() => {
                setQuery("")
                setHits(null)
                saved.hits = null
              }}
            >
              <X size={14} />
            </button>
          )}
          {semantic && (
            <button
              className="ui-primary"
              disabled={busy || query.trim().length < 2}
              onClick={() => void find(query, true)}
            >
              {busy ? t("Шукаю…") : t("Знайти")}
            </button>
          )}
        </div>
        <div className="knowledge-modes">
          <button
            aria-pressed={!semantic}
            className={!semantic ? "active" : ""}
            onClick={() => {
              setSemantic(false)
              setHits(null)
              saved.hits = null
            }}
          >
            {t("Точні слова")}
          </button>
          <button
            aria-pressed={semantic}
            className={semantic ? "active" : ""}
            onClick={() => {
              setSemantic(true)
              setHits(null)
              saved.hits = null
            }}
          >
            <Sparkles size={12} />
            {t("За змістом")}
          </button>
          <span>{t("Зустрічі · проєкти · нотатки · домовленості")}</span>
        </div>
      </div>
      <div
        className="knowledge-results"
        ref={scroll}
        onScroll={(e) => (saved.scroll = e.currentTarget.scrollTop)}
      >
        {error && (
          <p className="error" role="alert">
            {error}
            <button
              className="ui-chip"
              onClick={() => void find(query, semantic)}
            >
              {t("Повторити")}
            </button>
          </p>
        )}
        {busy && (
          <div className="search-working">
            {semantic
              ? t("Оновлюю індекс і зіставляю джерела…")
              : t("Шукаю слова…")}
            <i />
          </div>
        )}
        {!busy && hits && (
          <p className="result-count">
            {hits.length === 60
              ? t("{n} перших джерел", { n: hits.length })
              : `${hits.length} ${many(hits.length, "джерело", "джерела", "джерел")}`}
          </p>
        )}
        {!busy && hits?.length === 0 && (
          <p className="search-empty">
            {t("Збігів немає. Спробуйте інше формулювання або пошук за змістом.")}
          </p>
        )}
        {!hits && !busy && !error && (
          <p className="search-empty">
            {semantic
              ? t("Введіть думку або питання й натисніть «Знайти».")
              : t("Пошук починається від двох символів.")}
          </p>
        )}
        {!busy &&
          hits?.map((h) => (
            <KnowledgeSource
              key={h.key}
              hit={h}
              onOpen={onOpen}
              onProject={onProject}
            />
          ))}
      </div>
    </div>
  )
}
