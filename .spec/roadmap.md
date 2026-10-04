# Roadmap

Product work, grouped by kind. Done rows stay, struck through.

Before adding a row, read every table. When a similar row exists, update it
with the new detail instead of adding another. Ideas that must not come back
are in the last table.

**Priority:** P1 next, P2 soon, P3 later or waiting for evidence or the owner.
**Complexity** (rough): XS hours, S a day, M a few days, L a week or two, XL
more.

## Bug fixes

| Item | Details | Priority | Complexity | State |
|---|---|---|---|---|
| Voice recognition after the speaker-model switch | Found by `exp/20_voices` (2026-09-29). Voices enrolled before the switch keep only 512-wide prints, and `store.Cosine` returns -1 across widths, so those people are silently never recognised again (two regular colleagues on the owner's archive); flagging them in Settings for one new naming may be enough. The recogniser self-enrols its matches at `Match` 0.55, which 13% of different-people pairs clear, and three enrolled centres are already 0.78–0.84 alike, so names may drift: measure against owner-confirmed names before changing it. `mine()` can leave the owner's print identical to a far-side label's (10 meetings). | P1 | M | next |
| Local-model project updates lose every operation | Found 2026-10-02. With Gemma 4 E2B a project received 9, 5 and 7 operations but kept no work, decisions or questions, only the status. `do` and `kind` are free text in the schema, and `Kept.Apply` silently skips anything but the exact lowercase values. Constrain them to enums and log what was ignored. | P2 | S | planned |
| ~~GitHub Copilot does not summarise~~ | ~~Fixed in 1.2.1. `copilot login` kept the token in the macOS keychain, which the SDK's `ModeEmpty` runtime never reads; it was found only through the CLI's `gh` fallback, absent from an app opened in Finder or the Dock. The login now runs with `COPILOT_DISABLE_KEYTAR=1` and `storeTokenPlaintext` in the Copilot home's `settings.json`, so the token lands in its `config.json`; a signed-out request says where to connect. A first summary accepted by hand now titles an untitled recording and moves its project, as an automatic one does, so recordings left without one recover fully. Checked in the app with a Finder `PATH`: sign-in, account, summary, title and project.~~ | ~~P1~~ | ~~M~~ | ~~done~~ |
| ~~Copy and Copy Markdown fail almost everywhere~~ | ~~Fixed in 1.2.1. The four copy actions (MCP setup in `Settings.tsx`; Markdown, one turn and a selection in `Transcript.tsx`) used `navigator.clipboard` and now write through Wails' clipboard (`copyText` in `api.ts`); a failure shows in the reader. Checked in the app: each one puts the expected text on the clipboard.~~ | ~~P1~~ | ~~S~~ | ~~done~~ |
| ~~Voice stats stale after renaming a speaker~~ | ~~"Voices and pace" (`Shape.tsx`) kept the old labels and colours until the meeting was reopened. Shipped in 1.2.1.~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |
| ~~MCP output schema missed the recording turn count~~ | ~~Shipped in 1.2.0.~~ | ~~P1~~ | ~~XS~~ | ~~done~~ |
| ~~Wrong plurals on Today and in the meeting preview~~ | ~~Shipped in 1.2.0.~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |
| ~~Ukrainian task sources in the English Ask~~ | ~~Shipped in 1.2.0: a task reads "text · owner · due · open/done" in both languages.~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |

## New features

| Item | Details | Priority | Complexity | State |
|---|---|---|---|---|
| Vocabulary and transcript correction | Let a person correct names, products, and technical terms, and reuse the corrections on later meetings. The prompt-only glossary experiment (`exp/18_glossary`) lowered accuracy and needed the context mode that loops; this needs a new measured approach. | P2 | L | planned |
| Dictation mode | One button and hotkey start and stop. After stopping, choose a project or leave the note unfiled. Optionally clean the dictated text without making AI mandatory for capture. | P2 | M | planned |
| Name recurring voices once, across the archive | Unnamed voices speak 53% of the time, and 114 action items are owned by `SPEAKER_xx` (109 meetings, 2026-09-28). `exp/20_voices` found one name per cluster unsafe at every threshold: two of the five largest groups hold other people's named voices. Workable only as confirm-by-ear, then apply at 0.70 or above, with the rest as suggestions. Needs the voice-recognition fix first. | P3 | L | blocked |
| The strip reacts to the owner's name | Others say the owner's name in 47 of 109 meetings: 185 turns, 35 of them questions (namesakes included). Highlight the sentence with the name and the line before it. Risk: the live scribe drops an utterance while the previous one is still being transcribed; first replay recorded meetings through the live path and measure recall and delay. | P3 | M | owner to decide |
| "What did I miss?" in the strip | Three lines on the last five minutes of the live transcript, through the chosen AI. With the previous row it gives the context of the question. Risk: rough live text and a weak small local model; measure on the same replays. | P3 | M | owner to decide |
| Memory during a meeting (a bet) | When the conversation returns to a topic with a past decision, the strip shows that decision with a link. No evidence yet: replay past meetings in order and count useful against noisy hints before building anything. | P3 | L | owner to decide |
| Decision history and a project time machine | | P3 | L | after evidence |
| Durable markers during a live recording | | P3 | S | after evidence |
| Meeting-efficiency insights | Based on measured participation and assignments. | P3 | M | after evidence |
| ~~Built-in read-only MCP server and client setup~~ | | | | ~~done~~ |
| ~~Living project state: stable IDs, rebuild, provenance, pinned edits~~ | | | | ~~done~~ |
| ~~Editable action items~~ | | | | ~~done~~ |
| ~~Command palette~~ | | | | ~~done~~ |
| ~~Exact and semantic search across meetings, projects, and notes~~ | | | | ~~done~~ |
| ~~Ask with typed sources and direct navigation to evidence~~ | | | | ~~done~~ |
| ~~Note deck and agenda notes~~ | | | | ~~done~~ |
| ~~Summary preview, comparison, acceptance, and undo~~ | | | | ~~done~~ |
| ~~Bin, restore, and empty bin~~ | | | | ~~done~~ |
| ~~AI from OpenAI, GitHub Copilot, or a local model; local search vectors~~ | ~~Shipped in 1.1.0.~~ | | | ~~done~~ |
| ~~Recording strip: tell the others, pause, stop~~ | ~~Shipped in 1.1.0.~~ | | | ~~done~~ |
| ~~Transcription schedule: after a recording, at a set time, or when the Mac is free~~ | ~~Shipped in 1.1.0.~~ | | | ~~done~~ |
| ~~Interface in Ukrainian and English~~ | ~~Shipped in 1.2.0.~~ | | | ~~done~~ |

## Improvements

| Item | Details | Priority | Complexity | State |
|---|---|---|---|---|
| Local model quality | The default, Gemma 4 E2B, is small and unmeasured: its first run chaptered everything at 0 s and dropped a spoken deadline. Measure current candidates (Gemma 4, Qwen3.5+, the Ukrainian MamayLM v2) on several anonymised real transcripts, project-update operations included. See [the measured spike](spikes/performance-local-llm-spike.md). | P2 | M | planned |
| Structured provenance for each summary claim | | P3 | L | after evidence |
| Word-level playback | Only after mapped token timestamps are proven; see "Whisper context and timestamps" in [decisions](decisions.md). | P3 | M | after evidence |
| Names in the live transcript | | P3 | M | after evidence |
| Better mic-only recognition of in-person meetings | | P3 | L | after evidence |
| Very large archives | Search benchmarks, pagination, and virtualization. | P3 | M | after evidence |
| ~~Combined Meetings workspace: timeline, reader, voice map, project dock~~ | | | | ~~done~~ |
| ~~Responsive drawers, focus mode, and reduced motion~~ | | | | ~~done~~ |
| ~~Background semantic-index progress with named phases~~ | | | | ~~done~~ |

## Do not propose again

| Idea | Why |
|---|---|
| Two microphones in one room | |
| Redaction before the model | |
| A generated follow-up message | It duplicates the summary and the Markdown export. |
| SRT, VTT, or DOCX export | |
| A warning that a decision affects someone who was not told | |
| A dashboard that scores who a project is waiting on | |
| Invented project health, velocity, burndown, or Gantt data | |
| Developer ID signing and notarization | Owner's decision. |
| Keeping the API keys in the Keychain | Owner's decision. |
| Guessing speaker names from how people address each other | First names are in 2% of turns, and 2 of 445 unnamed voices get a consistent candidate. |
| Detecting "off the record" phrases | None in 109 meetings. |
| Deadline reminders for action items | 22 of 369 items have a due date, 4 of them the owner's. |
| Closing action items from what later meetings say | The project fold already can, and it closed 1 of 132. |
