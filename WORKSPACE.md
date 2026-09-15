# The workspace

The plan for rebuilding the app's main screen on the design in
`02-efir.html`, which the owner picked as the direction.

## What that design is, and why it beats what I had been drawing

Every mockup I made was a **hero chart with a list bolted underneath**. This is
not that. It is a **workspace**: a three-pane desk you can live in all day,
where the chart is a small, subordinate instrument at the top rather than the
point of the screen.

That inversion is the whole lesson. A person opening this app is not admiring
their year — they are looking for what was said and what they owe somebody. The
picture of the year is worth 150 pixels, not 350.

Four ideas in it are better than anything I proposed:

- **The voice map.** A horizontal track under each meeting showing who spoke
  when, one colour per person, clickable to jump. The app has had the data since
  diarization worked and has never drawn it. It answers "did she actually say
  that" faster than reading.
- **The dock.** Projects as a floating row of tiles at the bottom, with spring
  motion and a peek label. This is the answer to the thing I failed at three
  times: it is not a tab bar, not a dot, not a chip. It is a persistent object
  with a place of its own.
- **The range strip.** A brush with draggable grips under the timeline. Time is
  filtered by dragging a window, and everything below obeys it. No filter panel.
- **The living document.** The project view is a document with commitments,
  decisions (including reversed ones) and an open question, each carrying how
  many meetings mentioned it and a link back to the source. This is exactly what
  `PROJECTS.md` describes, drawn.

Density is right for a tool that is open all day: 46px topbar, 30px date
headers, 69px record rows, 11–12px type. Colour is identity — a person, a
project — never decoration, with one warm accent (`#f19c79`) used sparingly.

## What it costs

The reference is one 108 KB file: ~60 KB of view code with fictional data. That
view code is thrown away — it gets rebuilt in React against the real bindings.
What is kept is the layout, the density, the palette, and the four ideas above.

This replaces **both** existing screens: `Library.tsx` and `Transcript.tsx`
collapse into one workspace. That is the largest single change the app has had.

## Where it is

Phases 0 to 3 are built and running. Phase 4 — the model-maintained document —
is next, then the polish in phase 5.

The reader now reads as one: title, the **voice map**, the player, then tabs
over a chapter strip, with the meeting's shape and its voices in a rail on the
right. Hovering a stretch of the voice map dims every turn that is not that
person, so one voice can be followed down the page without reading a word.

`Shape` was reused for the right rail rather than a second speaker panel being
written — it already measured share, pace, overlap and questions asked, and a
second one would have been the same numbers computed twice. The chapter list
that used to sit three blocks down inside the summary is now the strip, and was
deleted from the summary rather than left in both places.

The reference's left navigation was **not** taken: Today / Search / To do / Ask
/ Settings stay in the rail, and this workspace is the Meetings screen. That
also removed the whole "two navigations for a while" risk from the plan below —
`Library.tsx` and the full-screen `Transcript.tsx` collapsed into one screen in
a single step, and nothing else moved.

Design mode, for working on this without waiting for a build: put
`VITE_DESIGN=1` in `frontend/.env.development.local` and reload the dev server.
The sample data in `src/sample.ts` covers every call the workspace makes.

## Phases

Each phase leaves the app working. Nothing here is merged until the phase before
it is verified in the built bundle.

### 0 — Groundwork (half a day)

- Bundle the two fonts locally (Geologica, Geist Mono) rather than loading from
  Google — the app must work offline, which is half its point.
- Port the palette into `index.css` as tokens, converted to oklch to match the
  rest of the codebase.
- Keep the existing screens untouched and reachable while the workspace is built
  beside them.

### 1 — The shell (2 days)

Topbar, timeline, three-pane desk, dock. No new backend.

- `Workspace.tsx` — the grid: topbar / timeline / desk(records | reader) / dock.
- `Timeline.tsx` — rows are projects, columns are days, marks are meetings.
  Fed by one new service call (below). The range strip with draggable grips.
- `Dock.tsx` — project tiles, spring motion, peek label, drop target for filing
  a meeting (the drag already works; the target moves here).
- `Records.tsx` — the chronological list with sticky date headers and row tools.

**Backend, one addition:** `Span(from, to)` returning a compact row per
recording — id, started, duration, folder, kind — for the timeline. `Recent(400)`
would work but sends transcripts and summaries nobody draws.

### 2 — The reader (2 days)

The middle pane becomes the meeting, replacing the `Transcript` screen.

- Title, meta, **voice map** built from `turns` (start, finish, speaker), one
  colour per person from `colours.ts`, clickable to seek.
- Tabs: Підсумок / Розшифровка. Chapter strip from `summary.chapters`.
- Speaker panel on the right: share, meter, analytics — `Analytics(id)` already
  returns this.
- The player keeps the waveform it has; the voice map sits above it.

**Backend: nothing new.** Everything is already returned by `Open(id)`.

### 3 — The project view, without a model (2 days)

The middle pane becomes the project.

- The document, assembled mechanically: commitments are the action items of
  every meeting in the project, grouped by identical text so "N згадок" is a
  count rather than a guess; decisions and open questions likewise.
- Participants rail: `Appearances` and per-person hours — mostly built already.
- Project settings in place: rename, colour, per-project toggles, disband.

**Backend:**
- `groups` gains `auto_summary` and `voice_names` columns.
- `ProjectState(group)` returning the mechanical aggregate.

This is already useful and works with no OpenAI key.

### 4 — The living document (the feature)

Replace the mechanical aggregate with the model-maintained state from
`PROJECTS.md`: operations against stable ids, provenance, pinned human edits,
rebuild. The view from phase 3 does not change shape — only its source.

### 5 — Polish

Focus mode, the four dialogs, keyboard navigation, quicklook on hover, toasts,
`prefers-reduced-motion` and `prefers-contrast` (the reference handles both and
the app currently handles neither).

## Decisions I need from the owner before phase 1

1. **Does the sidebar go?** The reference drops the left nav into the dock plus
   a breadcrumb. Today / Search / To do / Ask / Settings have to live somewhere.
2. **Notes.** A dictated or solo note has no speakers and no voice map. Does it
   get the same reader with the empty parts hidden, or its own simpler one?
3. **Phase order.** The shell first is the safe route. Doing the reader first
   would put the voice map in your hands within two days but leaves the app with
   two navigation systems for a while.
