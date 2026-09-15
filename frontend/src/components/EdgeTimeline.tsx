import type { ComponentProps } from "react"
import { CalendarDays, SlidersHorizontal } from "lucide-react"
import Timeline from "./Timeline"
import Reveal from "./Reveal"
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
            Хронологія
          </span>
          <span className="edge-line" />
        </>
      }
    >
      <header className="timeline-title">
        <span>
          <SlidersHorizontal size={14} />
          <strong>Хронологія зустрічей</strong>
          <small>{props.marks.length} записів</small>
        </span>
        <small>
          {latest
            ? new Date(latest).toLocaleDateString("uk", {
                day: "numeric",
                month: "long",
              })
            : "Архів порожній"}
        </small>
      </header>
      <Timeline {...props} />
      <div className="timeline-fields">
        <label>
          Від
          <input
            aria-label="Початок діапазону"
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
          До
          <input
            aria-label="Кінець діапазону"
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
          <button onClick={() => props.onRange(null)}>Увесь період</button>
        )}
        <small>
          Наведіть на запис · протягніть діапазон · клік фіксує панель
        </small>
      </div>
    </Reveal>
  )
}
