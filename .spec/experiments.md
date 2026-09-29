# Experiments

The `exp/` tree is where uncertain audio, model, and performance ideas are
measured before they affect production.

## Layout

```text
exp/
├── NN_name/   one focused executable or analysis
├── out/       reproducible text results
└── truth/     reviewed ground truth and labels
```

Raw private recordings are local inputs and must not be committed.

## When an experiment is required

Use an experiment before changing:

- channel alignment or echo handling;
- VAD and timestamp behavior;
- Whisper context, decoding, model, or prompt settings;
- diarization model, channel, clustering, or threshold;
- voice-recognition evidence thresholds;
- long-meeting performance or memory behavior;
- any setting justified as “faster”, “more accurate”, or “better”.

UI layout and ordinary application logic do not need an `exp/` executable.

## Method

1. State one falsifiable hypothesis.
2. Record the current shipping baseline.
3. Change one variable.
4. Run on a real meeting and the edge case that broke the previous approach.
5. Define the acceptance metric before reading the result.
6. Write machine-readable or plain-text output into `exp/out/`.
7. Compare quality and speed, not one without the other.
8. Keep a failed result when it prevents the same dead end from recurring.
9. Move code into `internal/` only after it beats the baseline.
10. Summarize the shipping decision in `.spec/decisions.md`.

Do not tune a threshold on one recording and call it a general improvement.

## Running

Experiments are ordinary Go commands:

```bash
go run ./exp/01_echo "/path/to/recording.wav"
go run ./exp/15_rows "/path/to/recording.wav"
go run ./exp/18_glossary
```

Experiments requiring optional native dependencies use build tags. For example:

```bash
brew install speexdsp
go run -tags speex ./exp/05_aligned "/path/to/recording.wav"
```

Run from the repository root so local module and `build/whisper.cpp` paths
resolve consistently.

## Existing evidence

The current experiment set covers:

- echo, channel alignment, and folding;
- Whisper repetition and VAD timing;
- Parakeet and Whisper model comparisons;
- speaker models, channels, and clustering thresholds;
- microphone-owner detection;
- row reconstruction and timestamp accuracy;
- glossary prompting;
- local LLM structured summaries and grounded Q&A;
- recurring unnamed voices across meetings (`exp/20_voices`).

The durable conclusions are in `.spec/decisions.md`; detailed outputs remain in
`exp/out/`.
