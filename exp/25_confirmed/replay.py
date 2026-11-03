"""Experiment 25 — which voice samples may recognise a speaker (2026-11-02).

    python3 -I exp/25_confirmed/replay.py <archive snapshot.db> <logs/mt.log> [recording ...] > exp/out/25-confirmed.md

Recording 322 named its main speaker after somebody who was not in the meeting.
Every voice the app recognised had been filed as another sample of that person,
pushing out the samples the owner had named by hand, and a rename never took a
voice back from the name it left. This replays every transcription the app log
records, in order, against the voiceprints stored in an archive snapshot:

  before  as shipped until now: every recognised voice is filed; a full profile
          drops the older of its two most alike samples; a rename does not
          unlearn; every sample recognises
  all     the owner's rules of 2026-11-02: what the owner named or confirmed is
          never dropped; the app's guess only takes a free slot of ten; a
          rename takes the voice back from the old name; every sample
          recognises
  sure    the same rules, but only confirmed samples recognise

Nobody confirms a guess during the replay, so `sure` is its lower bound. The
truth is every name the owner gave by hand (a name in a recording that the log
says the recogniser did not give, "-" meaning nobody), plus corrections in
exp/truth/25-corrections.json: {"322": {"<label>": "<name>" or null}}.
Names never leave the script: people print as P1… in alphabetical order.
"""

import json
import math
import re
import sqlite3
import sys
from collections import Counter
from pathlib import Path

db = sqlite3.connect(f"file:{sys.argv[1]}?mode=ro", uri=True)
detail = [int(r) for r in sys.argv[3:]]
corrections = Path(__file__).resolve().parents[1] / "truth" / "25-corrections.json"
fixes = json.loads(corrections.read_text()) if corrections.exists() else {}

cos = lambda a, b: sum(x * y for x, y in zip(a, b)) / math.sqrt(sum(x * x for x in a) * sum(y * y for y in b))
names = sorted(n for (n,) in db.execute("SELECT name FROM people") if n != "Me" and any(c.isalpha() for c in n))
code = {n: f"P{i + 1}" for i, n in enumerate(names)}
show = lambda who: "nobody" if who is None else code.get(who, who if who.startswith("SPEAKER_") else "?")
stored = {r: json.loads(raw) for r, raw in db.execute("SELECT id, voices FROM recordings WHERE voices IS NOT NULL")}
held = {(r, s): d for r, s, d in db.execute("SELECT recording, speaker, SUM(finish - start) FROM turns GROUP BY recording, speaker")}

order, auto = [], {}
for line in open(sys.argv[2], errors="replace"):
    if m := re.search(r"transcribing id=(\d+)", line):
        auto[int(m[1])] = set()
        order.append(int(m[1]))
    elif "voices recognised" in line and (m := re.search(r'id=(\d+) who="?\[(.*?)\]', line)):
        auto[int(m[1])] = {n for n in names if n in m[2]}


def run(rule):
    profiles = {n: [] for n in names}  # [print, recording, confirmed]

    def file(name, print_, r, sure):
        s = profiles[name]
        if rule == "before":
            s.append([print_, r, sure])
            if len(s) > 10:
                worst, twin = -2.0, 0
                for i in range(len(s)):
                    for j in range(len(s)):
                        if i != j and (c := cos(s[i][0], s[j][0])) > worst:
                            worst, twin = c, min(i, j)
                s.pop(twin)
            return
        if len(s) >= 10:
            guess = next((i for i, x in enumerate(s) if not x[2]), None)
            if not sure or guess is None:
                return
            s.pop(guess)
        s.append([print_, r, sure])

    given, seen = {}, {}
    for r in order:
        voices = stored.get(r, {})
        prints = {l: p for l, p in voices.items() if l != "Me" and len(p) == 192 and held.get((r, l), 0) >= 30}
        pool = {n: [x[0] for x in s if rule != "sure" or x[2]] for n, s in profiles.items()}
        pairs = sorted(((max(cos(p, q) for q in pool[n]), l, n) for l, p in prints.items() for n in names if pool[n]), reverse=True)
        got, taken = {}, set()
        for c, l, n in pairs:
            if c >= 0.55 and l not in got and n not in taken:
                got[l] = (n, c)
                taken.add(n)
        given[r], seen[r] = got, prints
        for l, (n, _) in got.items():
            file(n, voices[l], r, False)
        for l in voices:  # names the owner gave by hand in this recording
            if l in names and l not in auto[r] and len(voices[l]) == 192:
                old = got.get(l, (None,))[0]
                if rule != "before" and old and old != l:
                    profiles[old] = [x for x in profiles[old] if x[1] != r]
                file(l, voices[l], r, True)
    return given, seen, profiles


truth = {}
for r, voices in stored.items():
    for l in voices:
        if r in auto and l not in auto[r] and (l in names or not any(c.isalpha() for c in l)):
            truth[(r, l)] = l if l in names else None
for r, labels in fixes.items():
    for l, who in labels.items():
        truth[(int(r), l)] = who

results = {rule: run(rule) for rule in ("before", "all", "sure")}

print("# Which samples may recognise a voice\n")
print(f"Generated by `exp/25_confirmed/replay.py` from a private archive snapshot and its app log: "
      f"{len(order)} transcriptions replayed in order, the last being recording {order[-1]}. "
      f"People are P1…P{len(names)}. A label counts once it has a 192-d print and 30 s of speech.\n")
checked = {k: who for k, who in truth.items() if k[1] in results["sure"][1].get(k[0], {})}
people = len({who for who in checked.values() if who})
print(f"The owner's answers cover {sum(1 for w in checked.values() if w)} voices of {people} people, "
      f"and {sum(1 for w in checked.values() if w is None)} voices that are nobody enrolled.\n")
print("| rule | labels named | people's voices: right | wrong | unnamed | nobody's voices: left unnamed | named |")
print("|---|---:|---:|---:|---:|---:|---:|")
for rule, (given, seen, _) in results.items():
    tally = Counter()
    for (r, l), who in checked.items():
        got = given[r].get(l, (None, 0))[0]
        tally[("nobody " if who is None else "") + ("right" if got == who else "unnamed" if got is None else "wrong")] += 1
    print(f"| {rule} | {sum(len(g) for g in given.values())} | {tally['right']} | {tally['wrong']} | {tally['unnamed']} "
          f"| {tally['nobody right']} | {tally['nobody wrong']} |")

held_sure = {n: sum(1 for x in s if x[2]) for n, s in results["sure"][2].items()}
print(f"\nAt the end, under `sure`, {sum(1 for c in held_sure.values() if c)} people hold "
      f"{sum(held_sure.values())} confirmed samples: what the owner's names alone give.")

for r in detail:
    print(f"\n## Recording {r}\n")
    print("| label | speech, s | truth | before | all | sure |")
    print("|---|---:|---|---|---|---|")
    for l in sorted(results["sure"][1].get(r, {}), key=lambda l: -held.get((r, l), 0)):
        cells = []
        for rule in results:
            n, c = results[rule][0][r].get(l, (None, 0))
            cells.append(f"{show(n)} {c:.2f}" if n else "unnamed")
        who = truth.get((r, l), "?")
        print(f"| {show(l) if l in names else l} | {held.get((r, l), 0):.0f} | {'?' if who == '?' else show(who)} | " + " | ".join(cells) + " |")
