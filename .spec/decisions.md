# Engineering decisions

These are measured or product-critical choices that are easy to accidentally
“simplify” into a regression. Re-test before changing them.

## Local-first boundary

- Transcription, diarization, playback, analytics, keyword search, and storage
  are local.
- AI is optional — OpenAI, GitHub Copilot, or models on this Mac — and
  isolated in `internal/insights`.
- Models are downloaded once into persistent app storage.
- The app window opens before model setup completes.
- Invalid or legacy persisted settings fall back independently. One bad setting
  must not prevent startup.

## One application process

- The frontend is embedded by Wails.
- The queue and HTTP MCP server share the same service and SQLite database.
- Claude uses the same executable through `--mcp-stdio`.
- Do not introduce a second daemon or MCP binary without a concrete need.

## Audio channels are semantic

- Left is microphone; right is system audio.
- System-channel speech is strong evidence of a meeting.
- The microphone side identifies the laptop owner.
- Diarization prefers the cleaner system channel.
- Do not average or discard channels before ownership-dependent logic.

## Echo and alignment

- Capture clocks drift. The system backlog is corrected one sample at a time,
  not in large blocks.
- Playback folding switches ownership between microphone and system audio in
  short windows, with a crossfade when ownership changes.
- A simple average duplicates far-side speech.
- Experimental NLMS and Speex cancellation were rejected after they failed on
  real unaligned capture. No echo-cancellation filter ships.
- Playback cache names are versioned so algorithm changes invalidate old files.

Evidence is under `exp/01_echo` through `exp/05_aligned` and corresponding
`exp/out/` files.

## Whisper context and timestamps

- Rolling text context was disabled because it caused repeated phrases and was
  slower on measured meetings.
- A glossary prompt was tested and reduced accuracy while requiring the context
  mode that caused repetition. Do not re-enable it as the vocabulary feature.
- Whisper VAD removes silence. Segment timestamps are mapped back to the
  original recording; raw token timestamps from the Go binding are not.
- Two VAD assets are intentional. `vad.bin` trims silence inside completed-file
  Whisper transcription; `silero_vad.onnx` runs continuously through sherpa to
  detect speech and start/stop live recording. Neither replaces the other.
- Transcript rows therefore use segment timing. Word-level navigation needs a
  verified mapped-token implementation, not the raw token fields.
- `large-v3` weights were much larger and slower for only marginal measured
  accuracy gain over the shipping turbo Q5 model.

Evidence: `exp/06_whisper_loops`, `exp/14_timeline`, `exp/15_rows`,
`exp/17_models`, and `exp/18_glossary`.

## Speaker strategy

- Split speakers freely, then rejoin clusters using enrolled identity.
- Diarizing the system channel prevents the laptop owner's room echo from
  contaminating far-side clustering.
- The shipping threshold is intentionally loose because identity rejoining is
  part of the full pipeline. Do not tune it against one file.
- A learned name requires at least 30 seconds of speech evidence.
- The microphone owner is assigned from settings, not matched as a stranger.
- Community-1 was measured and did not justify adding Python and PyTorch.

Evidence: `exp/11_speakers`, `exp/12_community1`, `exp/13_company`, and
`exp/16_speakers2`.

## Queue and live behavior

- The processing queue is single-worker.
- Active recording prevents new historical backlog jobs from starting. An
  already-running job is not preempted.
- Processing is heavy: two back-to-back meetings (1 h and 48 min) kept the Mac
  fully busy for 11.5 minutes right after the second one (2026-09-28). The
  settings therefore choose when the queue runs: after each recording (the
  default), at a time of day, or when the Mac is free.
- "At a time" is stateless: a recording is due at the first hh:mm on or after
  its start, so one made later waits for the next day, and one whose hour
  passed while the Mac slept runs as soon as it wakes.
- "Free" means nobody has touched the keyboard, mouse or trackpad for five
  minutes (IOKit `HIDIdleTime`), the one-minute load average is under half the
  cores, and the GPU's `Device Utilization %` is under 50. None of these needs
  a privacy permission. The GPU figure is a moment, so a spike only costs one
  30-second pass. Load in the two minutes after a job is the app's own and does
  not stop a batch while the Mac stays unattended.
- A recording asked for by hand ("Transcribe now", or a retry) goes next,
  whatever the schedule, but still after a job in flight and after a live
  recording. The request lives in memory; after a restart the recording follows
  the schedule again.
- A note dictated with the Note button is asked for by hand too: it goes next
  the moment it is filed. While a recording runs, the worker looks again every
  second instead of every 30: the wake a finished recording sends arrived while
  the recorder still said it was recording, and the note waited up to half a
  minute (`TestADictatedNoteIsTakenWithinSecondsOfStopping`).
- Every queued recording shows why it waits: models, a recording, the hour,
  somebody at the Mac, other work, or its turn.
- Silence or a meeting with no words is a valid finished result.
- A transcription result remains useful when diarization fails.
- One failed job must not stop future jobs.

## Project state

- A project is a living state, not concatenated summaries.
- The model returns operations against stable IDs instead of a replacement
  document.
- Every durable item keeps source provenance.
- Repeated commitments update/restate one item rather than creating duplicates.
- Rebuild replays meetings from the beginning.
- A first summary accepted by hand (the automatic one failed or was skipped)
  does what the automatic one would: it titles a recording nobody titled and
  moves its project. Later editions keep the title and leave the project.
- Human edits pin content. The model may close a pinned item but may not
  silently reword or reassign it.

## UI behavior

- The app is a workspace, not a dashboard.
- The timeline, recording list, reader, and project dock stay spatially related.
- Library opens a meeting in place; other screens return to their origin.
- Destructive actions use bin + undo and reserve their layout space.
- Every wait has a visible state.
- Motion explains navigation or state and respects reduced motion.
- A summary asked for by hand keeps being written while another meeting or
  screen is open. The draft lives in the window (`frontend/src/drafts.ts`),
  not in the reader, which unmounts: before, the answer came back to nobody and
  the button looked as if it had never been pressed. Back in the meeting it
  shows as being written, then as the comparison to accept. A second press
  while one is being written does nothing.
- Accepting a summary compares the one the reader saw with the stored one list
  by list (`filled` in `store/notes.go`). The generated bindings turn a list
  stored as null into an empty one, so compared byte for byte a summary with no
  action items could never be replaced: "the summary has changed" (1.6.0).
- Work behind the screen shows in the title bar's corner on every screen, only
  while there is some: those drafts, and the recording the queue is
  transcribing or summarising (`Meetings.Working`, polled every 2 s). Each line
  opens its meeting; a failed draft can be retried or dismissed there.
- The player's play button never moves. The rest opens to its left while the
  pointer or focus is on it and while it plays; it used to grow from the
  button's side, so the button slid out from under the pointer. The wave is 56
  bars made from the 300-point envelope (one bar a point came out zero pixels
  wide), and the position and loudness under the playhead reach the styles as
  `--p` and `--level` once a frame, without React rendering.
- The transcript's "to the moment playing" control appears only once something
  has played and lights the line it finds. Its arrow-to-a-line icon read as
  Download, and at 0:00 a press moved nothing.

## Interface language

- The interface is Ukrainian or English, switched in Settings without a
  restart. The Ukrainian text is the key (`t("…")`); `frontend/src/en.ts` maps
  it to English, and a key without English does not compile. A word with two
  meanings carries one after `||` ("Скасувати||undo").
- No i18n library: two languages, typed keys, and Ukrainian plurals already
  handled by `Intl.PluralRules` did not justify one.
- Messages the Go side writes stay Ukrainian. The interface translates the ones
  it knows, keeping any ": detail" after a known head, so Go needs no language
  of its own. Results the interface words itself (index, retention, voice)
  come back as numbers.
- A first run follows the Mac: Ukrainian if it is among the preferred
  languages, English otherwise. An older settings file stays Ukrainian.
- English loads as a separate chunk only when chosen.
- WebKit draws `<input type="time">` in the app's locale whatever `lang` says,
  so the schedule's time is a text field.

## MCP safety

- HTTP is loopback-only.
- Tools are read-only, non-destructive, idempotent, and closed-world.
- The OpenAI key and raw voiceprint vectors are never exposed.

## Local LLM

- Local grounded Q&A is viable on all three measured models.
- No measured model matched every structured-summary contract; even the 32B
  model omitted one explicit open question.
- Measured 8B and 12B models made structured-summary errors that would
  corrupt action/deadline or chapter data.
- The owner chose to ship the option before quality is settled (2026-09-27):
  the default is the small Gemma 4 E2B Q4_0 (Apache-2.0, 2.8 GB), picked for
  size and licence, not measured against OpenAI. A bigger model is a Hugging
  Face `.gguf` link in the settings. Its first real run chaptered everything
  at 0 s and left a spoken deadline out of `due`.
- Ollama is the benchmark runtime, not a shipping dependency.
- The runtime is `llama-server` v0.5.0, built from a pinned commit and bundled
  like `audiotee`: one lazy helper process per model file, loopback only, a
  per-start key in its environment, and `--sleep-idle-seconds 300` so an idle
  model leaves memory. Linked in, its ggml would clash with whisper.cpp's; as a
  process, running out of memory cannot stop a recording.
- Local generation uses Chat Completions. llama-server's Responses endpoint
  ignores `text.format`; its chat endpoint turns the schema into a grammar.
- Generation and embeddings are separate choices. The local embedder is
  Qwen3-Embedding-0.6B Q8_0 with the query-side instruction; its 1024-wide
  vectors are cut to 512 and renormalised so the archive keeps one width.
  The `meta` table names the model behind the stored vectors; a different
  one clears them and reindexes, since vectors from two models do not compare.
- Meetings, notes and projects are re-embedded in the background as soon as
  the embedder changes, with a named progress line in the settings. Left to the
  first question, that answer waited about 2.5 minutes on a 100-meeting
  archive. Transcript passages are reindexed from the settings button.
- Helpers stop in Wails' `OnShutdown`, not a `defer`: quitting ends the process
  before `app.Run` returns. A helper left by a crash is killed when the next
  one for the same file starts.

Evidence: `exp/19_local_llm`, `exp/out/N-local-llm.jsonl`,
`.spec/spikes/performance-local-llm-spike.md`, and
`TestLocalModelsSummariseAndFindOnThisMac` (`MT_TEST_LLAMA=1`).

## GitHub Copilot

- GitHub Models, the OpenAI-compatible inference API, was retired on
  2026-07-30. The supported route is the Copilot SDK, which drives the Copilot
  CLI. Reverse-engineered Copilot API proxies are not an option.
- The CLI is downloaded when Copilot is chosen, pinned to 1.0.105 — the version
  SDK v1.0.28 was released against ("SDK snapshot for Copilot CLI 1.0.105-1";
  1.0.85 and v1.0.14 until 2026-11-01) — and always run with `--no-auto-update`;
  without it the binary hands over to whatever newer copy the person has.
- Sign-in is the CLI's own `copilot login`, which opens the browser. Its state
  lives in `~/MeetingTranscriber/copilot`, apart from the person's own Copilot
  history. `ModeEmpty` turns the keychain off, so the token has to live in the
  CLI's config there. The login runs with `COPILOT_DISABLE_KEYTAR=1` and
  `"storeTokenPlaintext": true` in `settings.json`: without a keychain the CLI
  asks before writing the token to a file, and only at a terminal, so a login
  started by the app exited with "the token was not saved". CLI 1.0.85 and 1.0.105 keep
  it under `authTokens` in `config.json`, mode 0600. Kept in the keychain
  (1.1.0–1.2.0), the token was found only through the CLI's `gh` fallback:
  fine from a terminal, absent in an app opened from Finder or the Dock, whose
  `PATH` is `/usr/bin:/bin:/usr/sbin:/sbin`. Every summary then failed with
  "Session was not created with authentication info".
- Each request first asks the CLI whether anyone is signed in (local, under
  10 ms). If nobody is, the app says where to connect instead of passing on
  the CLI's model-resolution error.
- Sessions run in `ModeEmpty` with no tools, every permission refused, and a
  replaced system prompt. The SDK has no schema option, so the schema goes in
  the prompt and the JSON is taken from the reply. Each session is deleted
  after its answer.
- No model chosen means Copilot's own `auto`; left empty, the CLI would use its
  default model instead. The account's model list leaves Auto out for that
  reason.
- `SendAndWait` keeps only the English text of a failure. A spent quota is
  recognised by the session error event's `errorType` and reported with what to
  do. Once the monthly quota is spent every model is refused, GPT-5 mini
  included (checked 2026-09-28).
- Copilot bills AI Credits per token. Business and Enterprise need the Copilot
  CLI policy enabled. The SDK has no embeddings; search uses OpenAI or local.
- The model list in the settings is newest first; in Copilot's order a new
  model sat in the middle. A model the account has but has not enabled (policy
  `unconfigured`) is listed and cannot be chosen, with a line on enabling it on
  github.com; one an administrator disabled is left out. `models.list` on the
  pinned CLI 1.0.85 already returned GPT-6.1 Sol for the owner's Copilot
  Business account (`exp/23_copilot_models`, 2026-10-31), so the pin was not
  what hid it; 1.0.105 returns the same list. GPT-6.1 Terra (announced
  2026-10-26, rolled out gradually) is refused for that account by both
  versions, so the organisation does not have it yet.
- Moving to CLI 1.0.105 (2026-11-01) changed nothing the app relies on: the
  token file keeps its keys (`config.json` is JSON with `//` comments), the
  existing sign-in works, and `copilot login` still starts under
  `COPILOT_DISABLE_KEYTAR=1` with `storeTokenPlaintext`. A completed new
  sign-in has not been tried. The npm package is now only a loader, so the
  JavaScript can no longer be read there.

## Recording strip

- While a meeting (or a recording started by hand) is in progress, a capsule
  hangs under the menu bar, centred on the primary display: a reminder to tell
  the others, then the latest live line, with Pause and Stop. It shows over
  every Space and full-screen app and is excluded from screen capture.
- It is a non-activating panel at status level. Wails' notch window was
  rejected: it makes itself key on hover and takes the keyboard from the call.
- WebKit swallows the first click on an inactive panel, then makes the panel
  key. `strip_darwin.go` answers `acceptsFirstMouse:` YES and
  `needsPanelToBecomeKey` NO for the strip's web view alone, through a
  class-level swizzle gated by an associated object. Giving the view a class of
  its own crashed: WebKit observes it with KVO, whose generated class must stay
  its class.
- It is placed by plain X/Y from the primary display's work area. Wails beta.16
  divides `Options.Screen` coordinates by the Retina scale a second time.
- It is created when the recording starts and closed when it ends, so the main
  window stays the last one to close.
- It is dragged by its own body (`--wails-draggable`; buttons opt out) and the
  place it is let go is kept in `meta.strip-at`, in the same points from the
  primary display's top-left as the placement above. The next strip opens there
  if a connected display's work area still holds that point, else centred as
  before. Moves in the first two seconds are placement, not the person's.
- Pause holds the file open. Nothing heard while held is written, transcribed
  or kept for a later preroll, and the detector stands still so a pause cannot
  end the meeting. Stop while held still stops.

## Pause for a private conversation

- The record button at the foot of the rail pulls out a drawer when pointed at
  or focused: Record with its shortcut, and "don't record" for 5, 15 or 60
  minutes. The drawer closes when the pointer leaves or the app loses focus. WKWebView reaches buttons by Option-Tab, not Tab, unless macOS
  keyboard navigation is on.
- The pause files a recording in progress first, then forgets everything that
  could carry into the next one: the ring a preroll replays, the speech the
  detector was counting, a Record press not yet applied. Until it ends nothing
  is detected, written, transcribed or kept.
- The capture loop does all of it on its own goroutine at the next frame
  (`hush`); the Wails call only sets the end time.
- Record (the menu or ⌘R) ends the pause. While paused, the button itself
  resumes listening.
- The pause lives in memory: quitting the app ends it.

## Voice notes

- A voice note is a recording in which nobody but the owner was heard:
  `recordings.kind = note`; the only other kind is `meeting`. The detector
  decides when a recording starts: a note unless the system channel carried
  3 s of speech in the minute before. While it records, speech on the system
  channel (any at all), or the company detector's second voice, turns it into a
  meeting; a meeting never turns back. The company detector is asked only when
  system audio capture is off in Settings, since with it the call side already
  answers, so with system audio on a conversation in the room, heard only by
  the microphone, is a note too. A recording's kind carries into its next
  hourly file; a file already handed over keeps the kind it had, and the kind
  never changes after that. Notes come from the
  Note button, from Record pressed while only the owner speaks, and from
  recordings the app starts itself, which are deleted when they stop unless
  `keep_notes` is on (a Settings switch, off by default). Notes existed before
  1.7.0, mixed into the Library list; 1.7.0 gave them the button, a screen and a
  prompt of their own. No data was migrated.
- A meeting view is one whose list or numbers would read wrong with a voice note
  in them, as "time in meetings" would: today the Library list and its timeline,
  the topic vocabulary, and Today's recent meetings and minutes in meetings. A
  project's recordings, a person's history and search are not. Each meeting view
  leaves notes out with `kind = meeting`: in SQL in `store.Span`, `store.Topics`
  and `store.Brief`, and in `Meetings.tsx` for the Library list, because
  `store.Recent` also feeds the queue, To do, MCP and audio retention, which
  must see notes. A new meeting view adds the same condition and a case to
  `internal/store/voicenotes_test.go`. Everything about the archive as a whole
  keeps notes: search, Ask, MCP, projects, To do (a note's to-do opens its card)
  and retention.
- The Note button sits above Record at the foot of the rail. A press starts a
  dictated note (`Recorder.Dictate`), the one kind of note that the button
  makes; the button then reads Done, and a second press, or Record's Stop, ends
  it. Silence does not: like any recording started by hand, it runs until it is
  stopped. A dictated note is the microphone's alone, whatever else happens: the
  detector's speech history is cleared at the press, so system-channel speech
  from before cannot make it a meeting; the system channel is zeroed before
  detection and before writing, as already done while the app plays audio
  itself; no preroll is poured in; the company detector is not asked; when the
  recording is cut into the next hourly file, that file is dictated too; the
  strip under the menu bar, there to remind the owner to tell the others, is
  not shown.
- When a recording stops, its file is deleted if it is under five seconds of
  audio and not dictated; if it is dictated and holds not one frame the speech
  detector judged speech; or if the app started it, it is still a note, system
  audio is on, `keep_notes` is off and nobody asked for it (`asked`, set when
  Record or Note started it, or when a line was typed into it). A meeting is
  never deleted otherwise. A room conversation the app started is therefore
  lost on the default settings: a known limit, see "Rescue a recording the app
  discarded" in the roadmap. A dropped dictation sets `Status.Missed`, so the
  button says "Silence" and the Notes screen says nothing was heard, for five
  seconds. The recorder
  hands it to the processing queue (`internal/library`) with `Filed.Dictated`,
  and the queue takes it next whatever the schedule says, as "Transcribe now"
  does: still after a job in flight and after a recording in progress has
  stopped. The flag lives in memory, so after a restart before transcription the
  note waits for the schedule like any recording; that is accepted.
- While any other recording runs, the button does not start a second one,
  since there is one microphone. It opens a field beside the button
  (`Recorder.Jot`); the line typed becomes one of that recording's sticky notes
  (`store.Sticky`, the notes shown in a meeting's page), at the seconds since
  its file began, preroll included. Typing also sets the in-memory flag that
  Record sets (`asked`), so a recording the app started itself is no longer
  deleted as unwanted. Only the five-second minimum can still drop it, and the
  typed lines go with it (logged as `typed_lines_lost`). The seconds count from
  the start of the current hourly file.
- A note is written out by its own prompt (`insights.Polish`), in the summary
  language from the settings: the speaker's own words in the first person,
  fillers and corrections resolved ("tomorrow, no, Friday" is Friday), and
  to-dos with a due date `YYYY-MM-DD` whenever the day follows from what was
  said. Above the transcript the prompt lists the recording's day and the seven
  days after it, so that "on Friday" needs no counting; a date further off ("by
  the 15th") is still a date, and only a vague time ("next week") stays as
  words. Everything is dated from the recording's start, not from when the
  prompt runs, so a note rewritten months later keeps its dates. There are no
  decisions, chapters, topics or to-do owners, and the prompt asks for about
  400 words at most; neither the length nor the date format is checked in code,
  and the interface shows a due that is not a date as it is. A note is written
  out automatically only when Automatic summaries is on Everything; on
  Meetings, the default, a note goes to no model until "Write the text" in its
  card's menu. Without AI, or when the call fails, the card shows the raw
  transcript.
- Measured (`exp/out/24-notes.md`) with Copilot `gpt-6.1-sol` writing, the model
  in the owner's settings, and `gpt-5.6-terra` judging which of two unlabelled
  texts is better; users get whichever model they choose. Six made-up dictations
  (`exp/24_notes/cases.json`) are checked by code for each to-do's date, the
  number of to-dos and the words that must stay or go: 12 of 12 over two runs.
  On 29 archive notes the new text was judged better than the 1.6.0 prompt's
  overview, decisions and questions 21 times, worse 7, tie 1, with 4 unsupported
  statements in all against 5. The old format is still preferred for hour-long
  room conversations kept as notes (5 of 7); that is accepted, since dictation
  is what the screen is for. A change to the prompt is checked with `go run
  ./exp/24_notes -out <dir>`, which runs the made-up cases alone and needs only
  Copilot; with `-db` on a copy of an archive it also compares the change with
  the prompt kept in the program as `previousNote` (1.6.0's; put the current
  wording there first). Every made-up check must pass, and the change must be
  judged better at least as often as worse. Nothing checks the dates themselves:
  a day Whisper turns into a non-word stays undated because the prompt forbids
  guessing, and one misheard as another day gets that day.
- An older note keeps its text until "Write the text again" in its card's
  menu. That makes a draft, kept by the window (`drafts.ts`) until the app
  quits; the card shows it with Accept and Keep it as it was. Accept replaces
  the whole summary, so the note's to-dos, with their ticks and edits, become
  the draft's; the undo it offers for ten seconds puts the old summary back,
  ticks included. Keep drops the draft.
- A card's menu also opens the recording's full page in Library (chapters,
  decisions, its sticky notes, speakers): an older note, or a room conversation
  kept as one, has more there than the card shows. The cards flow newest
  first, the day and time on each, in `display: grid-lanes` where WebKit has it
  (macOS 27) and a plain grid elsewhere. Day groups were tried first and
  dropped: with a few notes a day, a wide window showed two cards a row and the
  rest of itself empty.

Evidence: `exp/24_notes`, `internal/listen/dictate_test.go`,
`internal/store/voicenotes_test.go`, and a sandbox copy of the packaged app
whose microphone played a synthesized dictation. The Mac was in use, so the
"when the Mac is free" schedule would have kept a meeting waiting, yet the
queue took the note 11 ms after it stopped; it was transcribed in 1 s and
written in about 9 s.

## Logging

- One line per event in `~/MeetingTranscriber/logs/mt.log` and on stderr:
  `2026-10-04T13:34:56.456 INFO file.go:34:func() - message key=value`.
  `log.go` writes the front; slog's text handler writes the attributes.
- Wails logs through the same writer at warning level. Its information lines
  are build details and one line per asset served.
- Every LLM call logs its task, provider and model when it starts, and its
  time, status and reply size when it ends. Summaries log the recording, their
  time and outcome; recordings log their kind and whether a person started
  them; the first line names the version and the commit.

## Summary prompts and the project picture

- Three prompts, three scenarios: a meeting (`Summarise`), a note (nobody else
  in the recording: `Polish`, see Voice notes), and the project fold. A new
  prompt is compared on the owner's own recordings first (`exp/22_summaries`,
  with the summaries kept out of the repository). Measured on 10 meetings and
  the 3 notes with enough speech: owners as `SPEAKER_nn` 3 to 0 (the name is
  taken from the talk), topics 3.7 to 3.3 a recording, and a television drama
  no longer yields tasks owned by the speaker. The note sample is small.
- A project is a ledger of lines with provenance plus a written picture. The
  fold sees only the open lines, grouped by stream, with finished ones counted
  (`insights.Held`); it puts each line in a stream (reusing a name, never a
  heap: about fifteen open lines, then a narrower stream), keeps only what a
  reader would want in a month, and closes what the meeting finished. Every
  eighth meeting a pass over the whole document merges repeats, retires
  finished work and splits oversize streams (`Kept.Tidy`: a pinned line keeps
  its words and is never merged away, a decision is never retired). After each
  meeting `Brief` writes the picture from the open lines; it can point only at
  ids that exist (`Kept.Known`). A rebuild writes it once, at the end.
- Measured on a 30-meeting project (`exp/21_project`, same model): the ledger
  improved little (307 to 250 lines, 97% to 94% open, repeats about the same),
  so the ledger is not what is shown first; the picture is. A speaker label is
  never an owner (`Kept.Apply`). `do` and `kind` are enums in the schema, and
  the log counts operations the document could not use.
- Documents made before this have no streams or picture until rebuilt; the
  project page says so.
- The meeting write-up is detailed (2026-10-31). An hour of talk had come out
  as two sentences and a line per chapter, and the chapter lines reached only a
  list of titles. Now the overview is a paragraph (what the meeting was for,
  the main subjects and what came of each, what is open) and each chapter says
  what was said in that part, by whom and why, with the figures, at the length
  the part earns; the reader shows the chapters as "How it went" below the
  decisions, commitments and questions, and Copy Markdown includes them.
  Measured on six meetings of 20 minutes or more and one 60-minute note
  (`exp/out/22-summaries.md`): chapter text 589 to about 3,500 characters a
  meeting, overview 288 to about 750, judged more useful blind 7 of 7 twice,
  about 24 s instead of 14 per meeting. Detail first brought five unsupported
  statements into the prose; rules against filling in what a vague phrase
  leaves out and against stating an inference as said took that to none. The
  weak spot left is topics reused from the archive's vocabulary: 3 of 25
  judged not to fit.
- On GPT-5.6 Terra, the owner's main model (2026-11-01, judged by GPT-6 Sol):
  chapter text 618 to about 4,100 characters a meeting, judged more useful 7
  of 7 and then 6 of 7. Terra first worked out ratios nobody said and widened
  what one person said to others; two rules (numbers as said, a statement
  stays with its person) took its unsupported statements from 11 to 4,
  against 9 for the previous prompt in the same run. The judge varies by
  about three statements between runs on identical texts. GPT-6.x Terra is
  refused for the owner's account by the pinned CLI.
- The prose never names a speaker by a label such as SPEAKER_05 ("a
  participant", or a role): renaming a speaker later changes the transcript,
  not the summary's text. Owners keep the measured rule above.
- A draft asked for by hand may take five minutes before it gives up; two was
  enough for the short summary only.

## Topics

- A summary's `topics` are one vocabulary, not free text. `store.Topics` lists
  those the archive's meetings use, a note having none (grouped in Go,
  case-insensitively: SQLite's `lower()` skips Cyrillic), and `Summarise` puts
  the first 80 into the prompt with the rule: reuse one exactly when it fits,
  add one only when none does.
  `insights.Canon` then fixes spelling that differs only in case or spacing.
- Existing summaries are not rewritten. Merging synonyms across the archive is a
  separate, measured job.
- In the reader a topic narrows the Library list (`topic` state in
  `Meetings.tsx`, composing with project and time range); it never navigates.

## Today's rhythm

- `store.Rhythm` measures meetings only (not notes): hours per Monday-start
  week over 12 weeks, minutes by weekday and hour (a meeting is spread over the
  hours it spans), and speech seconds per person over the chosen window, with
  every `SPEAKER_xx` voice pooled as one unnamed share.
- Nothing is scored or projected. Closed commitments over time are not charted:
  `done` has no timestamp.

## AI usage and cost

- `insights.Client` reports every model call (chat and embeddings) to a hook
  the service sets in `Setup.Usage`; the `usage` table keeps task, provider,
  model, tokens, seconds and failure, never content. A failed call is kept too,
  with whatever tokens the provider reported.
- OpenAI and llama-server report tokens in the response. Copilot reports them
  per model call in `AssistantUsage` session events (`inputTokens`,
  `outputTokens`, `copilotUsage.totalNanoAiu`), summed per request; the model
  the event names replaces "auto".
- OpenAI cost is tokens times the list price in `insights/price.go`, a
  snapshot (gpt-5.4-mini $0.75/$4.50 and text-embedding-3-small $0.02 per
  million). A model not in the table shows tokens only, never an invented
  price. Copilot is shown as AI Credits, taking 1 AIU = 1 AI Credit
  ($0.01 each, per GitHub's billing page); that conversion is not yet checked
  against a real billed call. A local model costs nothing but time.

## Updates

- The app asks GitHub's `releases/latest` at launch and every hour
  (`MT_UPDATE_URL` replaces the address, for tests); a setting turns it off.
  No library: Sparkle needs its own signing keys and framework, and
  go-selfupdate replaces a single binary, not a bundle.
- Installing downloads the release's `.dmg`, checks it against the asset's
  sha256 digest, copies the `.app` out of the mounted image next to the
  installed one (the same volume, so the swap is a rename), and checks
  `codesign --verify --deep --strict` and the bundle version. Nothing is
  swapped until a person presses Restart, which is refused during a recording.
  A detached shell then waits for the app to quit, exchanges the bundles,
  puts the old one back if that fails, and opens the new one.
- A download made by the app carries no quarantine flag, so the unnotarized
  update opens without the Gatekeeper dialog. Every release is signed by the
  same local identity, which is what keeps the microphone grants. A copy that
  is not an `.app`, or sits where the user cannot write, offers the release
  page instead.
- A system notification announces a version once (`meta.update-notified`);
  the card in the window is the offer, and "Later" is remembered per version.

## Packaging

- Run and distribute the `.app`, not the bare Go executable.
- macOS privacy grants belong to the application identity.
- The bundle includes `audiotee`, `llama-server`, and sherpa dynamic libraries.
- Local signing stabilizes developer permissions; public distribution still
  requires Developer ID signing and notarization.

## Windows

- Built on the Mac: `make exe` cross-compiles with mingw-w64 and packs an NSIS
  installer. whisper.cpp and llama.cpp come from the same pinned commits, built
  in `build-windows/`. Windows-only code lives in `_windows.go` files and in
  `Windows:` window options, which macOS ignores; the macOS targets read nothing
  from the Windows part of the Makefile.
- x86-64 only: sherpa-onnx ships no ARM64 Windows libraries. Whether Windows
  on ARM runs the x64 build, and passes the installer's AVX2 check, is not
  known.
- AVX2 is the floor. A cross build leaves `GGML_NATIVE` off, and ggml then
  compiles plain SSE2 unless each extension is named, so AVX2, FMA, F16C and
  BMI2 are named. The installer refuses a processor without AVX2
  (`IsProcessorFeaturePresent(40)`, answered from Windows 10 build 19041), as
  Whisper would stop on its first instruction. OpenMP is off: ggml uses its own
  threads and no libgomp ships.
- The executable imports only Windows itself and `sherpa-onnx-c-api.dll`:
  libstdc++ and winpthread are linked statically, and `llama-server.exe` is
  linked with `-static`.
- sherpa-onnx is imported through its cross-platform package
  (`sherpa-onnx-go/sherpa_onnx`), which on macOS re-exports the same
  `sherpa-onnx-go-macos` module as before.
- Whisper runs on the processor; no GPU backend is built for Windows, and its
  speed there is not measured.
- The installer is per user (`%LOCALAPPDATA%\Programs\Meeting Transcriber`, no
  administrator), adds Start-menu and desktop shortcuts and an uninstall entry,
  and runs Microsoft's WebView2 bootstrapper only when the registry shows no
  runtime. Uninstalling removes the program and WebView2's data for it, never
  `%USERPROFILE%\MeetingTranscriber`.
- One copy at a time. Windows starts a process for every click, and a second
  one would open the same database and microphone and, through
  `listen.Recover`, try to discard the first one's recording in progress. A
  process that finds the named mutex `MeetingTranscriber` brings the first
  window forward and exits in `init`, before it opens anything. The installer
  waits on the same mutex. `--mcp-stdio` is exempt.
- No update from inside the app: `update.Bundle` finds no `.app`, so the card
  offers the release page.
- "When the computer is free" reads `GetLastInputInfo` for the five minutes and
  the processors' busy share over one second (`GetSystemTimes`) in place of the
  load average; the GPU is not read.
- FFmpeg and the Copilot CLI are the Windows builds from the same releases,
  with pinned sha256: `ffmpeg-win32-x64.gz` and `copilot-win32-x64.zip`, a zip
  holding one `copilot.exe`.
- Console programs the app starts (FFmpeg, `llama-server`, `copilot login`) run
  with `CREATE_NO_WINDOW` (`quiet`): the app has no console to lend them, so
  Windows would open a window for each. The Copilot SDK hides its CLI the same
  way. A `llama-server` left by a crash is not found again, as there is no
  `pkill`; it unloads its model after five idle minutes.
- The strip is an always-on-top window kept out of the taskbar, without frame
  decorations, excluded from screen capture by Wails. Unlike the macOS panel, a
  click on it makes it active.
- A first run takes the interface language from Windows' preferred display
  languages.
- Interface words that name the Mac or ⌘ say "this PC" and Ctrl on Windows
  (`windows` and `mod` in `i18n.ts`). The shortcuts already took Ctrl, and
  Wails turns WebView2's own accelerator keys off, so Ctrl+R records instead of
  reloading the page.
