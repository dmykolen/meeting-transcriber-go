import { useEffect, useState } from "react"
import { Meetings, type UsageReport } from "../api"
import { locale, t } from "../i18n"

const days = 30

const tokens = (n: number) =>
  n >= 1e6 ? `${(n / 1e6).toFixed(1)}M` : n >= 1e3 ? `${Math.round(n / 1e3)}k` : String(n)
const dollars = (n: number) => `$${n < 0.01 && n > 0 ? "<0.01" : n.toFixed(2)}`
const credits = (n: number) => n.toLocaleString(locale(), { maximumFractionDigits: 1 })

/**
 * What the models were asked and what it cost: a bar a day, then a line a
 * model. OpenAI is priced per token from a list in the app, so a model it does
 * not know shows tokens only; Copilot reports AI Credits itself; a local model
 * costs nothing but time.
 */
export default function Usage() {
  const [report, setReport] = useState<UsageReport | null>(null)

  useEffect(() => {
    let alive = true
    const read = () =>
      Meetings.Usage(days)
        .then((r) => alive && setReport(r as UsageReport))
        .catch(() => {})
    read()
    const timer = setInterval(read, 15000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [])

  if (!report) return null
  const calls = report.models.reduce((n, m) => n + m.calls, 0)
  if (!calls)
    return <p className="px-4 py-3 text-[12px] text-faint">{t("Запитів до моделей за {days} днів ще не було.", { days })}</p>

  const top = Math.max(...report.daily.map((d) => d.tokens), 1)
  const day = (d: string) => new Date(d + "T12:00").toLocaleDateString(locale(), { day: "numeric", month: "short" })
  return (
    <div className="space-y-3 px-4 py-3 text-[12px]">
      <p className="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-soft">
        <span>
          <b className="font-medium text-text tabular-nums">{calls}</b> {t("запитів за {days} днів", { days })}
        </span>
        {report.cost > 0 && (
          <span title={t("За цінами OpenAI; модель без відомої ціни не враховано.")}>
            ≈ <b className="font-medium text-text tabular-nums">{dollars(report.cost)}</b> OpenAI
          </span>
        )}
        {report.credits > 0 && (
          <span>
            <b className="font-medium text-text tabular-nums">{credits(report.credits)}</b> AI Credits
          </span>
        )}
      </p>

      <div className="flex h-14 items-end gap-[3px]" role="img" aria-label={t("Токени за днями")}>
        {report.daily.map((d) => (
          <div
            key={d.day}
            className="flex-1 rounded-t-[2px] bg-accent/70"
            style={{ height: `${Math.max((d.tokens / top) * 100, d.calls ? 4 : 0)}%`, minWidth: 2 }}
            title={`${day(d.day)} · ${t("{n} запитів, {tokens} токенів", { n: d.calls, tokens: tokens(d.tokens) })}${
              d.cost ? ` · ${dollars(d.cost)}` : ""
            }${d.credits ? ` · ${credits(d.credits)} AI Credits` : ""}`}
          />
        ))}
      </div>
      <div className="flex justify-between text-[10px] text-faint">
        <span>{day(report.daily[0].day)}</span>
        <span>{day(report.daily[report.daily.length - 1].day)}</span>
      </div>

      <table className="w-full text-left tabular-nums">
        <thead className="text-[10px] uppercase tracking-wider text-faint">
          <tr>
            <th className="py-1 font-semibold">{t("Модель")}</th>
            <th className="py-1 text-right font-semibold">{t("Запити")}</th>
            <th className="py-1 text-right font-semibold">{t("Токени вх. / вих.")}</th>
            <th className="py-1 text-right font-semibold">{t("Час")}</th>
            <th className="py-1 text-right font-semibold">{t("Вартість")}</th>
          </tr>
        </thead>
        <tbody className="text-soft">
          {report.models.map((m) => (
            <tr key={m.provider + m.model} className="border-t border-line/40">
              <td className="py-1.5">
                <span className="text-text">{m.model}</span> <span className="text-faint">{m.provider}</span>
              </td>
              <td className="py-1.5 text-right">
                {m.calls}
                {m.failed > 0 && <span className="text-warn"> · {t("{n} невдало", { n: m.failed })}</span>}
              </td>
              <td className="py-1.5 text-right">
                {tokens(m.input)} / {tokens(m.output)}
              </td>
              <td className="py-1.5 text-right">{Math.round(m.seconds)} {t("с")}</td>
              <td className="py-1.5 text-right">
                {m.priced ? `≈ ${dollars(m.cost)}` : m.credits ? `${credits(m.credits)} Credits` : m.provider === "local" ? t("локально") : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
