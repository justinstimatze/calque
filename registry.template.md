# calque registry

Durable record of adjudicated dual-path pairs for this repo. Lives at
`.calque/registry.md`. This is the memory that survives context resets: grep it
before assuming two paths are independent, and update it whenever a pair is
fixed, cleared, or newly found.

Each entry:

```
## <pair id>  — <verdict>
- left:  <file>::<qualname>
- right: <file>::<qualname>
- signal: <what calque fired on>
- verdict: drift | contracted-twin-ok | false-alarm
- policy: collapse-to-single-path | differential-test | none
- note: <one line — why, and the shared source if both delegate>
- reviewed: <date> by <who>
```

Verdicts:
- **drift** — same contract, behavior diverges. Fix = collapse to single-path.
- **contracted-twin-ok** — intentionally parallel, currently in sync. Pin with a
  differential test so it stays that way.
- **false-alarm** — coincidental signal overlap; suppress from future triage.

---

<!-- entries below -->
