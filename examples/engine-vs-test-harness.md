# calque — dual-path suspects (example output)

A representative `calque scan` run over a real Python codebase, across the
boundary where a **production engine** meets a **test harness that reimplements
engine behavior** — the canonical dual-path case (the harness is supposed to
mirror the engine and silently drifts). Symbol names are lightly anonymized.

```
boundary: engine*.py  ×  testing.py
```

Each row is a candidate, ranked by contract-invariant signal overlap. calque is
recall-only — adjudicate each as drift / contracted-twin-ok / false-alarm, then
record the verdict in the registry.

## 1. 1.00  `Engine._sync_cap` (engine.py)  ≟  `TestHarness._sync_cap` (testing.py)
- shared-strings=1; name~1.00(cap+sync); shared-calls=1

## 2. 0.79  `Engine._load_schedules` (engine.py)  ≟  `_load_schedules` (testing.py)
- name~1.00(load+schedules); shared-calls=7; shared-strings=3

## 3. 0.52  `ExploreMixin._find_physical_object` (engine_explore.py)  ≟  `TestHarness._find_physical_object` (testing.py)
- name~1.00(find+object+physical); shared-calls=1

## 4. 0.45  `Engine._seed_all_manifests` (engine.py)  ≟  `TestHarness.seed_manifest_file` (testing.py)
- shared-calls=8; shared-strings=10; name~0.20(seed)

## 5. 0.44  `ExploreMixin._refresh_location_refs` (engine_explore.py)  ≟  `TestHarness._reveal_refs_for_location` (testing.py)
- shared-calls=11; name~0.50(location+refs); shared-strings=8

## 6. 0.40  `SocialMixin.get_dialogue_options` (engine_social.py)  ≟  `TestHarness.get_dialogue_options` (testing.py)
- name~1.00(dialogue+options); shared-calls=2

## 7. 0.35  `ExploreMixin._handle_move` (engine_explore.py)  ≟  `TestHarness.move` (testing.py)
- name~1.00(move); shared-writes=['world.current_loc', 'world.current_zone']; shared-calls=10; shared-strings=2

## 8. 0.34  `ProgressionMixin._check_place_memory` (engine_progression.py)  ≟  `TestHarness.fire_place_memory` (testing.py)
- name~0.67(memory+place); shared-strings=2; shared-calls=1

> Reading the signal: row 1 fires on an exact name-role match plus a shared
> emitted string — the harness and engine both compute the same capped value, a
> prime drift candidate. Row 7's `shared-writes=[world.current_loc, …]` says both
> mutate the same state, so the question to adjudicate is *do they mutate it the
> same way*. High `name~` with low effect overlap is the signature of a real twin
> one side fakes.
