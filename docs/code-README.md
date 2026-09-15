# Code guide

This document explains the first-party Go module for a new engineer. It is the code-level companion to the product-facing root README.

## What this module is

This repository is a desktop-first, single-binary Meeting Transcriber application built around:

- a Wails desktop shell
- local audio capture
- local transcription and speaker processing
- a local SQLite database
- optional OpenAI calls for summaries, semantic search, and project rollups

It is not a server-backed architecture. Most work happens on the machine that runs the app.

## Core design idea

The app records **two separate audio channels**:

- **left**: microphone near the laptop owner
- **right**: system audio / far side of the call

That split is not an implementation detail. It is the foundation for:

- detecting “meeting” vs “note”
- suppressing app-playback echo
- attributing microphone-owned speech to `You`
- running speaker logic on the cleaner far-side signal
- keeping automatic notes conservative when nobody else was present

If future changes collapse these channels too early, several downstream assumptions break.

## Runtime architecture

At a high level the module has five layers:

1. **Desktop shell and lifecycle**
   - `main.go`
   - Wails services
   - embedded frontend
   - local `/audio/...` file serving for playback

2. **Capture and recording**
   - `internal/audio`
   - `internal/listen`
   - device abstraction, VAD, meeting detector, preroll buffer, WAV writing

3. **Transcription and speaker pipeline**
   - `internal/media`
   - `internal/engine`
   - decode, fold, align, transcribe, diarize, extract voiceprints

4. **Persistence and retrieval**
   - `internal/store`
   - recordings, transcript rows, summaries, FTS, analytics, people memory, project state, knowledge, passages

5. **Application orchestration and UI surface**
   - `internal/library`
   - `internal/service`
   - background queue, summarisation policy, project updates, Wails-bound methods

`internal/insights` is the only outbound AI boundary.

## Startup and lifecycle

### Composition root

`main.go` owns process startup:

1. resolve the app home folder via `internal/home`
2. open a log file early
3. load config
4. open the SQLite database
5. create `service.Setup`, `library.Library`, and `service.Meetings`
6. create the Wails app and bind services
7. start a background goroutine that:
   - fetches missing models/tools
   - loads `engine.Engine`
   - starts `listen.Recorder`
   - starts the library queue
   - starts retention tidying

The log file is opened before `run()` because startup failures must still be visible.

### First run

`internal/service/setup.go` drives the “not ready yet” screen:

- `Setup.Fetch(ctx)` downloads missing assets
- `Setup.Wait()` gates model loading
- `Setup.Loaded(err)` marks the engine stage
- `Status.State()` is the narrow UI-safe polling surface

The app window appears before models are ready. First-run work is intentionally visible and asynchronous.

### Graceful shutdown

- the process root listens for interrupt / SIGTERM
- device streams, goroutines, and temp `.part` files are cleaned up best-effort
- on next launch, `listen.Recover()` removes stale partial recordings

## Production package map

### `main.go`

Responsibilities:

- process composition root
- Wails application setup
- first-run orchestration
- embedded assets via `//go:embed all:frontend/dist`
- `/audio/...` middleware that serves folded mono audio when available, falls
  back to the raw recording if folding fails, and uses `http.ServeFile` so
  browsers can seek long recordings with Range requests

Key functions:

- `main()`
- `run()`
- `sound(dir, cache)`

### `internal/home`

Responsibilities:

- own the app filesystem layout
- load/save config
- provide defaults and config compatibility behavior

Key paths:

- `Recordings(dir)`
- `Models(dir)`
- `Logs(dir)`
- `Cache(dir)`
- `Database(dir)`

Key symbols:

- `Config`
- `Listen`
- `Keep`
- `Choice`
- `Duration`
- `Defaults()`
- `Load(dir)`
- `Save(dir, cfg)`

Important behavior:

- `MT_HOME` overrides the default home folder
- bad or obsolete config values should cost the setting, not the app
- empty language means “use default”, not “auto-detect”
- old boolean `summarise` config is accepted and mapped

### `internal/models`

Responsibilities:

- define downloadable assets
- fetch missing models/tools on first run
- unpack archives safely
- report progress

Key symbols:

- `Model`
- `Set`
- `Required()`
- `Optional(transcriber)`
- `Have(dir, model)`
- `Path(dir, model)`
- `Tools(dir)`
- `Fetch(ctx, dir, set, report)`

Operational assumptions:

- models are not embedded in the binary
- required assets are fetched in small-first order
- executables live in sibling `bin/`
- `.have-<key>` markers store the source URL, so URL changes invalidate cached markers automatically

### `internal/audio`

Responsibilities:

- open microphone and optional system-audio capture
- expose them as a fixed-rate stereo stream

Key symbols:

- `Device`
- `Stream`
- `Open(ctx, system)`
- `SampleRate`
- `FrameSize`
- `Waiting`

Platform split:

- `mic.go`: microphone capture through `malgo`
- `system_darwin.go`: macOS system audio through the external `audiotee` CLI
- `system_other.go`: Windows loopback or Linux monitor-source capture

Important invariant:

- the mic path must not stall if the system device is absent or slower

### `internal/listen`

Responsibilities:

- always-on recording loop
- meeting/note detection
- preroll buffering
- live transcript preview
- WAV file finalization

Key files and symbols:

- `listen.go`
  - `Recorder`
  - `New(...)`
  - `Run(ctx)`
  - `Pause(on)`
  - `Muffle(on)`
  - `Hear(system)`
  - `Recording()`
  - `Status()`
- `detector.go`
  - detector state machine
- `company.go`
  - mic-only “someone else is here” heuristic
- `ring.go`
  - speech-aware preroll ring
- `live.go`
  - rough transcript while recording
- `vad.go`
  - sherpa Silero wrapper
- `wav.go`
  - safe WAV writer that patches the header at close

State model:

- `Off`
- `Opening`
- `Listening`
- `Recording`
- `WrappingUp`
- `Paused`
- `Broken`

Critical semantics:

- meetings auto-start only after sustained evidence
- system-channel speech is decisive evidence that someone else is present
- mic-only “company” logic may promote a note to a meeting, but should not be trusted to delete one
- preroll replays speech history, not leading silence
- active recordings roll forward without needing to “earn” the next file again

### `internal/media`

Responsibilities:

- decode arbitrary input to 16 kHz mono float32
- split app-made stereo recordings into aligned mic/system sides
- produce a listenable mono playback file
- compute waveform shapes for the UI

Key symbols:

- `Rate`
- `Decode(path)`
- `Sides(path)`
- `Voices(path)`
- `Offset(a, b)`
- `Fold(mic, tap)`
- `Listenable(path, cache)`
- `Shape(path)`
- `Mono(path, samples)`

Important behaviors:

- the decoder stack is WAV → Core Audio (`afconvert`) on macOS → `ffmpeg`
- `Fold` is an ownership-switching crossfade, not a plain average
- app recordings must be aligned before folding or analysis
- the cache filename encodes a version prefix so stale folded audio can be invalidated cheaply

### `internal/engine`

Responsibilities:

- hide whisper.cpp and sherpa-onnx details from the rest of the app
- transcribe, diarize, and attribute speakers

Key files and symbols:

- `engine.go`
  - `Engine`
  - `Open(modelsDir, opts)`
  - `Run(heard, apart)`
  - `Solo(heard)`
  - `Attribute(turns, spans)`
  - `Print(samples)`
- `asr.go`
  - Whisper configuration and transcript extraction
- `diarize.go`
  - sherpa speaker diarization config
- `voices.go`
  - speaker embeddings
- `clean.go`
  - simple cleanup before ASR
- `parakeet.go`
  - optional alternate ASR engine

Important pipeline split:

- ASR runs on the heard/folded mono signal
- diarization prefers the separate far-side signal when available
- diarization failure should not discard otherwise-valid words

Important implementation choices:

- Whisper runs with VAD enabled
- `SetMaxContext(0)` is deliberate; it avoids repetition loops
- turn timing uses mapped segment times, not raw token times from the current Go binding
- speaker attribution uses maximum overlap, not midpoint heuristics

### `internal/store`

Responsibilities:

- own the durable state of the app
- perform migrations
- maintain search and secondary indexes

Main data areas:

- recordings
- transcript turns
- summaries
- FTS transcript search
- analytics
- groups / project buckets
- kept project state
- people / voice memory
- passages and vector search
- knowledge corpus
- sticky notes / actions

Key files and symbols:

- `store.go`
  - `Open(path)`
  - `SaveTranscript(...)`
  - `SaveSummary(...)`
  - `Delete(id)`
  - `Rename(id, from, to)`
  - `Retitle(id, title)`
- `analytics.go`
  - `Analyse(turns, duration)`
- `groups.go`
  - group membership, bin, timeline marks
- `kept.go`
  - `Kept`
  - `Item`
  - `Word`
  - `Held(group)`
  - `Keep(group, kept)`
  - `Rebuild(group)`
  - `Apply(...)`
  - `Pin(...)`
- `people.go`
  - `Remember`
  - `Recognise`
  - `Same`
  - `Trace`
- `passages.go`
  - `Cut`
  - `Index`
  - `Closest`
- `search.go`
  - FTS query builder and transcript search
- `standing.go`
  - mechanical cross-meeting project rollup
- `knowledge.go`
  - knowledge sources and cached embeddings
- `notes.go`
  - notes, action edits, summary acceptance

Important storage decisions:

- SQLite uses WAL mode
- FTS is updated explicitly in the same transaction, not via triggers
- transcript saves replace prior rows for a recording
- summaries are stored separately from transcript rows
- vectors are stored as little-endian float32 blobs, not JSON
- soft delete uses a `deleted` marker for recordings

### `internal/library`

Responsibilities:

- background transcription queue
- summarisation policy
- project-state advancement
- retention orchestration
- local and semantic knowledge search

Key symbols:

- `Library`
- `New(...)`
- `Use(engine)`
- `Policy(when)`
- `Owner(name)`
- `Wait(busy func() bool)`
- `Run(ctx)`
- `Add(kind, path, started, title)`
- `Again(id)`
- `Advance(ctx, id)`
- `Rebuild(ctx, group)`
- `SearchKnowledge(...)`
- `AskKnowledge(...)`
- `Tidy(done, days)`

Important design points:

- the queue is intentionally single-worker
- `busy()` lets it stand down while the live/listening side needs the machine
- interrupted work is rediscovered from durable state by `next()`
- summarisation is fail-open: transcription completion should not depend on the LLM

### `internal/insights`

Responsibilities:

- all outbound OpenAI work
- structured summaries
- semantic embeddings
- project-state updates

Key symbols:

- `ErrNoKey`
- `New(key, model, language)`
- `Ready()`
- `Summarise(ctx, title, turns)`
- `Answer(ctx, question, passages)`
- `Ask(ctx, question, sources)`
- `Advance(ctx, project, held, meeting)`
- `Embed(ctx, texts)`

Important constraints:

- this is the only package that should send meeting text off-machine
- summaries use a hand-authored strict JSON schema
- embeddings are `text-embedding-3-small`, 512 dimensions

### `internal/service`

Responsibilities:

- provide the Wails-bound app surface
- convert library/store results into UI-friendly shapes

Key types:

- `Meetings`
- `Setup`
- `Status`
- service-facing DTOs in `reader.go`

Key `Meetings` method groups:

- capture and live state: `Listener`, `Playing`, `Listening`, `Record`, `Live`
- recordings and playback: `Recent`, `Open`, `Waveform`, `Import`, `Delete`, `Again`, `Redate`
- transcript and summary: `Search`, `Ask`, `Summarise`, `Markdown`
- people: `Rename`, `ThisIsMe`, `People`, `Forget`, `PaintPerson`, `Samples`, `Appearances`
- groups and projects: `Groups`, `NewGroup`, `File`, `Standing`, `RebuildProject`, `PinItem`, `TickItem`
- housekeeping: `Reindex`, `Tidy`, `Bin`, `Restore`, `EmptyBin`
- config: `Settings`, `SaveSettings`, `RevealFolder`

This layer should stay thin. Most real rules live below it.

## Data flow

### Recording flow

1. `audio.Open()` yields stereo frames
2. `listen.Recorder` runs VAD and the meeting detector
3. speech-aware preroll is replayed into a `.part` WAV
4. on finish, the `.part` file becomes a final recording
5. `library.Add(...)` creates a queued DB row

### Transcription flow

1. `library.Run()` picks the oldest unfinished recording
2. `media.Voices()` or `media.Sides()` prepares audio
3. `engine.Run()` or `engine.Solo()` produces turns and speaker evidence
4. library post-processes:
   - mic-owned speech becomes `You`
   - tiny fragments are folded into stable clusters
   - known people are recognised from stored voiceprints
5. `store.SaveTranscript(...)`
6. passage extraction and optional embeddings
7. optional summary generation

### Project rollup flow

1. a recording is filed into a group/project
2. once it has a summary, `library.Advance(...)` can run
3. `insights.Advance(...)` returns operations against stable kept-state IDs
4. `store.Kept.Apply(...)` bounds what the model is allowed to change

### Search flow

- exact transcript search: FTS only, fully local
- semantic meeting search: stored passage embeddings + brute-force cosine
- knowledge search:
  - exact: local text scan/search
  - semantic: lazily build/update embeddings when an API key exists

## Configuration

Defaults come from `internal/home.Defaults()` and include:

- language: `uk`
- transcriber: `whisper`
- OpenAI model: `gpt-5.4-mini`
- summarise policy: `meetings`
- transcript density: `compact`
- listening enabled
- system audio enabled
- auto-note retention disabled

Important config semantics:

- `language = auto` explicitly enables detection
- empty language does not
- OpenAI is optional; local transcription and speaker processing still work without it
- switching to Parakeet pulls extra models only when selected

## External integrations and build assumptions

`go.mod` and `Makefile` capture important operational knowledge:

- `go 1.26.2`
- Wails v3 beta desktop app
- local `replace` for whisper.cpp Go bindings:
  - `github.com/ggerganov/whisper.cpp/bindings/go => ./build/whisper.cpp/bindings/go`
- `whisper.cpp` is built from a pinned commit
- `audiotee` is built from a pinned commit on macOS
- sherpa dylibs are copied into the `.app` bundle and the rpath is rewritten
- running from the `.app` bundle matters for macOS permission behavior

If tests or builds fail because whisper bindings are missing, check the local `build/whisper.cpp/...` tree first.

## Concurrency and resource use

The app intentionally avoids broad parallelism.

- `Setup` has mutex-protected state and a one-shot ready channel
- `audio.Stream` uses goroutines per device plus one mixer
- the live transcript allows one utterance in flight at a time
- the transcription queue is single-worker
- the queue can pause when live capture is actively using the machine
- retention runs separately and only touches finished audio

This is partly correctness, partly thermal/resource management. Transcribing an old meeting while recording a new one hurts both latency and user experience.

## Invariants and “do not regress” rules

### Audio and capture

- sample rate is fixed at 16 kHz end-to-end
- frame size is fixed to the VAD window size
- left channel is microphone, right channel is system audio
- missing system audio must not stop note recording
- system backlog is trimmed rather than allowed to create persistent echo

### Detection and recording

- short speech should not start a meeting
- system-channel speech is strong evidence of a meeting
- a brief chime should not convert a note into a meeting
- mic-only “company” may promote notes, but should not destroy them
- forced recordings obey explicit stop and hard limits
- preroll must not reach back into the previous finished recording
- silence-only or too-short garbage is handled explicitly rather than left ambiguous

### Transcription and speaker attribution

- diarization errors should not discard a usable transcript
- `You` comes from channel ownership, not speaker guessing
- over-splitting can be repaired later; over-merging is harder
- two clusters should not be merged into one person just because both resemble the same enrolled voice
- voiceprint/source arrays must remain aligned

### Persistence and retrieval

- transcript saves replace old rows for the recording
- FTS stays transactionally in sync with transcript writes and speaker renames
- user-retitled meetings keep their title across summary rewrites
- audio retention deletes only processed audio, not transcripts/summaries
- kept project IDs are stable and never silently reused
- pinned kept-state text is not reworded by the model

### Search and knowledge

- hostile or malformed FTS input must not break transcript search
- semantic search should return only meaningfully close passages
- missing embeddings should be recoverable by later reindexing

## Failure semantics

The code prefers graceful degradation:

- no OpenAI key: local features still work; AI features disable or fail clearly
- no system audio: note capture still works
- bad config: defaults are kept where possible
- corrupt stored kept-state: rebuild it instead of crashing
- diarization failure: keep the transcript if possible
- failed summary/project update: do not leave the recording unfinished
- unreadable recording: fail the row, then continue the queue

## Tests as specification

The tests under `internal/**` encode many behavior contracts:

- detector thresholds and transitions
- preroll and overlap boundaries
- mixer behavior under missing or faster/slower system audio
- decoder support and WAV edge cases
- folding and alignment behavior
- transcript save/replace semantics
- FTS safety
- standing/kept-state/project rules
- people recognition and anti-merge safeguards
- retention only deleting processed audio

When changing production behavior, treat these tests as a compatibility spec, not just a safety net.

## Experiments and design history

The `exp/` tree is not shipping code. It is the design notebook that explains why several production choices exist.

### Short conclusions

- **Echo cancellation was investigated seriously and not adopted as the main answer.**
  The channel split and later switching fold were more reliable than trying to subtract room echo from a mixed signal.

- **The system tap is written late in source recordings.**
  The important bug was steady negative offset, not just abstract clock drift. Alignment must allow the tap to be shifted earlier.

- **Summing the channels was wrong.**
  `media.Fold` exists because summing the tap with its microphone room copy
  double-counted the far side and hurt playback/transcription.

- **Whisper repetition loops were real.**
  `n_max_text_ctx = 0` / `SetMaxContext(0)` is a measured mitigation, not a random simplification.

- **Parakeet is available but not the preferred default.**
  It avoided some loop pathologies structurally, but lost too much in wording/language quality on the target material.

- **Diarizing the far side alone was a strong win.**
  The app benefits from using the cleaner remote-only signal when it exists.

- **Current token timestamps from the Go whisper binding were not trustworthy enough under VAD.**
  Segment/word-level mapped timing is preferred for rows.

- **A glossary prompt was tested and rejected for the shipping configuration.**
  It needs context to stay alive, and that context reintroduces the repetition risk the app explicitly avoids.

### Experiment index

- `01_echo`: measure how much echo survives and where it sits
- `02_aec`: hand-rolled least-squares echo cancellation attempt
- `03_speex`: SpeexDSP cancellation comparison
- `04_drift`: clock-drift hypothesis investigation
- `05_aligned`: prove the practical issue is negative offset / late tap writing
- `06_whisper_loops`: isolate loop behavior and validate zero context
- `07_parakeet`: compare Parakeet against Whisper on target material
- `08_split`: separate-channel transcription experiment
- `09_switch`: historical switch-vs-sum mix prototype, preserved under `//go:build old`
- `10_pipeline`: bundle several ASR pipeline recommendations into measured comparisons
- `11_speakers`: segmentation/embedding/channel sweep for speaker quality
- `12_community1`: prepare equivalent inputs for pyannote community-1 comparisons
- `13_company`: mic-only “is someone else here?” detection sweep
- `14_timeline`: show token-clock vs segment-clock drift
- `15_rows`: evaluate row-building strategies
- `16_speakers2`: measure speaker agreement against a known reference
- `17_models`: separate quantization cost from model-distillation cost
- `18_glossary`: glossary benefit vs context cost and loop risk

## Practical caveats for future work

- comment-heavy files often encode measured rationale; do not delete that knowledge without migrating it somewhere durable
- many behaviors assume app-owned stereo WAVs have specific channel semantics
- the whisper.cpp local replace is a build dependency, not an optional nicety
- AI features are layered on top of a usable local-first base; do not let them become hard prerequisites
- if you change thresholds or constants, look for an experiment or test that justified them first

## Related files worth reading first

Outside this document, the most useful orientation files are:

- repository root `README.md`
- `WORKSPACE.md`
- `PLAN.md`
- `IDEAS.md`
- `exp/README.md`
- `Makefile`

If you are changing architecture, read the experiments before “simplifying” away a measured compromise.
