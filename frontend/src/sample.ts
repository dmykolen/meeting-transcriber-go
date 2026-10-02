// Sample data for working on the interface without the models.
//
// Only reachable when the Wails bridge is absent AND the build is a development
// one: `import.meta.env.DEV` is a compile-time constant, so Vite removes this
// file entirely from a production bundle. Nothing here ever ships.
import type {
  Answer,
  Meeting,
  Recording,
  Settings,
  Sticky,
  Action,
  Summary,
} from "./api"

const summary = {
  title: "Скрипти, безпека і Northwind",
  overview:
    "Коротко пройшлися по поточних задачах команди: Софія працює над скриптами для витягування даних, Тарас — над архітектурними й безпековими питаннями, а Марта — над пакетом документів для Northwind.",
  chapters: [
    { start: 282, title: "Скрипти та дані", summary: "" },
    { start: 441, title: "Безпека та доступ", summary: "" },
    { start: 733, title: "Матеріали для Northwind", summary: "" },
  ],
  topics: ["скрипти", "безпека", "Northwind"],
  decisions: [
    "Продукт не закривають повністю, а обмежують для зовнішнього світу; доступ лишиться через VPN.",
    "Питання з безпекою планують закрити до 3-го числа.",
  ],
  action_items: [
    {
      task: "Продовжити розбиратися зі скриптами для витягування даних",
      owner: "Sofia",
      due: "",
      done: false,
    },
    {
      task: "Додати матеріали відповіді на запитання Northwind",
      owner: "Marta",
      due: "сьогодні",
      done: false,
    },
  ],
  open_questions: ["Які саме IP-діапазони треба вказати для обмежень доступу?"],
}

/** How many meetings the project document has read; a rebuild walks it up. */
let folded = 12

const recordings: Recording[] = [
  {
    id: 1,
    kind: "meeting",
    audio: "sample-1.wav",
    group: 0,
    title: summary.title,
    started: new Date(Date.now() - 3e6).toISOString(),
    duration: 1826,
    language: "uk",
    status: "done",
    progress: 1,
    turns: 153,
    speakers: ["Marta", "Sofia", "Taras Petrenko"],
    summary,
  },
  {
    id: 2,
    kind: "meeting",
    audio: "sample-2.wav",
    group: 0,
    title: "Планування спринту 14",
    started: new Date(Date.now() - 9e7).toISOString(),
    duration: 2387,
    language: "uk",
    status: "summarising",
    progress: 0.9,
    turns: 210,
    speakers: ["Marta", "Ostap"],
  },
  {
    id: 3,
    kind: "note",
    audio: "sample-3.wav",
    group: 0,
    title: "note 2026-09-01 08:12.wav",
    started: new Date(Date.now() - 1.8e8).toISOString(),
    duration: 214,
    language: "uk",
    status: "transcribing",
    progress: 0.35,
    turns: 0,
  },
  {
    id: 4,
    kind: "meeting",
    audio: "sample-4.wav",
    group: 0,
    title: "meeting 2026-08-30 22:39.wav",
    started: new Date(Date.now() - 2.6e8).toISOString(),
    duration: 0,
    language: "",
    status: "failed",
    progress: 0,
    turns: 0,
    problem: "could not read the audio: no data chunk",
  },
  {
    id: 5,
    kind: "meeting",
    audio: "sample-5.wav",
    group: 0,
    title: "meeting 2026-09-28 17:31.wav",
    started: new Date(Date.now() - 4e6).toISOString(),
    duration: 3677,
    language: "",
    status: "queued",
    progress: 0,
    turns: 0,
  },
]
const rushed = new Set<number>()

const transcript = [
  { start: 42, end: 44, speaker: "Marta", text: "Привіт, привіт." },
  {
    start: 55,
    end: 59,
    speaker: "Marta",
    text: "Софія, ти вже повернулася з відпустки?",
  },
  {
    start: 72,
    end: 80,
    speaker: "Sofia",
    text: "Так, з понеділка. Навіть встигла розібрати пошту.",
  },
  {
    start: 282,
    end: 291,
    speaker: "Sofia",
    text: "Вчора розбиралася з тестовою базою, сьогодні вже почала писати скрипти.",
  },
  {
    start: 441,
    end: 452,
    speaker: "Taras Petrenko",
    text: "Продукт повністю не закриваємо — обмежуємо для зовнішнього світу, доступ лишається через VPN.",
  },
]

const notes: Sticky[] = [
  {
    id: 1,
    recording: 1,
    project: 0,
    text: "Перевірити доступ через VPN перед демонстрацією.\n\nНе забути зовнішню команду.",
    colour: "lime",
    at: 441,
  },
  {
    id: 2,
    recording: 1,
    project: 0,
    text: "Попросити Софію показати контрольну вибірку — важливо звірити джерела.",
    colour: "blue",
    at: 282,
  },
  {
    id: 3,
    recording: 1,
    project: 0,
    text: "Ідея для наступної зустрічі: пройти весь шлях нового користувача.",
    colour: "pink",
    at: 0,
  },
  {
    id: 4,
    recording: 1,
    project: 0,
    text: "Мої питання\nХто погоджує IP-діапазони?\nКоли фінальний перегляд?",
    colour: "orange",
    at: 0,
  },
  {
    id: 5,
    recording: 0,
    project: 1,
    text: "Підготувати матеріали для спільного рев’ю.",
    colour: "blue",
    at: 0,
  },
]
let nextNote = 6
const deleted = new Set<number>()
let held = false
export const sample = {
  SearchKnowledge: async (q: string, semantic: boolean) =>
    [
      ...transcript.map((t, i) => ({
        key: "t" + i,
        kind: "transcript",
        recording: 1,
        project: 0,
        note: 0,
        title: summary.title,
        start: t.start,
        text: t.text,
      })),
      ...notes.map((n) => ({
        key: "n" + n.id,
        kind: "note",
        recording: n.recording,
        project: n.project,
        note: n.id,
        title: n.project ? "Northwind" : summary.title,
        start: 0,
        text: n.text,
      })),
      {
        key: "s1",
        kind: "summary",
        recording: 1,
        project: 0,
        note: 0,
        title: summary.title,
        start: 0,
        text: summary.overview,
      },
      {
        key: "p1",
        kind: "project",
        recording: 0,
        project: 1,
        note: 0,
        title: "Northwind",
        start: 0,
        text: "Доступ лишається через VPN. Матеріали готуються до спільного рев’ю.",
      },
    ].filter((h) => semantic || h.text.toLowerCase().includes(q.toLowerCase())),
  AskKnowledge: async (q: string) => {
    await new Promise((r) => setTimeout(r, 650))
    return {
      text: "Доступ до продукту залишили через VPN. У нотатках є нагадування перевірити цей доступ перед демонстрацією, а в проєкті — підготувати матеріали до рев’ю.",
      sources: (await sample.SearchKnowledge(q, true)).slice(3, 9),
    }
  },
  Playing: async () => {},
  Notes: async (r: number, p: number) =>
    notes
      .filter((n) => (r ? n.recording === r : n.project === p))
      .map((n) => ({ ...n })),
  PutNote: async (n: Sticky) => {
    const saved = { ...n, id: n.id || nextNote++ }
    const i = notes.findIndex((x) => x.id === saved.id)
    if (i >= 0) notes[i] = saved
    else notes.push(saved)
    return { ...saved }
  },
  RemoveNote: async (id: number) => {
    const i = notes.findIndex((n) => n.id === id)
    if (i >= 0) notes.splice(i, 1)
  },
  EditAction: async (id: number, index: number, a: Action) => {
    const r = recordings.find((r) => r.id === id)
    if (!r) return
    r.summary ??= {
      title: r.title,
      overview: "",
      chapters: [],
      decisions: [],
      action_items: [],
      open_questions: [],
      topics: [],
    }
    r.summary.action_items ??= []
    if (index < 0) r.summary.action_items.push({ ...a })
    else r.summary.action_items[index] = { ...a }
  },
  ExactSearch: async (q: string) => sample.Search(q),
  PreviewSummary: async () => {
    await new Promise((r) => setTimeout(r, 750))
    return {
      ...summary,
      overview:
        "Узгодили доступ до продукту через VPN. Софія продовжує роботу над скриптами, Марта готує пакет матеріалів для Northwind. IP-діапазони ще потребують уточнення.",
    }
  },
  AcceptSummary: async (id: number, _before: Summary, after: Summary) => {
    const r = recordings.find((r) => r.id === id)
    if (r) r.summary = structuredClone(after)
  },
  Recent: async () => recordings.filter((r) => !deleted.has(r.id)),
  Open: async (id: number): Promise<Meeting> => {
    const r = recordings.find((r) => r.id === id) ?? recordings[0]
    const until = new Date()
    until.setHours(19, 0, 0, 0)
    const queued = r.status === "queued"
    return {
      ...r,
      transcript: queued ? [] : transcript,
      wait: !queued ? "" : rushed.has(r.id) ? "next" : "time",
      until: until.toISOString(),
    }
  },
  Rush: async (id: number) => {
    rushed.add(id)
  },
  Search: async (q: string) =>
    [
      {
        recording: 1,
        title: summary.title,
        start: 441,
        speaker: "Taras Petrenko",
        text: transcript[4].text,
      },
      {
        recording: 1,
        title: summary.title,
        start: 282,
        speaker: "Sofia",
        text: transcript[3].text,
      },
    ].filter((h) =>
      h.text
        .toLowerCase()
        .includes(String(q).toLowerCase().split(/\s+/).pop() ?? ""),
    ),
  Ask: async (): Promise<Answer> => ({
    text: "Домовилися не закривати продукт повністю, а обмежити доступ для зовнішнього світу через VPN. Питання з безпекою планують закрити до 3-го числа.",
    sources: [
      {
        recording: 1,
        title: summary.title,
        start: 441,
        speaker: "Taras Petrenko",
        text: transcript[4].text,
      },
      {
        recording: 1,
        title: summary.title,
        start: 282,
        speaker: "Sofia",
        text: transcript[3].text,
      },
    ],
  }),
  Rename: async () => {},
  Paint: async () => {},
  Loose: async () => 12,
  Span: async () =>
    recordings.map((r) => ({
      id: r.id,
      kind: r.kind,
      started: r.started,
      duration: r.duration,
      folder: r.group,
    })),
  TickItem: async () => {},
  PinItem: async () => {},
  // A rebuild in design mode takes the time a real one feels like, so the
  // waiting state is something that can actually be looked at.
  RebuildProject: async () => {
    for (folded = 0; folded < 12; folded++)
      await new Promise((r) => setTimeout(r, 600))
  },
  Moment: async (_id: number, text: string) => (text.length % 6) * 120 + 180,
  Standing: async () => ({
    written: folded > 0,
    folded,
    status:
      folded > 0
        ? "Міграцію узгоджено, чекає на ревʼю безпеки. Доступ лишається через VPN — рішення від 5 вересня скасувало попереднє. Дві речі прострочені, обидві на боці безпеки."
        : "",
    meetings: 12,
    hours: 8.4,
    first: recordings[3].started,
    last: recordings[0].started,
    work: [
      {
        item: 1,
        state: "open",
        by: "",
        pinned: false,
        text: "Узгодити перелік ролей із безпекою",
        owner: "Taras",
        due: "",
        done: false,
        times: 3,
        from: 1,
        index: -1,
        when: recordings[0].started,
      },
      {
        item: 2,
        state: "done",
        by: "",
        pinned: true,
        text: "Закрити доступ ззовні",
        owner: "Sofia",
        due: "",
        done: true,
        times: 1,
        from: 2,
        index: -1,
        when: recordings[1].started,
      },
    ],
    decisions: [
      {
        item: 3,
        state: "standing",
        by: "",
        pinned: false,
        text: "Доступ лишається через VPN",
        owner: "",
        due: "",
        done: false,
        times: 2,
        from: 1,
        index: -1,
        when: recordings[0].started,
      },
      {
        item: 4,
        state: "overturned",
        by: "Доступ лишається через VPN",
        pinned: false,
        text: "Відкрити продукт назовні",
        owner: "",
        due: "",
        done: true,
        times: 1,
        from: 2,
        index: -1,
        when: recordings[1].started,
      },
    ],
    questions: [
      {
        item: 5,
        state: "open",
        by: "",
        pinned: false,
        text: "Які IP-діапазони віддаємо назовні",
        owner: "",
        due: "",
        done: false,
        times: 3,
        from: 2,
        index: -1,
        when: recordings[1].started,
      },
    ],
    people: [
      {
        name: "Taras Petrenko",
        seconds: 5400,
        meetings: 9,
        last: recordings[0].started,
      },
      {
        name: "Sofia",
        seconds: 2100,
        meetings: 4,
        last: recordings[2].started,
      },
    ],
  }),
  PaintPerson: async () => {},
  RenameGroup: async () => {},
  Samples: async () => [
    {
      recording: 1,
      speaker: "Sofia",
      title: summary.title,
      audio: "sample-1.wav",
      start: 282,
      finish: 301,
    },
    {
      recording: 2,
      speaker: "Sofia",
      title: "Планування спринту 14",
      audio: "sample-2.wav",
      start: 40,
      finish: 66,
    },
  ],
  Appearances: async () => [
    { id: 1, name: "Northwind", count: 8, colour: "" },
    { id: 0, name: "", count: 2, colour: "" },
  ],
  // Mutates the row so that renaming can actually be judged in design mode
  // rather than snapping back to the model's title on the next read.
  Retitle: async (id: number, title: string) => {
    const r = recordings.find((x) => x.id === id)
    if (r) r.title = title
  },
  SaveNote: async (id: number, note: string) => {
    const r = recordings.find((r) => r.id === id)
    if (r) r.note = note
  },
  Delete: async (id: number) => {
    deleted.add(id)
  },
  Import: async () => recordings[0],
  RevealFolder: async () => {},
  Settings: async (): Promise<Settings> => ({
    language: "uk",
    transcriber: "whisper" as const,
    openaiKey: "sk-demo",
    openaiModel: "gpt-5.4-mini",
    summarise: "meetings" as const,
    keepNotes: false,
    density: "compact",
    listening: true,
    system: true,
    startSpeech: 20,
    quietEnds: 180,
    preroll: 300,
    keepAudioDays: 30,
    aiProvider: "copilot" as const,
    copilotModel: "",
    localModel: "",
    transcribe: "at" as const,
    transcribeAt: "19:00",
    uiLanguage: "uk" as const,
    embeddings: "local" as const,
    folder: "/Users/you/MeetingTranscriber",
  }),
  AIStatus: async () => ({
    ready: true,
    searchable: false,
    fetching: "Qwen3 Embedding 0.6B",
    fraction: 0.42,
    indexing: "",
    indexed: 0,
    signingIn: false,
    said: [],
    problem: "",
  }),
  Copilot: async () => ({
    login: "octocat",
    models: [
      { id: "gpt-5-mini", name: "GPT-5 mini" },
      { id: "gpt-5.4-mini", name: "GPT-5.4 mini" },
    ],
  }),
  ConnectCopilot: async () => {},
  Hold: async (on: boolean) => {
    held = on
  },
  Actions: async (done: boolean) =>
    recordings
      .filter((r) => !deleted.has(r.id))
      .flatMap((r) =>
        (r.summary?.action_items ?? []).map((a, index) => ({
          ...a,
          recording: r.id,
          index,
          title: r.title,
          started: r.started,
        })),
      )
      .filter((a) => done || !a.done),
  Tick: async (id: number, index: number, done: boolean) => {
    const a = recordings.find((r) => r.id === id)?.summary?.action_items?.[
      index
    ]
    if (a) a.done = done
  },
  Markdown: async () => "# " + summary.title + "\n\n" + summary.overview,
  SaveSettings: async () => {},
  State: async () => ({
    stage: "ready" as const,
    what: "",
    fraction: 1,
    done: 0,
    total: 0,
  }),

  // The strip is only there while something records.
  Listening: () => ({
    phase: location.hash === "#strip" ? (held ? "held" : "recording") : "listening",
    asked: false,
    kind: "meeting",
    elapsed: 0,
    quiet: 0,
    system: true,
    problem: "",
  }),
  Summaries: () => true,
  MCPStatus: async () => ({
    status: "running" as const,
    url: "http://127.0.0.1:8765/mcp",
    command:
      "/Applications/Meeting Transcriber.app/Contents/MacOS/MeetingTranscriber",
    problem: "",
  }),
  Waveform: async () =>
    Array.from({ length: 300 }, (_, i) =>
      Math.max(
        0,
        0.35 +
          0.4 * Math.sin(i / 7) +
          0.25 * Math.sin(i / 2.3) * (i % 11 < 7 ? 1 : 0.2),
      ),
    ),
  Groups: async () => [
    { id: 1, name: "Northwind", count: 8 },
    { id: 2, name: "AI platform", count: 5 },
  ],
  NewGroup: async (name: string) => ({ id: 3, name, count: 0 }),
  File: async () => {},
  DropGroup: async () => {},
  InGroup: async () => [],
  Bin: async () => recordings.filter((r) => deleted.has(r.id)),
  Restore: async (id: number) => {
    deleted.delete(id)
  },
  EmptyBin: async () => {
    for (let i = recordings.length - 1; i >= 0; i--)
      if (deleted.has(recordings[i].id)) recordings.splice(i, 1)
    deleted.clear()
    return "Кошик очищено"
  },
  Again: async () => {},
  ThisIsMe: async () => 6,
  Reindex: async () => ({ meaning: 486, words: 0 }),
  Tidy: async () => ({ files: 3, mb: 412, kept: false }),

  Analytics: async (id: number) =>
    id === 3
      ? {
          speech: 190,
          silence: 24,
          overlap: 0,
          words: 402,
          pace: 127.0,
          balance: 0,
          speakers: [
            {
              speaker: "You",
              seconds: 190,
              share: 1,
              turns: 31,
              longest: 41,
              words: 402,
              pace: 127,
              questions: 2,
            },
          ],
          busiest: Array.from({ length: 48 }, (_, i) => ({
            at: i * 4.5,
            words: Math.round(20 + 14 * Math.sin(i / 3)),
          })),
        }
      : {
          speech: 1490,
          silence: 310,
          overlap: 46,
          words: 3120,
          pace: 125.6,
          balance: 0.87,
          speakers: [
            {
              speaker: "Marta",
              seconds: 700,
              share: 0.45,
              turns: 61,
              longest: 92,
              words: 1520,
              pace: 130.3,
              questions: 14,
            },
            {
              speaker: "Sofia",
              seconds: 540,
              share: 0.35,
              turns: 48,
              longest: 71,
              words: 1080,
              pace: 120.0,
              questions: 5,
            },
            {
              speaker: "Taras Petrenko",
              seconds: 310,
              share: 0.2,
              turns: 44,
              longest: 38,
              words: 520,
              pace: 100.6,
              questions: 9,
            },
          ],
          busiest: Array.from({ length: 48 }, (_, i) => ({
            at: i * 37.5,
            words: Math.round(40 + 55 * Math.sin(i / 4) + (i % 5) * 9),
          })),
        },

  People: async () => [
    { id: 1, name: "Marta", samples: 6, meetings: 12 },
    { id: 2, name: "Sofia", samples: 4, meetings: 9 },
    { id: 3, name: "Taras Petrenko", samples: 8, meetings: 21 },
  ],
  Forget: async () => {},
  Summarise: async () => {},

  Live: () => [
    {
      at: 12,
      who: "them",
      text: "Софія, ти вже повернулася з відпустки?",
    },
    { at: 21, who: "you", text: "Так, з понеділка. Я вже дивлюся на скрипти." },
    {
      at: 34,
      who: "them",
      text: "Добре, тоді давай о четвертій зберемося і подивимося баги.",
    },
  ],

  Brief: async (days: number) => ({
    since: new Date(Date.now() - days * 864e5).toISOString(),
    minutes: 148,
    skipped: 7,
    spared: 63,
    meetings: [],
    voices: ["Taras Petrenko", "Marta", "Sofia"],
    decided: [
      {
        recording: 1,
        title: summary.title,
        started: new Date(Date.now() - 3e6).toISOString(),
        text: "Продукт не закривають повністю, а обмежують для зовнішнього світу; доступ лишиться через VPN.",
      },
      {
        recording: 1,
        title: summary.title,
        started: new Date(Date.now() - 3e6).toISOString(),
        text: "Питання з безпекою планують закрити до 3-го числа.",
      },
    ],
    mine: [
      {
        recording: 1,
        title: summary.title,
        started: new Date(Date.now() - 3e6).toISOString(),
        index: 0,
        task: "Продовжити розбиратися зі скриптами для витягування даних",
        owner: "Sofia",
        due: "",
        done: false,
      },
      {
        recording: 1,
        title: summary.title,
        started: new Date(Date.now() - 3e6).toISOString(),
        index: 1,
        task: "Додати матеріали відповіді на запитання Northwind",
        owner: "Marta",
        due: "сьогодні",
        done: false,
      },
    ],
    overdue: [
      {
        recording: 2,
        title: "Планування спринту 14",
        started: new Date(Date.now() - 5 * 864e5).toISOString(),
        index: 0,
        task: "Узгодити перелік ролей із безпекою",
        owner: "Taras Petrenko",
        due: "цього тижня",
        done: false,
      },
    ],
    nagging: [
      {
        text: "Які саме IP-діапазони треба вказати для обмежень доступу?",
        times: 3,
        said: [
          {
            recording: 1,
            title: summary.title,
            started: new Date(Date.now() - 3e6).toISOString(),
            text: "",
          },
          {
            recording: 2,
            title: "Планування спринту 14",
            started: new Date(Date.now() - 5 * 864e5).toISOString(),
            text: "",
          },
          {
            recording: 3,
            title: "Планування спринту",
            started: new Date(Date.now() - 12 * 864e5).toISOString(),
            text: "",
          },
        ],
      },
    ],
  }),
  Record: () => undefined,
}

// Stress fixtures are development-only, for reviewing real responsive components.
if (new URLSearchParams(location.search).has("stress")) {
  const people = [
    "Марта",
    "Софія",
    "Тарас",
    "Марія",
    "Андрій",
    "Софія",
    "Остап",
    "Юлія",
    "Сергій",
    "Катерина",
  ]
  recordings[0].speakers = people
  transcript.splice(
    0,
    transcript.length,
    ...Array.from({ length: 120 }, (_, i) => ({
      start: i * 12,
      end: i * 12 + 9,
      speaker: people[i % people.length],
      text: `${i + 1}. Перевіряємо зміни в доступі та підготовку документів. Важливо узгодити це з командою до наступної зустрічі.`,
    })),
  )
  sample.Groups = async () =>
    Array.from({ length: 24 }, (_, i) => ({
      id: i + 1,
      name: i === 0 ? "Northwind" : `Проєкт ${String(i + 1).padStart(2, "0")}`,
      count: i + 3,
    }))
  for (let i = 0; i < 8; i++)
    notes.push({
      id: nextNote++,
      recording: 1,
      project: 0,
      text: `Додаткова нотатка ${i + 1}. Зберегти контекст і перевірити домовленості.`,
      colour: ["lime", "blue", "pink", "orange"][i % 4],
      at: 0,
    })
}
