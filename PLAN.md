# Meeting Transcriber — working plan

The standalone, single-binary edition.

## Done

- [x] `internal/home` — `~/MeetingTranscriber/`: config, db, recordings, models, logs
- [x] `internal/models` — first-run download, 583 MB, resumable, progress
- [x] `internal/engine` — whisper.cpp (Metal) + sherpa-onnx diarization, verified
      at 19.2× realtime on a real meeting
- [x] `internal/insights` — summary and Q&A over `openai-go/v3` Responses API,
      verified live on a 153-turn meeting
- [x] `Makefile` — vendors and builds whisper.cpp at a pinned commit

- [x] `internal/store` — SQLite with FTS5 search, 13 tests
- [x] `internal/media` — WAV natively, afconvert, then ffmpeg; 12 tests
- [x] `internal/library` — the queue: decode → engine → store → summarise, 8 tests

- [x] `internal/audio` — microphone + system capture, ported from `daemon/source`
- [x] `internal/listen` — ring, detector, recorder; Silero through sherpa-onnx
      rather than a second ONNX runtime, 12 tests
- [x] `internal/service` — the bound surface: 25 methods, typed TS bindings
- [x] `main.go` — Wails v3, models loaded behind the window rather than in front
- [x] `frontend/` — React 19 + Vite 7 + Tailwind v4 + Motion + lucide
- [x] Six screens — Today, Library, Transcript, Search, To do, Ask, Settings —
      all driven by hand in a browser
- [x] Import from the UI, through the native file picker
- [x] `make bundle` / `make dmg` — signed .app, dylibs carried inside it, 34 MB disk image
- [x] Verified end to end: 583 MB downloaded, a real meeting captured through the
      system tap, replayed 46 s from before it was noticed, transcribed at 12×
      realtime with two speakers separated, and the app run from the DMG copy
      outside the build tree

- [x] Any format — WAV here, everything Core Audio reads through afconvert,
      the rest through an ffmpeg the app fetches. Six containers pinned by test.
- [x] Named voices — a voiceprint per speaker, kept per recording; naming
      somebody once teaches the app their voice. Verified on real audio: the
      name came back by itself on the next run.
- [x] Analytics — talk time, silence, overlap, pace, questions asked, how evenly
      the floor was shared, and the shape of the hour. Computed from the rows,
      so it follows a rename with no migration.
- [x] Summarise on demand, and a summary that fills overview, topics, decisions,
      owners and deadlines. Verified against a real Ukrainian meeting.
- [x] Record now makes a note, not a meeting — and it becomes a meeting on its
      own if somebody else starts talking.
- [x] App icon. The disk image lays its window out too, but only on a machine
      that has granted Finder automation — see the note below.
- [x] **Live transcript** — the meeting written down as it happens, from the
      utterances the detector already cuts. The queue stands aside while a
      meeting is being recorded so the two do not fight over the cores.
- [x] **Today** — what happened, what was decided, what is still owed, what is
      past its date, and what has been asked in more than one meeting without
      an answer. All local, no key needed.

- [x] **Who said it, using the channels.** The microphone side is the person
      sitting here and the system side is everybody else, and the app was
      throwing that away. A recording with a silent system channel is now never
      diarized at all, and in a call everything markedly louder on the
      microphone is folded into one speaker. Measured: seven minutes of one
      voice went from three speakers to one, and a 64-minute meeting from nine
      labels to seven with the owner's own turns finally in one place.
- [x] Retention — audio older than `keep.audio_days` is deleted daily and on
      demand; the transcript, summary and analytics always stay.
- [x] Playback — a range-served route inside the app, a player under the header,
      click a row to hear it, and the row being spoken is lit.
- [x] Search by meaning — passages embedded at 512 dimensions, cosine in memory,
      blended with the keyword index. Verified on 574 real passages: "who owns
      testing quality" finds Ukrainian passages about tests and metrics where
      keyword search returns nothing.

- [x] **The echo.** Playing a recording back gave every remote sentence twice,
      once through the room and once from the tap. Measured: the microphone led
      the tap by 495 ms for a whole meeting, because the mixer's backlog bound
      was half a second and was only ever trimmed *at* the bound. The bound is
      now 125 ms, and the mono mix takes the tap whenever the tap has anything
      instead of averaging the two — averaging was summing a signal with a room
      recording of itself. Correlation with the clean tap went from 0.605 to
      0.898, and one real meeting went from 11 speakers to 4.
- [x] Ukrainian by default, and named rather than detected. A summary is written
      in the configured language whatever the transcript mixes in.
- [x] Ten voiceprints per person, up from eight
- [x] Delete on hover in the Library, and transcribe-again inside a meeting
- [x] "This is me" — one enrolment for the person holding the laptop
- [x] Parakeet, choosable in the settings, downloaded only when chosen

- [x] **The swallowing.** The first fold switched between the channels
      outright — 272 hard changes in 26 minutes, average step 0.056 at the seam
      against the 0.02 where a click is audible. Replaced with a ducker: the tap
      always at full level, the microphone attenuated under it, gain continuous
      so no seam is possible. Two settings, because the audiences disagree —
      0.25 for a listener (2 swallowed quarter-seconds against 142), 0.10 for
      the models (1263 words against 1192, three runs each).
- [x] **The bin.** Deleting is one quiet click and an undo, not a coral button
      and a confirmation. Nothing is destroyed for a fortnight.
- [x] **Projects.** Recordings can be filed into groups and the Library filters
      by them.
- [x] **Junk costs nothing.** A recording nobody else was in and nobody asked
      for is discarded when it ends — pressing Record always keeps it, which is
      the one unambiguous signal that a note was wanted. Summaries default to
      meetings only, so a phone call never reaches a model.

- [x] **The far side.** The clustering threshold was 0.9 and gave four times too
      many speakers on a real meeting. Measured against recordings whose answer
      is known — one the owner said had two people in it, and one built by
      splicing two different meetings so there are certainly at least three —
      1.10 is right on both and 1.20 begins collapsing people together. A real
      26-minute meeting went from 11 speakers to 2 plus an 18-second stray.
- [x] **A name needs evidence.** A voiceprint can be computed from four seconds
      and that is nowhere near enough to be right about whose it is; an
      18-second fragment was handed a colleague's name. Half a minute is the
      bar now, and the microphone side is never matched at all — it is the
      person holding the laptop, which "This is me" settles once and for all.
- [x] **What the listener threw away** is counted and shown on Today, so the
      setting can be seen working rather than believed.

- [x] **A waveform in the player.** Three hundred buckets of the file actually
      being served, clickable, with the played part lit. Scrubbing was blind.
- [x] **Echo cancellation, and the capture bug it uncovered.** NLMS against the
      tap as the reference — four tests on synthetic rooms: 12 dB+ removed, your
      own voice unchanged, safe through double talk. On real recordings it
      diverged, and the trace said why: the mixer trimmed the system channel's
      backlog in ~67 ms blocks several times a second, so the two channels were
      never sample-locked and no fixed delay held for longer than a moment. The
      mixer now drops one sample per frame instead — measured at 0.06 s/min of
      correction against 22.5 s/min, a smooth drift a filter can follow. Wired
      in behind a guard that returns the microphone untouched on any block it
      cannot improve, so recordings made before the fix are unaffected: the same
      meeting still transcribes to 1455 words.

- [x] **A meeting can be renamed.** The title is the field — a textarea, not an
      input, because titles wrap and an input would silently turn a two-line
      heading into one line that scrolls. No pencil, nothing that appears only
      on hover, no dialog. A typed title is pinned: `titled` is set, and
      summarising again leaves it alone, which is the same rule the project page
      will need for every item a person edits. Pinned by two store tests.

- [x] **A colour that stays put — for a person and for a project.** It is derived
      from the name rather than from a position in a list, which is what was
      wrong before: the palette was indexed by whoever spoke first, so one
      person was a different colour in every meeting. Twelve hues evenly spaced
      around the wheel, with the arc around the app's own violet left out so
      that "this is interactive" and "this is Marta" stay different signals. A
      first attempt used fourteen hand-picked hues and had pairs 12 degrees
      apart, which at one lightness are the same colour — the picker showed
      duplicates. Overridable per person and per project in Settings.

- [x] **A saved voice can be listened to.** Each voiceprint now carries the
      meeting and speaker it came from, and the app plays the longest thing that
      person said there. The claim "this is Marta" was 512 numbers nobody could
      examine; it is now evidence. Prints saved before this are matched back to
      their meetings on the next start by exact vector equality, so the samples
      already collected became playable rather than being lost. Two tests pin
      the one thing that would silently break it: prints and sources are two
      arrays that have to be cut in the same place.

- [x] **Projects can be renamed and recoloured**, in Settings alongside people.

- [x] **Making a project worked nowhere.** The button called `prompt()`, which
      the WebView this ships inside does not render, so nothing happened in the
      packaged app — only in a browser. It is now a field in the header, which
      is also the pattern the rest of the app uses.

- [x] **The projects are one strip, and it is also where meetings are filed.**
      The first version was a row of tabs with a coloured dot beside each name.
      The owner rejected it outright — it is the most obvious way to attach a
      colour to a label and the colour did no work.

      What replaced it: each project is a block as wide as the number of
      meetings it holds, so the strip is a picture of what the library is
      actually made of, including how much of it was never filed. Nothing else
      in the app shows that. Without colour the strip is a grey rectangle, so
      the colour is finally load-bearing rather than decorative.

      The blocks are the drop targets. Drag a meeting onto one and it is filed;
      the block widens to meet the card before it is released, so where the
      thing will land is visible a moment early. Dropping on Unfiled takes it
      back out. Navigation, proportion and filing are one object.

      Two things were wrong in the first build and fixed on the evidence:
      pure proportion made a one-meeting project a 2% sliver with no room for
      its name, so every block now starts wide enough to read and the counts
      divide up what is left; and an empty project had no block at all, which
      meant a project nothing could ever be dragged into.

- [x] **Drag and drop does not use the HTML5 drag API.** The WebView this ships
      inside never fires `dragstart` for page elements — the same class of trap
      as `prompt()`. It is done with pointer events instead, which also means
      the card being carried is drawn by the app rather than by the system.

- [x] **Meetings became a workspace.** The screen that was a list of cards and
      the screen that was one meeting are now one: a timeline, a list, a reader
      and a dock of projects. Built on the design the owner chose after three
      of mine were rejected; the lesson recorded in WORKSPACE.md is that every
      one of mine was a hero chart with a list bolted underneath, and what he
      wanted was a room to work in where the chart is a small instrument.

      Landed with it: Geologica bundled into the binary rather than fetched from
      Google, since the app has to work offline; `Span` for the timeline, which
      draws four hundred marks without shipping four hundred summaries;
      `Standing`, which folds a project's meetings into one document so that the
      same commitment made three times is one line saying so.

      `list()` is now variadic, which removed the one place a number was
      concatenated into SQL text.

- [x] **The reader, restructured.** Title, voice map, player, then tabs over a
      chapter strip, with the meeting's shape and its voices on the right.

      The voice map is the piece that was missing: one segment per turn, as wide
      as the turn was long, in that person's colour. The app has had the data
      since diarization worked and never drew it. An hour where one person
      speaks in three long blocks looks nothing like an hour of real
      back-and-forth, and the difference is now visible at a glance. Hovering a
      voice dims every turn that is not theirs.

      Nothing was written twice: `Shape` became the right rail rather than a new
      speaker panel, and the chapter list moved out of the summary into the
      strip instead of appearing in both.

- [x] **Stop stops.** The button in the corner did nothing for every meeting the
      app started by itself — which is almost all of them. `Force(false)` only
      set `stopping` when the recording had been forced *on*, so an automatic
      recording ignored it entirely. Two tests now cover both ways in.

- [x] **The echo, properly this time.** Ducking was a fixed fraction of the
      microphone, which says nothing about how loud the echo lands next to the
      voice it echoes. Measured on a real meeting: the microphone ran at 0.053
      against the tap's 0.028, so a duck to a quarter left the room's copy only
      6 dB under the far side — the worst tenth sat at −4.5 dB, plainly audible
      215 ms late. Ducking is now relative to the tap (`Under`), which holds it
      at a steady −18.4 dB however loud the speakers were.

      A second hole came out of the test that pinned it: with the speakers very
      loud the room copy exceeded four times the tap, the level test read that
      as "he must be talking" and stopped ducking — exactly where the echo is
      worst. Level cannot tell a loud echo from a voice, so the fold now asks
      whether the microphone *looks like* the tap arriving late, reusing the
      alignment already written for the canceller. Bounded by `Reach`, because a
      tap forty times quieter cannot be echoing at that volume — without which a
      notification chime read as an echo and ducked somebody mid-word.

      Cached playback copies carry a version in their name now. A stale cache is
      how a fixed echo goes on being heard.

- [x] **Diarization: split freely, rejoin by identity.** Measured on two real
      recordings whose answers are known, and they do not meet: the meeting of
      two needs 1.10, the meeting of six needs 0.90, and 1.10 finds three of the
      six. There is no threshold that serves both.

      Rejoining by identity alone was the first attempt and it was worse than
      the disease: `Match` at 0.55 asks "could this be Marta", and in a room of
      six the answer is yes four times over. That meeting came back as two
      people with the owner folded in among them. Two clusters must also sound
      like *each other* — `Rejoin` at 0.75, measured, where the four candidates
      sat at 0.686 to 0.737 — and the owner's own label is never the one folded
      away, because the microphone channel is the one thing the app is sure of.

      Result on that meeting: five speakers where there had been three, with
      Dima, who had been missing altogether, back at five minutes of speech.

      So the choice is which mistake to make, and only one is recoverable.
      Splitting one voice in two can be undone afterwards — the app knows what
      Marta sounds like. Folding two people into one cannot. The threshold is
      back to 0.90 and `store.Same` rejoins whatever clusters match the same
      enrolled person; anybody the app has never been introduced to stays a
      speaker of their own, which is the honest answer for the three people in
      that meeting who are not enrolled yet.

## Now

- [ ] **Echo cancellation does not earn its place, and the owner should decide
      whether it goes.** Now measured on a recording made after the mixer was
      sample-locked, and on all 21 real meetings.

      The mixer fix worked: the delay between the channels holds at -213 to
      -224 ms across a whole minute where it used to jump. Alignment finds it
      correctly. Everything downstream of that is the problem.

      On the probe recording the filter *diverges* — at the shipped step of 0.5
      it makes the block 3.4 and 9.1 dB louder, and only the guard saves it.
      Sweeping the step: 0.1 -> +0.4 dB, 0.02 -> +0.8, 0.005 -> +0.7/+0.8,
      0.001 -> +0.7/+1.4. The ceiling is about 1.4 dB, which is inaudible.

      Across every meeting the guard accepts 26 of 1092 blocks — 2.4% — and on
      every meeting longer than a few minutes the best block improves by 0.0 to
      0.9 dB. The two figures above 4 dB are both on recordings of three and six
      blocks, which is noise.

      The reason is physical rather than a bug: the coherence between the
      microphone and the tap is 0.17-0.41, so roughly 90% of what the microphone
      hears is not linearly predictable from the tap, and NLMS on that material
      is guaranteed to wander. macOS appears to cancel the echo before we ever
      see it — on the probe the microphone drops 17 dB at the 30-second mark
      while the tap holds level, which is a voice-processing filter converging.
      What is left is the non-linear residue, which is exactly the part a linear
      filter cannot touch.

      So `Cancel` costs about 26 seconds of alignment per meeting and returns
      the microphone unchanged 97.6% of the time. The recommendation is to
      delete `internal/media/echo.go` and keep the ducking in `Fold`, which the
      owner confirmed fixed the audible echo. Left in place pending that call,
      since it was an explicit request.
- [ ] An 18-second stray survives on a 26-minute meeting. Absorbing it would
      mean a threshold relative to the recording's length, and one file is not
      enough to fit that on.
- [ ] **1.10 may under-count a crowded meeting.** On two 65-minute recordings it
      finds 3 speakers where 0.9 found 8 and 9 — and pyannote says one of them
      has 6 people in it. Neither number is ground truth and pyannote
      over-segments the same way 0.9 does, but the direction is a real risk:
      the threshold was fitted on meetings of two and three. It is the right
      default for the meetings this app actually records; an all-hands may come
      back short. Settling it needs a recording where somebody counted.
- [ ] Several people in one room are one voice, because they arrive on one
      channel. The same limitation the Python edition has.

## Then

- [ ] Windows and Linux: `system_other.go` is written and has never been run
- [ ] Export to SRT/VTT/DOCX — `Markdown` is the only export today

## Notes worth keeping

- **The two VAD models are not the same one.** `vad.bin` is ggml, lives inside
  whisper.cpp, and answers about a finished file — without it Whisper wrote
  "Дякую." over the first ninety seconds of silence. `silero_vad.onnx` runs
  through sherpa-onnx and answers about the last 32 ms, which is what decides a
  meeting is happening. Neither replaces the other.
- **Silero through sherpa, not a second runtime.** The daemon drives Silero with
  `yalue/onnxruntime_go` and ships a 41 MB `onnxruntime.dylib` beside the binary.
  sherpa-onnx is already linked in for the speaker models and carries its own
  runtime, so this edition asks it instead — one dependency fewer, and its
  wrapper handles the 64-sample context that made a bare 512-sample hop score
  0.003 on real speech.
- **The bundle carries its dylibs.** Go links `libsherpa-onnx-c-api.dylib` and
  `libonnxruntime.dylib` through an rpath into the Go module cache, which is
  useless on anybody else's machine. `make bundle` copies both into
  `Contents/Frameworks` and rewrites the rpath; `otool -l` should show
  `@executable_path/../Frameworks` and nothing else.
- **`open` does not pass the environment.** `OPENAI_API_KEY` reaches a process
  started from a shell and not one started from Finder, so the key belongs in
  Settings. The app now says so at startup and on the screens that need it,
  rather than quietly skipping every summary.
- **The live transcript was almost free.** The detector already cuts the audio
  where somebody stopped talking, because it has to know that anyway. Those
  finished segments are exactly what a transcriber wants to be fed, so the
  feature is one goroutine and a bounded list — no second VAD, no second model,
  no new audio path.
- **The disk image window needs Finder.** Setting a background and icon
  positions is only possible through Finder, which needs a one-time Automation
  permission for whatever runs `make`. Without it `make dmg` still produces a
  working image, just an unstyled one, and says so.
- **The channels are the diarizer.** Left is the microphone, right is the
  system tap, and that says more about who is speaking than any clustering
  algorithm can work out from the sound. Measured before touching it: on a
  seven-minute recording of one person, sherpa returned 7 speakers at a
  clustering threshold of 0.7, 4 at 0.9 and 1 at 1.2 — there is no threshold
  that is right for both that recording and a real meeting. The channel is not
  a guess.
- **A merge pass on voiceprints was tried and rejected.** Embedding each
  cluster and joining the ones that sound alike collapses a solo recording to
  one speaker at every threshold — and collapses six real people to one or two
  at the same thresholds, because their cluster voiceprints sit at 0.49–0.80
  and one person split in three sits at 0.59–0.81. The ranges overlap
  completely. Pooling per segment instead of splicing the audio made it worse,
  not better. Do not re-litigate without new numbers.
- **Averaging two channels was the bug behind three others.** Without headphones
  the microphone hears the far side coming out of the speakers, so the average
  of the two channels is a signal summed with a delayed copy of itself. That is
  an audible echo on playback, comb filtering for Whisper, and a smeared energy
  comparison for speaker attribution. `media.Fold` takes the tap whenever the
  tap is active and the microphone otherwise. Measured on a real meeting:
  correlation with the clean tap 0.605 averaged against 0.898 folded, and the
  averaged signal was *quieter* than either channel because the copies cancel.
- **Parakeet was rejected on a broken measurement.** Fed the averaged signal it
  produced three words in three minutes, which is what put it in the "rejected"
  column. On the folded signal it produces 161. It is still not the default:
  Whisper found 320 words on the same three minutes, in Ukrainian, where
  Parakeet wrote Russian — sherpa's transducer takes no language argument, so
  there is nothing to set that stops it. It is in the settings so the numbers
  can be checked rather than believed.
- **A settings file must never stop the app.** Changing `summarise` from a bool
  to a string made every machine that had already written the old file refuse to
  start: the decode failed, Load returned the error, and the window never opened.
  Worse, the reason was invisible — the log file is closed by `run`'s defers
  before `main` gets the error, so the one message that mattered went to a closed
  file. The logger now lives in `main` and writes to stderr as well, an
  unreadable key costs that key and not the app, and `home.Choice` reads the old
  bool as what it meant. `home_test.go` pins all of it.
- **A stronger speaker embedding is not the fix, and this was measured.** Every
  candidate sherpa publishes was tried on 26 seconds of three known, obviously
  different people: WeSpeaker resnet221_LM and resnet293_LM both return **one**
  speaker, CAM++_LM and ERes2NetV2 return two, and the CAM++ that ships returns
  two. The ResNets look excellent on recordings whose answer is 1 or 2 precisely
  because they merge everything. The threshold, not the model, was the lever.
- **Long recordings need a looser threshold than short ones.** A person's voice
  drifts further across half an hour — distance from the microphone, a cold, a
  headset swapped — than two people differ in a clean thirty-second clip. That
  is why 1.10 is right for meetings and wrong for studio clips, and why no
  single number can serve both.
- **Run it from the bundle, never as a bare binary.** macOS attributes the
  microphone and system-audio grants to the responsible application; started
  from a terminal, the grant goes to the terminal.

## Open questions

- Agents SDK: using `openai-go/v3` behind an `Ask`/`Structured` seam. The Go port
  of the Agents SDK is v0.1.0 and five months stale. Revisit when the roadmap's
  agentic items arrive.
