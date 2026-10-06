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
| Local-model project updates lose every operation | Found 2026-10-02. With Gemma 4 E2B a project received 9, 5 and 7 operations but kept no work, decisions or questions, only the status. `do` and `kind` are free text in the schema, and `Kept.Apply` silently skips anything but the exact lowercase values. The schema now constrains `do` and `kind` to enums and the log counts what was ignored (`ignored=`). Re-run the project replay with Gemma 4 E2B to see whether operations still go missing. | P2 | S | enums added, to re-measure |
| ~~The meeting menu (⋯) stays open after a click elsewhere~~ | ~~Shipped after 1.3.0: a pointer press anywhere outside the menu closes it, as Escape does.~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |
| Switching listening off mid-recording leaves the detector recording | Code reading, 2026-10-04, not reproduced. `Recorder.Pause` (the Settings switch) finishes the file from the Wails goroutine, racing the capture loop, and leaves the detector recording: a recording started by hand would then show as running after listening is switched back on, with nothing written. The private pause does this inside the loop (`hush`); route the switch the same way. | P2 | XS | planned |
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
| ~~AI usage and cost~~ | ~~Shipped after 1.3.0. Every model call, embeddings included, is kept in a `usage` table (task, provider, model, tokens, seconds, failure) and Settings shows 30 days: a bar a day, then a line per model with requests, tokens, time and cost. OpenAI is priced from a list in `insights/price.go`; an unknown model shows tokens only; Copilot shows the AI Credits its events report; a local model shows time. Not yet checked against a real billed Copilot call (quota spent): whether 1 AIU is 1 AI Credit.~~ | ~~P2~~ | ~~M~~ | ~~done~~ |
| Name recurring voices once, across the archive | Unnamed voices speak 53% of the time, and 114 action items are owned by `SPEAKER_xx` (109 meetings, 2026-09-28). `exp/20_voices` found one name per cluster unsafe at every threshold: two of the five largest groups hold other people's named voices. Workable only as confirm-by-ear, then apply at 0.70 or above, with the rest as suggestions. Needs the voice-recognition fix first. | P3 | L | blocked |
| The strip reacts to the owner's name | Others say the owner's name in 47 of 109 meetings: 185 turns, 35 of them questions (namesakes included). Highlight the sentence with the name and the line before it. Risk: the live scribe drops an utterance while the previous one is still being transcribed; first replay recorded meetings through the live path and measure recall and delay. | P3 | M | owner to decide |
| "What did I miss?" in the strip | Three lines on the last five minutes of the live transcript, through the chosen AI. With the previous row it gives the context of the question. Risk: rough live text and a weak small local model; measure on the same replays. | P3 | M | owner to decide |
| Memory during a meeting (a bet) | When the conversation returns to a topic with a past decision, the strip shows that decision with a link. No evidence yet: replay past meetings in order and count useful against noisy hints before building anything. | P3 | L | owner to decide |
| Decision history and a project time machine | | P3 | L | after evidence |
| Durable markers during a live recording | | P3 | S | after evidence |
| Meeting-efficiency insights | Based on measured participation and assignments. | P3 | M | after evidence |
| ~~Pause listening for a while, from the record button~~ | ~~Shipped in 1.3.0. Pointing at or focusing the record button pulls out a drawer: a beam leaves the button, a glass capsule unrolls, the controls blur in one by one, a lit plate glides under the pointer, and arcs show each pause's share of an hour. It holds Record (⌘R) and "don't record" for 5, 15 or 60 minutes; while paused the button counts down inside a fading ring and resumes on a click. A recording in progress is filed first, and the ring, the detector's speech and a pending Record press are forgotten in the capture loop (`hush`), so nothing heard before or during the pause reaches a later recording. Checked in the app: menu, countdown, resume, and a pause during a recording.~~ | ~~P1~~ | ~~M~~ | ~~done~~ |
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
| ~~A summary prompt for voice notes~~ | ~~Shipped after 1.4.0. A recording nobody else was in has its own prompt: the speaker's thought, own to-dos, no owners; it says when the microphone only heard something playing in the room. The meeting prompt had given a television drama's lines to the owner as tasks. Meetings got a stricter prompt too (a decision binds later work; two to five broad topics; an owner is a name, taken from the talk when only a label is given).~~ | ~~P2~~ | ~~S~~ | ~~done~~ |
| ~~Topics of a summary: filters and links~~ | ~~Shipped after 1.3.0. A topic in the reader is a button that narrows the Library list to the meetings with that topic (a `# topic ✕` chip in the header, composing with the project, the time range and the bin); it shows how many meetings share it. Topics match ignoring case and spacing. No page of its own for topics yet.~~ | ~~P3~~ | ~~S~~ | ~~done~~ |
| ~~Show the topics of a summary~~ | ~~Shipped after 1.3.0, in the reader.~~ | ~~P2~~ | ~~S~~ | ~~done~~ |
| ~~One vocabulary of topics~~ | ~~Shipped after 1.3.0. A new summary's prompt lists the topics the archive already uses (up to 80, most used first) and asks to reuse one exactly when it fits and to add a new one only when none does; the result is then spelled as the archive spells a topic that differs only in case or spacing, and repeats are dropped. Existing topics are not merged: only new summaries benefit, and re-summarising a meeting brings it into the vocabulary.~~ | ~~P2~~ | ~~S~~ | ~~done~~ |
| ~~Topics on the project page~~ | ~~Shipped after 1.4.0 as a constellation: an orb per topic, sized by how many meetings had it and brighter when recent, laid out by a force simulation (`d3-force`) so that topics which shared meetings sit together and are joined by a line; pointing at one lights what it is tied to, pressing it narrows the meetings list. First version was plain pills and was rejected as ugly.~~ | ~~P2~~ | ~~S~~ | ~~done~~ |
| ~~A plain heading instead of "Де воно стоїть"~~ | ~~Shipped after 1.4.0: "Стан проєкту" / "Project status".~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |
| Move the recording strip to another screen | Owner, 2026-10-05: draggable freely, between displays. Built: the strip is dragged by its own body (Wails `--wails-draggable`, the buttons stay buttons), where it is let go is kept (`meta.strip-at`, points from the primary display) and used the next time if a connected display still holds it. **Not yet checked in the app**: the native drag of a non-activating panel, the double-click on a drag area (Wails sends a zoom), and a second display. Only the owner's own running app could be reached, so nothing was clicked. | P2 | M | built, to check |
| ~~A project picture that accumulates~~ | ~~Shipped after 1.4.0. Lines belong to streams (a vocabulary of lines of work, like the topics), only what a reader would still want in a month is kept, a meeting closes what it finished, a pass every eighth meeting merges repeats and retires what is finished, and after each meeting the model writes the picture: a headline, where each stream stands, the decisions that shape the work and what needs attention, all pointing at lines by id. Measured on the owner's 30-meeting project (`exp/21_project`): the lines themselves barely changed (307 to 250, open share 97% to 94%); the written picture is the gain, about 25 lines against 104 work lines and 89 questions. Documents made before must be rebuilt (Project page, "written by the model"). Not yet measured: the second iteration (splitting oversize streams) on a full replay.~~ | ~~P2~~ | ~~L~~ | ~~done~~ |
| ~~Today, modernised, with charts~~ | ~~Shipped after 1.3.0. Three measured panels above the lists: hours in meetings per week over 12 weeks (with decisions and commitments per week in the tooltip), when in the week meetings happen (a weekday-by-hour grid of minutes), and who spoke over the chosen window. All from recordings and turns (`store.Rhythm`); nothing scored or estimated, and no chart for what the last table forbids. The lists below are unchanged.~~ | ~~P2~~ | ~~L~~ | ~~done~~ |
| Structured provenance for each summary claim | | P3 | L | after evidence |
| Word-level playback | Only after mapped token timestamps are proven; see "Whisper context and timestamps" in [decisions](decisions.md). | P3 | M | after evidence |
| Names in the live transcript | | P3 | M | after evidence |
| Better mic-only recognition of in-person meetings | | P3 | L | after evidence |
| Very large archives | Search benchmarks, pagination, and virtualization. | P3 | M | after evidence |
| ~~Logging that tells the story~~ | ~~Shipped in 1.3.0. Lines read `2026-10-04T13:34:56.456 INFO file.go:34:func() - message key=value` (`log.go`); Wails logs through it at warning level. Each LLM call logs task, provider and model at the start and time, status and reply size at the end; summaries and drafts log their recording, time and outcome; recordings log their kind and whether a person started them; the first line names the version and the commit. Checked in the app's log.~~ | ~~P1~~ | ~~S~~ | ~~done~~ |
| ~~Version in the corner of the sidebar~~ | ~~Shipped after 1.3.0: `v1.3.0` under the record button; it lights up when a newer version exists and reopens the update card.~~ | ~~P2~~ | ~~XS~~ | ~~done~~ |
| ~~Update from inside the app~~ | ~~Shipped after 1.3.0. Checks GitHub every hour, announces a new version with a system notification and a card in the window, downloads and verifies the disk image behind it, and swaps the app on Restart. See "Updates" in [decisions](decisions.md). Checked: the sha256 check, mounting, copy, signature of a real signed bundle, and the swap in tests; the card in design mode. Not clicked through in the real window: the install button.~~ | ~~P1~~ | ~~M~~ | ~~done~~ |
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
