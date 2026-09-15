# Experiments

Nothing here ships. Every experiment is one directory with one `main.go`, run by
hand, printing far more than a program normally should — the point is to see the
situation, not to be tidy about it.

    go run ./exp/01_echo    "$HOME/MeetingTranscriber/recordings/meeting 2026-09-09 11-58.wav"
    go run ./exp/02_speakers 74

The rule: an idea is only allowed into `internal/` after it has been measured
here against a real recording *and* against the edge cases that broke the last
attempt. Two rounds of "fixed it" that were not fixed is why this folder exists.

`truth/` holds ground truth the owner gave by ear. It is the only thing in this
repository that knows the right answer.

## What is being chased

**Echo.** The recordings are stereo: left is the microphone, right is the system
tap. Without headphones the microphone also picks up the far side coming out of
the speakers, so the two channels hold the same voice twice, a room delay apart.
Folding them to mono by ducking the microphone was the first attempt; it leaves
a second copy audible on real material.

**Voices.** sherpa-onnx splits the audio into speakers, then `store.Same` merges
clusters that match an enrolled voiceprint. On meeting 74 that merged a speaker
nobody has enrolled into somebody who is — the one mistake the design says must
never happen, because it cannot be undone afterwards.

## Findings so far

**The two channels are not aligned.** The microphone leads the system tap by a
steady ~230 ms for the whole of meeting 74 — the room appears to hear the far
side before it was played, which cannot happen acoustically and means audiotee's
stream is interleaved late. An echo canceller models a *causal* response, so an
echo sitting behind the reference is outside the model: SpeexDSP removed 0.0 dB
and the app's own NLMS removed 0.0 dB, and neither was broken.

Shift the tap 230 ms earlier and SpeexDSP removes 8.1 dB. The sweep peaks
cleanly (0.0 → 3.3 → 8.1 → 6.6 → 5.2), which is what a real alignment looks
like.

`internal/media.roomLag` currently forbids a negative answer, on the reasoning
that sound cannot be heard before it is played. That reasoning is right about
rooms and wrong about this file, and it is one of the things keeping the echo
in: it forces the search into the half-plane the answer is not in.

The experiments needing SpeexDSP carry `//go:build speex`, so `go build ./...`
and `go test ./...` stay clean on a machine without `brew install speexdsp`.
Run them with `go run -tags speex ./exp/05_aligned <file>`.

## Findings, second round

**Whisper repeats itself, and that is not echo.** Meeting 74 held "Данію." ten
times in a row. `max_text_ctx = 0` — condition_on_previous_text off — removes
every repetition and runs a quarter faster. A temperature fallback and beam
search were measured on the same audio and added nothing. Parakeet does not loop
but produced 11 rows for 23 minutes, largely nonsense; closed.

**Speakers: nothing works alone, one pair works.** Nine configurations against
the owner's ear (exp/out/F-speakers-74.txt). Every one on the mixed mono folds
the stranger into the enrolled colleague, including Rev's reverb segmentation
(also 5x slower, and non-production licensed) and both alternative embedding
models. Diarizing the SYSTEM CHANNEL ALONE with the 3D-Speaker zh_en embedding
is the only thing that keeps them apart — and it is faster, because the owner is
not in the audio being clustered at all.

Both are now in `internal/`. pyannote community-1, which the field considers the
current best, has no ONNX export in sherpa-onnx; only segmentation-3.0 and
reverb are available.

## community-1, measured

pyannote community-1 — the best open-source diarizer there is, and the one the
Python edition of this project uses — scored on the same meeting and the same
question as the nine sherpa configurations:

    on the MIXED audio   the stranger folded into the colleague.  Fails.
    on the SYSTEM TAP    kept apart.  Passes.

Which is exactly what sherpa does. The channel mattered and the model did not:
the app is already where it needs to be, and the missing ONNX export of
community-1 has stopped being a problem worth solving.

It is better calibrated — five speakers on the tap against sherpa's fourteen —
but the app splits on purpose and rejoins by voiceprint, so that is not a fault
on this side. Speed on MPS was 35-42x realtime against sherpa's 26x.

The cost of using it would be PyTorch and a Python runtime, which is the whole
thing the single-binary edition exists to avoid. Now there is a measurement
saying that price buys nothing here.

## The clock, and what was actually wrong

The owner ran one meeting through WhisperX to prove the audio was not at fault,
and it was not. Experiments 14 and 15 found the fault.

**Whisper's VAD deletes the silence, and only half the timestamps come back.**
whisper.cpp's built-in Silero VAD strips the silence out of the audio before the
model sees it, so everything the model says is on a shorter clock. whisper.cpp
knows this and maps SEGMENT times back for you — `whisper_full_get_segment_t0`.
It offers mapped TOKEN times too, and its own header says why they exist:

> unlike whisper_full_get_token_data().t0/t1 which stay in VAD-processed time

The Go binding never calls them. `pkg/whisper/context.go` builds `Token.Start`
from the raw struct field, so a `Segment` arrives with its own times on the
recording's clock and its tokens' times on the compressed one. `phrases()` built
every row out of the tokens.

Measured on a 167-second meeting (exp/out/I-timeline.txt): segments end at
167.34, tokens at 143.48. With the VAD off the two agree to 0.00 everywhere.
The gap is flat during continuous speech and jumps at every pause, which is what
removed silence looks like and what a sample-rate bug does not.

**What it cost, and what fixing it bought** (exp/out/J-rows.txt, scored word by
word against the reference with a Needleman-Wunsch alignment — a forward search
matches the wrong "окей" in a meeting that says it five times):

    what ships          a word sat a median 12.34 s from where it was said, speakers 47%
    a row per segment                          0.35 s                        speakers 66%

Four ways of fixing it were measured and they are the same to two decimal places
— segment times, our splitting with the token times stretched onto the segment's
span, whisper.cpp's own `max_len` splitting, and no VAD at all. So the shortest
one ships: a row is a segment, `phrases`/`spoken`/`word`/`ends` are gone, and
`token_timestamps` is not even asked for (`plain` in the table: identical).

`max_len` splitting scores marginally better and produces rows reading "писати."
on their own. Not worth eighty lines.

## Things that were not the problem

**The model.** exp/out/L-models.txt, both clips, against the owner's large-v3
reference — which flatters large-v3, and it still barely matters:

    turbo-q5_0 (574 MB, ships)   83% / 89%   47x realtime
    turbo-f16  (1.6 GB)          85% / 90%   42x
    large-v3-f16 (3.1 GB)        85% / 91%   16x

Three times the size and three times the time for one or two points. The 10-15%
that does not agree is the audio and the language, not the weights.

**A glossary.** exp/out/M-glossary.txt. Whisper's documented cure for "Azure
OpenAI" coming back "ажуру ПНІ" is an initial prompt naming the vocabulary. It
buys nothing here — 83% → 81%, 89% → 88% — and it cannot be had cheaply anyway:
whisper.cpp only reads an initial prompt when `n_max_text_ctx > 0`
(src/whisper.cpp:7216), which is the rolling context that made this app return
"Данію." ten times, and the Go binding does not expose `carry_initial_prompt`,
so the glossary cannot be pinned against the model's own output displacing it.

## Speakers, on a file where the answer is known

exp/out/K-speakers.txt sweeps segmentation × embedding × threshold against
WhisperX's word-level labels on the same meeting, scored on a 100 ms grid with a
one-to-one label mapping chosen by exhaustion rather than greedily.

    pyannote-3.0 + campplus @ 0.90   3 speakers   61%   ← what ships
    pyannote-3.0 + campplus @ 0.70   5 speakers   72%
    pyannote-3.0 + eres2netv2 @ 0.40 5 speakers   83%   (a spike; 0.50 drops to 68%)

Two things to remember before anybody moves the threshold. `Threshold = 0.90` is
deliberately loose because `store.Same` rejoins the clusters afterwards by
enrolled voiceprint, so raw agreement is not the shipping metric. And this is one
file: the 0.90 that is in the code came from two recordings whose answers were
known, and a number tuned on a third is not an improvement, it is an overfit.

Also worth knowing: pyannote-3.0 covers only 92% of the reference's speech at
all, which caps every row in that table. reverb-v2 covers 100% — and is six
times slower and non-production licensed.
