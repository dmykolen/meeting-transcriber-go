import type { ComponentProps } from "react"
import { CalendarDays, SlidersHorizontal } from "lucide-react"
import Timeline from "./Timeline"
import Reveal from "./Reveal"
import { locale, t } from "../i18n"
import { many } from "../api"
export default function EdgeTimeline(props: ComponentProps<typeof Timeline>) {
  const latest = props.marks.reduce(
    (date, m) => (m.started > date ? m.started : date),
    "",
  )
  return (
    <Reveal
      className="timeline-reveal"
      label={
        <>
          <span className="edge-line" />
          <span className="edge-caption">
            <CalendarDays size={12} />
            {t("Хронологія")}
          </span>
          <span className="edge-line" />
        </>
      }
    >
      <header className="timeline-title">
        <span>
          <SlidersHorizontal size={14} />
          <strong>{t("Хронологія зустрічей")}</strong>
          <small>
            {props.marks.length}{" "}
            {many(props.marks.length, "запис", "записи", "записів")}
          </small>
        </span>
        <small>
          {latest
            ? new Date(latest).toLocaleDateString(locale(), {
                day: "numeric",
                month: "long",
              })
            : t("Архів порожній")}
        </small>
      </header>
      <Timeline {...props} />
      <div className="timeline-fields">
        <label>
          {t("Від")}
          <input
            aria-label={t("Початок діапазону")}
            type="date"
            value={props.range?.[0] ?? ""}
            onChange={(e) =>
              e.target.value &&
              props.onRange([
                e.target.value,
                (props.range?.[1] || e.target.value) < e.target.value
                  ? e.target.value
                  : props.range?.[1] || e.target.value,
              ])
            }
          />
        </label>
        <span>—</span>
        <label>
          {t("До")}
          <input
            aria-label={t("Кінець діапазону")}
            type="date"
            value={props.range?.[1] ?? ""}
            onChange={(e) =>
              e.target.value &&
              props.onRange([
                (props.range?.[0] || e.target.value) > e.target.value
                  ? e.target.value
                  : props.range?.[0] || e.target.value,
                e.target.value,
              ])
            }
          />
        </label>
        {props.range && (
          <button onClick={() => props.onRange(null)}>{t("Увесь період")}</button>
        )}
        <small>
          {t("Наведіть на запис · протягніть діапазон · клік фіксує панель")}
        </small>
      </div>
    </Reveal>
  )
}
