import {
  useEffect,
  useState,
  useRef,
  useId,
  createContext,
  useContext,
} from "react"
import { AnimatePresence, motion } from "motion/react"
import {
  Brain,
  Cable,
  Check,
  ChevronRight,
  Copy,
  Ear,
  FolderOpen,
  LogIn,
  Mic,
  Rows2,
  Sparkles,
  Trash2,
  UserRound,
  X,
} from "lucide-react"
import { Browser } from "@wailsio/runtime"
import {
  Meetings,
  copyText,
  many,
  why,
  type AIState,
  type CopilotAccount,
  type Group,
  type MCPState,
  type Person,
  type Settings as Values,
  type Source,
} from "../api"
import { colourOf, picked, tone, wash } from "../colours"
import { setLang, t, tr } from "../i18n"
import Head from "../components/Head"
import Paint from "../components/Paint"
import Snippet from "../components/Snippet"

const SettingLabel = createContext("")

export default function Settings() {
  const [values, setValues] = useState<Values | null>(null)
  const [saved, setSaved] = useState(false)
  const [people, setPeople] = useState<Person[]>([])
  const [projects, setProjects] = useState<Group[]>([])
  const [busy, setBusy] = useState("")
  const [said, setSaid] = useState("")
  const [me, setMe] = useState("")
  const [mcp, setMcp] = useState<MCPState | null>(null)
  const [ai, setAI] = useState<AIState | null>(null)
  const [account, setAccount] = useState<CopilotAccount | null>(null)
  const [checking, setChecking] = useState(false)
  const saving = useRef(Promise.resolve())

  // Downloads and the GitHub sign-in move on their own; the screen follows.
  useEffect(() => {
    let alive = true
    const read = () =>
      Meetings.AIStatus()
        .then((s) => alive && setAI(s as AIState))
        .catch(() => {})
    read()
    const timer = window.setInterval(read, 1500)
    return () => {
      alive = false
      window.clearInterval(timer)
    }
  }, [])

  // Asked once Copilot is chosen, and again whenever a download moves on or
  // the sign-in ends: the answer starts the Copilot CLI for a moment.
  const copilot = values?.aiProvider === "copilot"
  const signingIn = !!ai?.signingIn
  const fetching = ai?.fetching ?? null
  useEffect(() => {
    if (!copilot || signingIn || fetching === null) return
    let alive = true
    setChecking(true)
    Meetings.Copilot()
      .then((a) => alive && setAccount(a as CopilotAccount))
      .catch((e) => alive && setSaid(why(e)))
      .finally(() => alive && setChecking(false))
    return () => {
      alive = false
    }
  }, [copilot, signingIn, fetching])

  const teach = () =>
    run("me", async () => {
      const name = me.trim()
      const taught = await Meetings.ThisIsMe(name)
      setMe("")
      voices()
      return taught
        ? t("Ваші репліки відтепер підписані «{name}», а голос запам’ятано з {n} {word}.", {
            name,
            n: taught,
            word: many(taught, "запису", "записів", "записів"),
          })
        : t(
            "Ваші репліки відтепер підписані «{name}». Жоден запис ще не має досить вашого голосу, тож голос запам’ятається з наступної зустрічі.",
            { name },
          )
    })

  // Both of these can take a moment and both have something to report, so they
  // share one line rather than each growing its own spinner.
  const run = async (what: string, job: () => Promise<unknown>) => {
    setBusy(what)
    setSaid("")
    try {
      setSaid(String(await job()))
    } catch (e) {
      setSaid(why(e))
    } finally {
      setBusy("")
    }
  }

  const voices = () =>
    Meetings.People().then((p) => setPeople((p as Person[]) ?? []))
  const folders = () =>
    Meetings.Groups().then((g) => setProjects((g as Group[]) ?? []))

  useEffect(() => {
    let alive = true
    const refreshMCP = () =>
      Meetings.MCPStatus()
        .then((state) => alive && setMcp(state as MCPState))
        .catch((e) => {
          if (alive)
            setMcp({
              status: "failed",
              url: "",
              command: "",
              problem: why(e),
            })
        })

    Meetings.Settings()
      .then((v) => setValues(v as Values))
      .catch((e) => setSaid(why(e)))
    voices().catch(() => {})
    folders().catch(() => {})
    refreshMCP()
    const timer = window.setInterval(refreshMCP, 3000)
    return () => {
      alive = false
      window.clearInterval(timer)
    }
  }, [])

  if (!values)
    return (
      <p className="reader-loading">{said ? tr(said) : t("Відкриваю параметри…")}</p>
    )

  const save = async (next: Values) => {
    setValues(next)
    setSaved(false)
    saving.current = saving.current
      .catch(() => {})
      .then(() => Meetings.SaveSettings(next))
    try {
      await saving.current
    } catch (e) {
      setSaid("Не збережено: " + why(e))
      return
    }
    setSaved(true)
    setTimeout(() => setSaved(false), 1600)
  }

  return (
    <div className="settings-screen flex h-full flex-col">
      <Head title={t("Параметри")}>
        <motion.span
          className="flex items-center gap-1 text-[11px] text-good"
          animate={{ opacity: saved ? 1 : 0 }}
        >
          <Check size={12} />
          {t("Збережено")}
        </motion.span>
        <div role="group" aria-label={t("Мова інтерфейсу")}>
          <Choice
            options={[
              { id: "uk", label: "Українська" },
              { id: "en", label: "English" },
            ]}
            value={values.uiLanguage}
            onChange={(id) => {
              void setLang(id)
              void save({ ...values, uiLanguage: id as Values["uiLanguage"] })
            }}
          />
        </div>
      </Head>
      {said && (
        <p className="reader-error" role="status">
          {tr(said)}
          {said.startsWith("Не збережено") && (
            <button className="ui-chip" onClick={() => void save(values)}>
              {t("Повторити")}
            </button>
          )}
        </p>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-10">
        <div className="settings-grid">
          <Group title={t("Слухання")} Icon={Ear}>
            <Row
              label={t("Автоматично записувати зустрічі")}
              hint={t("Аудіо обробляється на цьому пристрої.")}
            >
              <Toggle
                on={values.listening}
                onChange={(on) => save({ ...values, listening: on })}
              />
            </Row>
            <Row
              label={t("Джерела звуку")}
              hint={
                values.system
                  ? t("Ваш голос і звук співрозмовників записуються окремо.")
                  : t("Лише звук мікрофона. Автоматичні записи в цьому режимі не відкидаються.")
              }
            >
              <Choice
                options={[
                  { id: "both", label: t("Мікрофон + система") },
                  { id: "mic", label: t("Мікрофон") },
                ]}
                value={values.system ? "both" : "mic"}
                onChange={(id) => save({ ...values, system: id === "both" })}
              />
            </Row>
            <Row
              label={t("Почати після мовлення")}
              hint={t("Тривалість мовлення для автоматичного старту.")}
            >
              <Number
                value={values.startSpeech}
                unit={t("с")}
                min={5}
                max={300}
                onChange={(n) => save({ ...values, startSpeech: n })}
              />
            </Row>
            <Row
              label={t("Завершити після тиші")}
              hint={t("Пауза, після якої зустріч вважається завершеною.")}
            >
              <Number
                value={values.quietEnds}
                unit={t("с")}
                min={15}
                max={1800}
                onChange={(n) => save({ ...values, quietEnds: n })}
              />
            </Row>
            <Row
              label={t("Зберігати автоматичні голосові нотатки")}
              hint={t("Зберігати також записи, де говорите лише ви. Ручний запис зберігається завжди.")}
            >
              <Toggle
                on={values.keepNotes}
                onChange={(on) => save({ ...values, keepNotes: on })}
              />
            </Row>
            <Row
              label={t("Захопити початок")}
              hint={t("Додати попередні секунди, якщо зустріч помічено із запізненням.")}
            >
              <Number
                value={values.preroll}
                unit={t("с")}
                min={0}
                max={600}
                onChange={(n) => save({ ...values, preroll: n })}
              />
            </Row>
          </Group>

          <Group title={t("Розшифровка")} Icon={Rows2}>
            <Row
              label={t("Мова")}
              hint={
                t("Код мови: uk, en або auto. Явний вибір допомагає правильно розпізнавати українську.")
              }
            >
              <Text
                value={values.language}
                placeholder="uk"
                width="w-20"
                onChange={(v) => setValues({ ...values, language: v })}
                onDone={() => save(values)}
              />
            </Row>
            <Row
              label={t("Модель розпізнавання")}
              hint={t("Whisper підтримує вибір мови. Parakeet визначає її автоматично; перший запуск завантажить модель.")}
            >
              <Choice
                options={[
                  { id: "whisper", label: "Whisper" },
                  { id: "parakeet", label: "Parakeet" },
                ]}
                value={values.transcriber}
                onChange={(id) =>
                  save({ ...values, transcriber: id as Values["transcriber"] })
                }
              />
            </Row>
            <Row
              label={t("Коли розшифровувати")}
              hint={
                {
                  after: t(
                    "Одразу після кожного запису. Поки йде розшифровка, Mac працює на повну.",
                  ),
                  at: t("Записи чекають до вказаної години. Якщо Mac тоді спить, розшифровка почнеться, щойно він прокинеться."),
                  idle: t("Коли ви 5 хвилин не користуєтесь Mac і його не завантажує інша робота."),
                }[values.transcribe]
              }
            >
              <Choice
                options={[
                  { id: "after", label: t("Одразу") },
                  { id: "at", label: t("О годині") },
                  { id: "idle", label: t("Авто") },
                ]}
                value={values.transcribe}
                onChange={(id) =>
                  save({ ...values, transcribe: id as Values["transcribe"] })
                }
              />
            </Row>
            {values.transcribe === "at" && (
              <Row
                label={t("О котрій")}
                hint={t("Щодня. Записане пізніше чекає до наступного дня; потрібну зустріч можна розшифрувати одразу кнопкою в ній.")}
              >
                {/* Not type="time": WebKit draws that in the app's English
                    locale, as 07:00 PM, whatever lang says. */}
                <Text
                  value={values.transcribeAt}
                  placeholder="19:00"
                  width="w-20"
                  onChange={(v) => setValues({ ...values, transcribeAt: v })}
                  onDone={() => save(values)}
                />
              </Row>
            )}
          </Group>

          <Group title={t("Учасники")} Icon={UserRound}>
            <Row
              label={t("Мій голос")}
              hint={t("Вкажіть ім’я для голосу з мікрофона у наступних записах.")}
            >
              <div className="flex items-center gap-1.5">
                <input
                  value={me}
                  onChange={(e) => setMe(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && teach()}
                  placeholder={t("Ваше ім’я")}
                  className="w-32 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[12px] outline-none transition-colors placeholder:text-faint focus:border-accent/50"
                />
                <button
                  onClick={teach}
                  disabled={busy !== "" || !me.trim()}
                  className="flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1.5 text-[11.5px] font-medium text-ink transition-opacity disabled:opacity-30"
                >
                  <Mic size={13} />{" "}
                  {busy === "me" ? t("Запам’ятовую…") : t("Запам’ятати")}
                </button>
              </div>
            </Row>
            {people.length === 0 ? (
              <p className="px-4 py-3 text-[12px] leading-relaxed text-faint">
                {t(
                  "Учасників ще немає. Назвіть голос у зустрічі, щоб розпізнавати його надалі.",
                )}
              </p>
            ) : (
              people.map((p) => (
                <Face key={p.id} person={p} onChanged={voices} />
              ))
            )}
          </Group>

          <Group title={t("Проєкти")} Icon={FolderOpen}>
            {projects.length === 0 ? (
              <p className="px-4 py-3 text-[12px] leading-relaxed text-faint">
                {t("Створіть проєкт у Dock і додайте до нього зустрічі.")}
              </p>
            ) : (
              projects.map((g) => (
                <Folder key={g.id} group={g} onChanged={folders} />
              ))
            )}
          </Group>

          <Group title={t("AI та архів")} Icon={Sparkles}>
            <Row
              label={t("AI для підсумків і відповідей")}
              hint={
                {
                  openai: t("OpenAI за вашим ключем."),
                  copilot: t(
                    "Моделі вашого GitHub Copilot. Запити витрачають AI Credits плану.",
                  ),
                  local: t(
                    "Модель на цьому Mac: текст зустрічей нікуди не надсилається.",
                  ),
                }[values.aiProvider]
              }
            >
              <Choice
                options={[
                  { id: "openai", label: "OpenAI" },
                  { id: "copilot", label: "GitHub Copilot" },
                  { id: "local", label: t("Локально") },
                ]}
                value={values.aiProvider}
                onChange={(id) =>
                  save({ ...values, aiProvider: id as Values["aiProvider"] })
                }
              />
            </Row>
            {values.aiProvider === "copilot" && (
              <Row
                label="GitHub Copilot"
                hint={
                  ai?.signingIn
                    ? t("Підтвердіть доступ у браузері, який відкрився.")
                    : checking
                      ? t("Перевіряю акаунт…")
                      : account?.login
                        ? t("Підключено: {login}.", { login: account.login })
                        : t("Не підключено. Відкриється браузер, де треба підтвердити доступ.")
                }
              >
                <button
                  onClick={() =>
                    Meetings.ConnectCopilot().catch((e) =>
                      setSaid(why(e)),
                    )
                  }
                  disabled={signingIn || checking}
                  className="flex items-center gap-1.5 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[11.5px] text-soft transition-colors hover:border-accent/40 hover:text-text disabled:opacity-40"
                >
                  <LogIn size={13} />
                  {account?.login ? t("Інший акаунт") : t("Підключити")}
                </button>
              </Row>
            )}
            {values.aiProvider === "copilot" && !!account?.login && (
              <Row label={t("Модель Copilot")} hint={t("«Автоматично» — Copilot обирає сам.")}>
                <select
                  aria-label={t("Модель Copilot")}
                  value={values.copilotModel}
                  onChange={(e) =>
                    save({ ...values, copilotModel: e.target.value })
                  }
                  className="w-44 rounded-lg border border-line/60 bg-surface/60 px-2 py-1.5 text-[12px] outline-none transition-colors focus:border-accent/50"
                >
                  <option value="">{t("Автоматично")}</option>
                  {account.models.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.name}
                    </option>
                  ))}
                </select>
              </Row>
            )}
            {values.aiProvider === "local" && (
              <Row
                label={t("Локальна модель")}
                hint={t("Вбудована — Gemma 4 E2B, 2,8 ГБ. Для більшої вставте посилання на .gguf з Hugging Face.")}
              >
                <Text
                  value={values.localModel}
                  placeholder="Gemma 4 E2B"
                  width="w-56"
                  onChange={(v) => setValues({ ...values, localModel: v })}
                  onDone={() => save(values)}
                />
              </Row>
            )}
            <Row
              label={t("Пошук за змістом")}
              hint={
                values.embeddings === "local"
                  ? t("Qwen3 Embedding на цьому Mac, 0,6 ГБ. Після зміни індекс перебудовується.")
                  : t("Вектори для пошуку й Ask робить OpenAI. Після зміни індекс перебудовується.")
              }
            >
              <Choice
                options={[
                  { id: "openai", label: "OpenAI" },
                  { id: "local", label: t("Локально") },
                ]}
                value={values.embeddings}
                onChange={(id) =>
                  save({ ...values, embeddings: id as Values["embeddings"] })
                }
              />
            </Row>
            {(values.aiProvider === "openai" ||
              values.embeddings === "openai") && (
              <Row
                label={t("Ключ OpenAI")}
                hint={t("Лише для того, що вище обрано через OpenAI. Розпізнавання мовлення працює локально.")}
              >
                <Text
                  value={values.openaiKey}
                  placeholder="sk-…"
                  secret
                  width="w-56"
                  onChange={(v) => setValues({ ...values, openaiKey: v })}
                  onDone={() => save(values)}
                />
              </Row>
            )}
            {values.aiProvider === "openai" && (
              <Row label={t("Модель OpenAI")} hint={t("Назва моделі, доступної вашому ключу.")}>
                <Text
                  value={values.openaiModel}
                  placeholder="gpt-5.4-mini"
                  width="w-40"
                  onChange={(v) => setValues({ ...values, openaiModel: v })}
                  onDone={() => save(values)}
                />
              </Row>
            )}
            <Row
              label={t("Автоматичні підсумки")}
              hint={t("Для зустрічей, усіх записів або лише вручну. Ручне оновлення доступне в документі.")}
            >
              <Choice
                options={[
                  { id: "meetings", label: t("Зустрічі") },
                  { id: "always", label: t("Усе") },
                  { id: "never", label: t("Вимкнено") },
                ]}
                value={values.summarise}
                onChange={(id) =>
                  save({ ...values, summarise: id as Values["summarise"] })
                }
              />
            </Row>
            <Row
              label={t("Індекс розшифровок")}
              hint={t("Оновити старі розшифровки. Решта архіву індексується під час пошуку за змістом.")}
            >
              <button
                onClick={() =>
                  run("index", async () => {
                    const done = await Meetings.Reindex()
                    return t(
                      "Пошук за змістом оновлено: {meaning} уривків шукаються за змістом, {words} — лише за словами.",
                      { meaning: done.meaning, words: done.words },
                    )
                  })
                }
                disabled={busy !== "" || !!ai?.indexing}
                className="flex items-center gap-1.5 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[11.5px] text-soft transition-colors hover:border-accent/40 hover:text-text disabled:opacity-40"
              >
                <Brain size={13} />{" "}
                {busy === "index" ? t("Індексую…") : t("Оновити індекс")}
              </button>
            </Row>
            {ai && (ai.fetching || ai.indexing || ai.signingIn || ai.problem) && (
              <div
                role="status"
                className="space-y-1.5 border-t border-line/50 px-4 py-3 text-[11.5px] leading-relaxed"
              >
                {ai.fetching && (
                  <>
                    <p className="text-soft">
                      {t("Завантажую {what}", { what: ai.fetching })} ·{" "}
                      <span className="tabular-nums">
                        {Math.round(ai.fraction * 100)}%
                      </span>
                    </p>
                    <div className="h-1 overflow-hidden rounded-full bg-raised">
                      <div
                        className="h-full rounded-full bg-accent transition-[width] duration-500"
                        style={{ width: `${ai.fraction * 100}%` }}
                      />
                    </div>
                  </>
                )}
                {ai.indexing && (
                  <>
                    <p className="text-soft">
                      {t("Оновлюю пошук за змістом: {what}", {
                        what: tr(ai.indexing),
                      })}{" "}
                      ·{" "}
                      <span className="tabular-nums">
                        {Math.round(ai.indexed * 100)}%
                      </span>
                    </p>
                    <div className="h-1 overflow-hidden rounded-full bg-raised">
                      <div
                        className="h-full rounded-full bg-accent transition-[width] duration-500"
                        style={{ width: `${ai.indexed * 100}%` }}
                      />
                    </div>
                  </>
                )}
                {ai.signingIn &&
                  (ai.said ?? []).map((line) => (
                    <p key={line} className="break-all text-faint">
                      {line.startsWith("https://") ? (
                        <button
                          onClick={() => Browser.OpenURL(line).catch(() => {})}
                          className="text-left text-accent underline-offset-2 hover:underline"
                        >
                          {line}
                        </button>
                      ) : (
                        line
                      )}
                    </p>
                  ))}
                {ai.problem && <p className="text-warn">{tr(ai.problem)}</p>}
              </div>
            )}
          </Group>

          <Group
            title="MCP Server"
            Icon={Cable}
            wide
            afterTitle={<MCPIndicator state={mcp} />}
          >
            <MCPPanel
              state={mcp}
              onError={(message) => setSaid(message)}
            />
          </Group>

          <Group title={t("Зберігання")} Icon={FolderOpen}>
            <Row
              label={t("Зберігати аудіо")}
              hint={t("Строк у днях; 0 — без обмеження. Текст зберігається.")}
            >
              <Number
                value={values.keepAudioDays}
                unit={t("днів")}
                min={0}
                max={3650}
                onChange={(n) => save({ ...values, keepAudioDays: n })}
              />
            </Row>
            <Row
              label={t("Очистити старе аудіо")}
              hint={t("Видаляє лише звук за вказаним строком зберігання.")}
            >
              <button
                onClick={() =>
                  run("tidy", async () => {
                    const freed = await Meetings.Tidy()
                    return freed.kept
                      ? t("Нічого не видалено: аудіо зберігається без обмеження.")
                      : freed.files
                        ? t(
                            "Видалено аудіо {n} {word}, звільнено {mb} МБ. Розшифровки не змінено.",
                            {
                              n: freed.files,
                              word: many(freed.files, "запису", "записів", "записів"),
                              mb: freed.mb,
                            },
                          )
                        : t("Немає аудіо, старшого за цей строк.")
                  })
                }
                disabled={busy !== ""}
                className="flex items-center gap-1.5 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[11.5px] text-soft transition-colors hover:border-warn/50 hover:text-text disabled:opacity-40"
              >
                <Trash2 size={13} />{" "}
                {busy === "tidy" ? t("Очищаю…") : t("Звільнити місце")}
              </button>
            </Row>
            <Row label={t("Папка даних")} hint={values.folder}>
              <button
                onClick={() => Meetings.RevealFolder()}
                className="flex items-center gap-1.5 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[11.5px] text-soft transition-colors hover:border-accent/40 hover:text-text"
              >
                <FolderOpen size={13} /> {t("Відкрити")}
              </button>
            </Row>
          </Group>

          {said && (
            <motion.p
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              className="rounded-panel border border-line/60 bg-surface/50 px-4 py-2.5 text-[12px] leading-relaxed text-soft"
            >
              {tr(said)}
            </motion.p>
          )}

          <p className="flex items-start gap-2 text-[11px] leading-relaxed text-faint">
            <Trash2 size={12} className="mt-0.5 shrink-0" />
            {t("Усі локальні дані застосунку зберігаються у папці вище.")}
          </p>
        </div>
      </div>
    </div>
  )
}

function Group({
  title,
  Icon,
  children,
  wide = false,
  afterTitle,
}: {
  title: string
  Icon: typeof Ear
  children: React.ReactNode
  wide?: boolean
  afterTitle?: React.ReactNode
}) {
  return (
    <section className={wide ? "settings-wide" : undefined}>
      <h2
        className={`mb-2 flex items-center gap-1.5 text-[10.5px] font-semibold text-faint ${
          title === "MCP Server"
            ? "tracking-[0.04em]"
            : "uppercase tracking-wider"
        }`}
      >
        <Icon size={12} /> {title} {afterTitle}
      </h2>
      <div className="rounded-panel border border-line/60 bg-surface/40">
        {children}
      </div>
    </section>
  )
}

type MCPClient = "claude" | "codex" | "vscode"

function MCPIndicator({ state }: { state: MCPState | null }) {
  const status = {
    running: { label: t("MCP server працює"), colour: "bg-good" },
    starting: { label: t("MCP server запускається"), colour: "bg-accent" },
    failed: { label: t("MCP server недоступний"), colour: "bg-warn" },
    stopped: { label: t("MCP server зупинено"), colour: "bg-faint" },
  }[state?.status ?? "starting"]

  return (
    <span
      role="status"
      aria-label={status.label}
      title={status.label}
      className={`mcp-status-dot ml-0.5 size-1.5 rounded-full ${status.colour}`}
      data-active={
        state?.status === "running" || state?.status === "starting"
          ? "true"
          : undefined
      }
    />
  )
}

function MCPPanel({
  state,
  onError,
}: {
  state: MCPState | null
  onError: (message: string) => void
}) {
  const [client, setClient] = useState<MCPClient>("claude")
  const [copied, setCopied] = useState("")
  const url = state?.url || "http://127.0.0.1:8765/mcp"
  const command =
    state?.command ||
    "/Applications/Meeting Transcriber.app/Contents/MacOS/MeetingTranscriber"

  const configs: Record<
    MCPClient,
    { title: string; note: string; value: string }
  > = {
    claude: {
      title: "Claude Desktop",
      note: t("Локальний stdio · claude_desktop_config.json"),
      value: JSON.stringify(
        {
          mcpServers: {
            "meeting-transcriber": {
              command,
              args: ["--mcp-stdio"],
            },
          },
        },
        null,
        2,
      ),
    },
    codex: {
      title: "Codex",
      note: "Streamable HTTP · ~/.codex/config.toml",
      value: `[mcp_servers.meeting-transcriber]\nurl = "${url}"`,
    },
    vscode: {
      title: "VS Code · GitHub Copilot",
      note: "Streamable HTTP · User Profile mcp.json",
      value: JSON.stringify(
        {
          servers: {
            "meeting-transcriber": {
              type: "http",
              url,
            },
          },
        },
        null,
        2,
      ),
    },
  }
  const selected = configs[client]

  const copy = async (value: string, key: string) => {
    try {
      await copyText(value)
      setCopied(key)
      window.setTimeout(() => setCopied(""), 1600)
    } catch {
      onError(t("Не вдалося скопіювати. Виділіть текст вручну."))
    }
  }

  return (
    <div className="mcp-settings">
      <div className="mcp-meta">
        <div className="mcp-labels" aria-label={t("Доступні дані")}>
          {[
            t("Зустрічі"),
            t("Розшифровки"),
            t("Нотатки"),
            t("Проєкти"),
            t("Завдання"),
            "Read-only",
          ].map((label) => (
            <span key={label}>{label}</span>
          ))}
        </div>
        <div className="mcp-endpoint">
          <code className="min-w-0 flex-1 truncate text-[11px] text-soft">
            {url}
          </code>
          <button
            onClick={() => copy(url, "url")}
            aria-label={t("Копіювати адресу MCP")}
            className="shrink-0 rounded-md p-1.5 text-faint transition-colors hover:bg-surface hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            {copied === "url" ? <Check size={13} /> : <Copy size={13} />}
          </button>
        </div>
      </div>

      {state?.problem && (
        <p className="border-t border-line/50 px-4 py-2.5 text-[11px] leading-relaxed text-warn">
          {tr(state.problem)}.{" "}
          {t("Перезапустіть застосунок або змініть MT_MCP_ADDR.")}
        </p>
      )}

      <div className="mcp-connect border-t border-line/50">
        <div
          role="tablist"
          aria-label={t("Застосунок для підключення")}
          className="flex gap-1 border-b border-line/50 px-4 pt-3"
        >
          {(Object.keys(configs) as MCPClient[]).map((id) => (
            <button
              key={id}
              role="tab"
              aria-selected={client === id}
              onClick={() => setClient(id)}
              className={`border-b px-2.5 pb-2 text-[11px] font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent ${
                client === id
                  ? "border-accent text-text"
                  : "border-transparent text-faint hover:text-soft"
              }`}
            >
              {configs[id].title}
            </button>
          ))}
        </div>
        <div role="tabpanel" className="px-4 py-3.5">
          <p className="text-[10.5px] text-faint">
            {selected.note}
          </p>
          <div className="relative mt-2.5">
            <pre className="max-h-48 overflow-auto rounded-lg bg-ground/70 px-3 py-3 pr-11 text-[10.5px] leading-relaxed text-soft">
              <code>{selected.value}</code>
            </pre>
            <button
              onClick={() => copy(selected.value, client)}
              aria-label={
                copied === client ? t("Скопійовано") : t("Копіювати конфігурацію")
              }
              title={
                copied === client ? t("Скопійовано") : t("Копіювати конфігурацію")
              }
              className="absolute right-2 top-2 rounded-md border border-line/50 bg-surface/90 p-1.5 text-faint transition-colors hover:border-accent/40 hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              {copied === client ? <Check size={13} /> : <Copy size={13} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function Row({
  label,
  hint,
  children,
}: {
  label: string
  hint: string
  children: React.ReactNode
}) {
  const id = useId()
  return (
    <SettingLabel.Provider value={label}>
      <div role="group" aria-labelledby={id} className="settings-row">
        <div>
          <h3 id={id}>{label}</h3>
          <p>{hint}</p>
        </div>
        <div>{children}</div>
      </div>
    </SettingLabel.Provider>
  )
}

function Toggle({
  on,
  onChange,
}: {
  on: boolean
  onChange: (on: boolean) => void
}) {
  const label = useContext(SettingLabel)
  return (
    <button
      role="switch"
      aria-checked={on}
      aria-label={label}
      onClick={() => onChange(!on)}
      className={`flex h-[22px] w-10 items-center rounded-full px-0.5 transition-colors ${on ? "bg-accent" : "bg-raised"}`}
    >
      <motion.span
        layout
        transition={{ type: "spring", stiffness: 620, damping: 34 }}
        className={`size-[18px] rounded-full bg-ink ${on ? "ml-auto" : ""}`}
      />
    </button>
  )
}

function Text({
  value,
  placeholder,
  width,
  secret,
  onChange,
  onDone,
}: {
  value: string
  placeholder: string
  width: string
  secret?: boolean
  onChange: (v: string) => void
  onDone: () => void
}) {
  return (
    <input
      aria-label={useContext(SettingLabel)}
      type={secret ? "password" : "text"}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onDone}
      onKeyDown={(e) =>
        e.key === "Enter" && (e.target as HTMLInputElement).blur()
      }
      className={`${width} rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 text-[12px] outline-none transition-colors placeholder:text-faint focus:border-accent/50`}
    />
  )
}

/** A number with its unit attached, clamped where it is typed. */
function Number({
  value,
  unit,
  min,
  max,
  onChange,
}: {
  value: number
  unit: string
  min: number
  max: number
  onChange: (n: number) => void
}) {
  const label = useContext(SettingLabel)
  const [draft, setDraft] = useState(String(value))
  useEffect(() => setDraft(String(value)), [value])

  return (
    <div className="flex items-center gap-1.5 rounded-lg border border-line/60 bg-surface/60 px-2.5 py-1.5 focus-within:border-accent/50">
      <input
        aria-label={label}
        inputMode="numeric"
        value={draft}
        onChange={(e) => setDraft(e.target.value.replace(/[^\d]/g, ""))}
        onBlur={() =>
          onChange(Math.min(Math.max(parseInt(draft || "0", 10), min), max))
        }
        onKeyDown={(e) =>
          e.key === "Enter" && (e.target as HTMLInputElement).blur()
        }
        className="w-10 bg-transparent text-right text-[12px] tabular-nums outline-none"
      />
      <span className="text-[11px] text-faint">{unit}</span>
    </div>
  )
}

function Choice({
  options,
  value,
  onChange,
}: {
  options: { id: string; label: string }[]
  value: string
  onChange: (id: string) => void
}) {
  const id = useId()
  return (
    <div className="flex rounded-lg bg-raised p-0.5">
      {options.map((o) => (
        <button
          key={o.id}
          aria-pressed={value === o.id}
          onClick={() => onChange(o.id)}
          className="relative rounded-[6px] px-2.5 py-1 text-[11.5px] transition-colors"
        >
          {value === o.id && (
            <motion.span
              layoutId={id}
              className="absolute inset-0 rounded-[6px] bg-surface"
              transition={{ type: "spring", stiffness: 500, damping: 38 }}
            />
          )}
          <span
            className={`relative z-10 ${value === o.id ? "text-text" : "text-faint"}`}
          >
            {o.label}
          </span>
        </button>
      ))}
    </div>
  )
}

/**
 * One person the app has learnt.
 *
 * Closed it says the same three things it always did. Open it answers the two
 * questions the old row could not: *why* does the app think this is Marta —
 * here are the samples, play them — and *where* does she actually turn up.
 *
 * It expands in place rather than opening a panel over the list, so the row you
 * clicked stays where you clicked it.
 */
function Face({
  person,
  onChanged,
}: {
  person: Person
  onChanged: () => void
}) {
  const [open, setOpen] = useState(false)
  const [samples, setSamples] = useState<Source[] | null>(null)
  const [seen, setSeen] = useState<Group[]>([])
  const colour = colourOf(person.name, person.colour)

  // Fetched when the row is opened, not with the list: forty people would
  // otherwise mean forty queries for rows nobody looked at.
  useEffect(() => {
    if (!open || samples !== null) return
    Meetings.Samples(person.name).then((s) => setSamples((s as Source[]) ?? []))
    Meetings.Appearances(person.name).then((g) => setSeen((g as Group[]) ?? []))
  }, [open, samples, person.name])

  const paint = async (to: string) => {
    await Meetings.PaintPerson(person.name, to)
    onChanged()
  }

  return (
    <div className="border-b border-line/40 last:border-b-0">
      <div className="flex items-center gap-3 px-4 py-2.5">
        <button
          onClick={() => setOpen((o) => !o)}
          className="flex min-w-0 flex-1 items-center gap-3 text-left"
        >
          <motion.span
            animate={{ rotate: open ? 90 : 0 }}
            transition={{ duration: 0.18 }}
          >
            <ChevronRight size={13} className="shrink-0 text-faint" />
          </motion.span>
          <span
            className="size-2.5 shrink-0 rounded-full transition-transform"
            style={{ background: colour }}
          />
          <span className="min-w-0">
            <h3 className="truncate text-[13px] font-medium">{person.name}</h3>
            <p className="mt-0.5 text-[11px] text-faint">
              {person.samples} {many(person.samples, "зразок", "зразки", "зразків")} ·{" "}
              {person.meetings} {many(person.meetings, "зустріч", "зустрічі", "зустрічей")}
            </p>
          </span>
        </button>
        <button
          onClick={async () => {
            await Meetings.Forget(person.name)
            onChanged()
          }}
          title={t("Забути голос. Імена у збережених розшифровках залишаться.")}
          className="shrink-0 rounded-lg p-1.5 text-faint transition-colors hover:bg-raised hover:text-warn"
        >
          <X size={14} />
        </button>
      </div>

      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            className="overflow-hidden"
          >
            <div className="space-y-3.5 px-4 pb-4 pl-[38px]">
              <div>
                <Label>{t("Колір")}</Label>
                <div className="mt-1.5">
                  <Paint
                    colour={colour}
                    derived={tone(person.name)}
                    chosen={picked(person.colour)}
                    onPick={paint}
                  />
                </div>
              </div>

              {seen.length > 0 && (
                <div>
                  <Label>{t("У проєктах")}</Label>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    {seen.map((g) => (
                      <span
                        key={g.id}
                        style={
                          g.id === 0
                            ? undefined
                            : {
                                background: wash(
                                  colourOf(g.name, g.colour),
                                  16,
                                ),
                                color: colourOf(g.name, g.colour),
                              }
                        }
                        className={`rounded-full px-2 py-0.5 text-[10.5px] font-medium ${
                          g.id === 0 ? "bg-raised text-faint" : ""
                        }`}
                      >
                        {g.name || t("Поза проєктами")}
                        <span className="ml-1.5 tabular-nums opacity-60">
                          {g.count}
                        </span>
                      </span>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <Label>{t("Зразки голосу")}</Label>
                <div className="mt-1 -ml-1">
                  {samples === null ? (
                    <p className="px-1 py-1 text-[11.5px] text-faint">
                      {t("Завантажую…")}
                    </p>
                  ) : samples.length === 0 ? (
                    <p className="px-1 py-1 text-[11.5px] leading-relaxed text-faint">
                      {t(
                        "Для старих зразків джерела ще немає. Воно з’явиться після наступного розпізнавання.",
                      )}
                    </p>
                  ) : (
                    samples.map((src, i) => (
                      <Snippet
                        key={`${src.recording}-${i}`}
                        source={src}
                        colour={colour}
                      />
                    ))
                  )}
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-[10px] font-medium uppercase tracking-[0.08em] text-faint">
      {children}
    </span>
  )
}

/**
 * One project: its name, and the colour it wears everywhere else.
 *
 * The name is the field, the same way a meeting's title is — a project called
 * "northwind2" because that is what got typed at 9am should not need a menu to
 * become "Northwind".
 */
function Folder({ group, onChanged }: { group: Group; onChanged: () => void }) {
  const [name, setName] = useState(group.name)
  const [picking, setPicking] = useState(false)
  const colour = colourOf(group.name, group.colour)

  useEffect(() => setName(group.name), [group.name])

  const paint = async (to: string) => {
    await Meetings.Paint(group.id, to)
    onChanged()
  }

  return (
    <div className="border-b border-line/40 px-4 py-2.5 last:border-b-0">
      <div className="flex items-center gap-3">
        <button
          onClick={() => setPicking((p) => !p)}
          title={t("Колір проєкту")}
          style={{ background: colour }}
          className="size-2.5 shrink-0 rounded-full transition-transform hover:scale-125"
        />
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            e.stopPropagation()
            if (e.key === "Enter") e.currentTarget.blur()
            if (e.key === "Escape") {
              setName(group.name)
              e.currentTarget.blur()
            }
          }}
          onBlur={async () => {
            const to = name.trim()
            if (!to || to === group.name) return setName(group.name)
            await Meetings.RenameGroup(group.id, to)
            onChanged()
          }}
          spellCheck={false}
          className="-mx-1.5 min-w-0 flex-1 rounded-lg bg-transparent px-1.5 py-0.5 font-[inherit] text-[13px] font-medium text-text outline-none transition-colors hover:bg-raised/50 focus:bg-raised"
        />
        <span className="shrink-0 text-[11px] tabular-nums text-faint">
          {group.count} {many(group.count, "зустріч", "зустрічі", "зустрічей")}
        </span>
        <button
          onClick={async () => {
            await Meetings.DropGroup(group.id)
            onChanged()
          }}
          title={t("Видалити проєкт. Зустрічі залишаться, нотатки проєкту буде видалено.")}
          className="shrink-0 rounded-lg p-1.5 text-faint transition-colors hover:bg-raised hover:text-warn"
        >
          <X size={14} />
        </button>
      </div>

      <AnimatePresence initial={false}>
        {picking && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="overflow-hidden"
          >
            <div className="pb-1 pl-[22px] pt-2.5">
              <Paint
                colour={colour}
                derived={tone(group.name)}
                chosen={picked(group.colour)}
                onPick={paint}
              />
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
