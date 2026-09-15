# AGENTS.md

This file applies to the entire repository.

## Product

Meeting Transcriber is a local-first macOS desktop app that records meetings,
separates speakers, creates transcripts, extracts decisions and action items,
maintains project memory, and exposes the archive to AI clients through MCP.

The application is:

- Go with Wails v3;
- React 19, TypeScript, Vite, Tailwind CSS, Motion, and Lucide on the frontend;
- SQLite-backed;
- packaged as one macOS application bundle;
- designed for Ukrainian-speaking teams that naturally mix English technical
  terms into their work.

## The standard

Do not deliver the first acceptable implementation. Deliver the version that
makes the product feel inevitable.

For visual work, “modern” is not a style. Establish a strong concept, make the
product itself the visual evidence, and remove anything that does not strengthen
the story.

The successful product-demo reference is `demo.html`. Preserve its quality bar,
not its exact composition. Future work should feel related but must not become a
template with different text.

## Source priority

1. The current user request.
2. This file.
3. Actual source code and the running product.
4. Installed library APIs and build configuration.

Do not let generic templates, stale plans, old mockups, or unrelated Markdown
files dilute a clear request. Read another document only when the task directly
requires facts from it. For implementation truth, prefer executable code,
runtime behavior, and installed versions.

## Product interface character

The interface is a quiet instrument, not a cheerful SaaS dashboard.

- Dark, focused, precise, and native to macOS.
- Dense enough for daily work, but never cluttered.
- Violet is the identity color. Cool near-neutral surfaces make it feel vivid.
- Bright green and coral are functional status colors, not decoration.
- Typography must handle Ukrainian properly. Geologica is the established
  product face.
- State facts without judging people.
- No exclamation marks, congratulatory language, “Oops”, or chatty helper copy.
- Every wait must have a named visible state.
- Errors must say what happened and what the user can do.
- Prefer fewer boxes, borders, labels, and words.
- Do not automatically move the user between tabs.
- Destructive actions must be quiet, reversible, keyboard-accessible, and must
  not cover or move content when they appear.
- Hover may enhance an action but must never be its only entry point.

## Interaction and motion

Motion must explain change, reveal structure, or create narrative tension. It
must not exist merely because animation is available.

Prefer:

- scroll-linked transforms;
- masked or clipped reveals;
- sticky scenes with internal progression;
- perspective and layered depth;
- coordinated entrances and exits;
- spatial continuity between related states;
- subtle ambient motion;
- short physical easing with deliberate deceleration;
- opacity and transforms over layout-triggering animation.

Avoid:

- every element fading upward;
- bouncing or elastic easing;
- random particles unrelated to the product;
- spinning gradients used as decoration;
- gratuitous glass cards;
- endless carousels;
- horizontal slide decks;
- scroll hijacking that ignores user intent;
- animation that delays access to content;
- motion that makes screenshots harder to read.

Always support `prefers-reduced-motion`. The reduced-motion version must remain
complete and understandable, not blank or half-revealed.

## Cinematic HTML presentations

When asked for a demo, pitch, showcase, landing experience, or HTML
presentation, use the following standard.

### Format

- Produce one self-contained HTML file unless the user explicitly asks for a
  framework project.
- Embed screenshots, icons, fonts, CSS, and JavaScript.
- Do not depend on CDNs, remote fonts, external scripts, or network access.
- Keep the result directly openable with `open path/to/file.html`.
- Prefer WebP for embedded screenshots.
- Keep the file below 1 MiB when practical. Never bypass the repository's
  large-file check; optimize the artifact instead.

### Narrative

Do not make a conventional deck. Build one continuous vertical experience.

- Scrolling is the timeline.
- A wheel or trackpad movement should reveal, assemble, separate, focus, or
  dismiss information rather than merely translate the page downward.
- Use tall scene containers with sticky full-viewport stages.
- Derive normalized per-scene progress and expose it through CSS custom
  properties.
- Let one scene visually transform into the next.
- Use 6–9 strong scenes rather than many weak sections.
- Give the story an arc: problem → transformation → proof → strategic value →
  trust → memorable ending.

The viewer should understand the value without a presenter explaining every
screen.

### Content

- Use minimal copy and maximum visual proof.
- Headlines should be short enough to say aloud in one breath.
- Every sentence must earn its place.
- Do not add generic claims such as “revolutionize your workflow”, “seamless
  experience”, or “unlock the power of AI”.
- Promote outcomes, not implementation details.
- Show the actual app instead of drawing fake dashboard cards.
- Use concrete product data, decisions, owners, dates, questions, and sources.
- Never fabricate measured savings or adoption claims.

### Real product screenshots

Real screenshots are mandatory when the product can be run.

1. Start the current app or its design-data mode.
2. Set a deliberate viewport.
3. Navigate to representative states.
4. Populate interactions when needed: enter a search query, ask a question, open
   a transcript, select a project, or expose integration settings.
5. Move the pointer away and wait for hover effects to settle.
6. Capture several distinct states, not variations of one screen.
7. Inspect every screenshot before embedding it.

Use screenshots as scene material:

- crop them;
- layer them;
- move them through depth;
- reveal details with masks or lenses;
- extract key facts into the foreground;
- preserve enough resolution for the UI to remain credible.

Do not present screenshots as a plain gallery or a row of laptop mockups.

### Creative direction

Choose a concept before writing markup. The concept should connect directly to
the product.

Good examples for this product:

- spoken fragments condensing into institutional memory;
- a meeting becoming decisions, owners, and open questions;
- a lens searching across transcripts, notes, and projects;
- a stack of meetings resolving into one current project state;
- MCP clients orbiting the same local knowledge source;
- sound becoming a persistent visual signal.

The concept must determine layout, motion, rhythm, and transitions. Do not apply
effects independently.

### Visual system

- Use one dominant background family and one clear identity accent.
- Use large scale contrast: cinematic headlines against small operational
  labels.
- Prefer asymmetry and controlled off-screen composition over centered card
  grids.
- Build depth with scale, perspective, occlusion, blur, and contrast—not heavy
  drop shadows alone.
- Use ambient texture sparingly to avoid sterile flat color.
- Keep screenshots visually dominant.
- End with one memorable line and a strong color-field change.

## Required presentation workflow

Do not skip steps.

1. **Inspect the actual product**
   - Read the relevant UI code and data shapes.
   - Run the current interface.
   - Identify the product outcomes that matter to the requested audience.

2. **Capture evidence**
   - Capture real screens at a consistent high-resolution viewport.
   - Include at least four substantially different product states.

3. **Define the story**
   - Write the scene sequence before styling.
   - Remove any scene that repeats the previous claim.
   - Keep supporting copy intentionally sparse.

4. **Build**
   - Implement the complete experience in one coherent pass.
   - Use native browser capabilities before adding libraries.
   - Keep JavaScript small and deterministic.
   - Animate `transform` and `opacity` whenever possible.

5. **Optimize**
   - Convert screenshots to an efficient format.
   - Embed each unique asset only as many times as necessary.
   - Remove dead CSS, placeholders, external dependencies, and unused captures.

6. **Verify in a real browser**
   - Open the final file, not a partial development proxy.
   - Scroll through every scene.
   - Check intermediate progress, not only scene starts.
   - Test desktop and mobile viewports.
   - Confirm there is no horizontal overflow.
   - Confirm every embedded image has non-zero natural dimensions.
   - Confirm console errors and page errors are zero.
   - Confirm no remote resource requests occur.
   - Emulate reduced motion and verify the entire story remains visible.
   - Collect screenshots of the presentation itself as evidence.

## Frontend implementation rules

- Reuse existing product primitives when editing the app.
- Add an abstraction only when it removes real repeated behavior.
- Keep components direct and readable.
- Do not create wrapper components that only rename props.
- Avoid broad state-management machinery for local state.
- Preserve keyboard access, ARIA names, focus visibility, and native semantics.
- Use icon-only controls only when they have accessible labels and tooltips.
- Do not hide absent-data problems behind broad catches or silent empty states.
- Respect the established compact desktop layout.
- Do not redesign unrelated screens while implementing one feature.

## Backend implementation rules

- Prefer direct, idiomatic Go.
- Keep packages and functions small and responsibility-driven.
- Share the existing database and service objects rather than introducing
  parallel stores or coordinators.
- Do not add factories, registries, interfaces, or wrappers for hypothetical
  future requirements.
- Validate inputs at the boundary and return useful errors.
- Do not swallow failures.
- Preserve local-first behavior and avoid exposing secrets.
- MCP tools must remain explicit about read-only and destructive behavior.
- Measure performance claims; do not present estimates as measurements.

## Commands

Run from the repository root unless stated otherwise.

```bash
# Build the frontend
cd frontend && npm run build

# Run the frontend in development mode
cd frontend && npm run dev

# Build the Go executable
make

# Build and open the macOS app bundle
make run

# Build the distributable app bundle
make bundle

# Build the DMG
make dmg

# Run Go vet and tests
make test
```

Use the smallest validation command that covers the change, then expand only if
the result indicates broader risk.

## Definition of done

A task is not done because the file exists.

For product UI:

- the requested behavior works in the native app or browser;
- loading, empty, error, and success states are coherent;
- keyboard and pointer interactions work;
- there are no console errors;
- the changed screen has been visually inspected.

For a presentation:

- the story is understandable with minimal text;
- actual product screenshots are used;
- scrolling controls meaningful visual change;
- every scene earns its place;
- desktop and mobile work;
- reduced motion works;
- the file is self-contained;
- the final artifact passes repository checks;
- the result has been opened and viewed from beginning to end.

Before finishing, re-read the result and remove anything generic, repetitive,
decorative without purpose, or recognizably produced from an AI landing-page
template.
