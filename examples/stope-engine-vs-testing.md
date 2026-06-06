# calque -- dual-path suspects

boundary: `engine*.py`  ×  `testing.py`
suspect pairs: 18

Each row is a candidate, ranked by contract-invariant signal overlap. calque is recall-only -- adjudicate each as drift / contracted-twin-ok / false-alarm, then record the verdict in the registry.

## 1. 1.00  `GameEngine._sync_ruin_cap` (engine.py:923)  ≟  `ScriptedGame._sync_ruin_cap` (testing.py:2690)
- shared-strings=1; name~1.00(cap+ruin+sync); shared-calls=1

## 2. 0.79  `GameEngine._load_schedules` (engine.py:1636)  ≟  `_load_schedules` (testing.py:39)
- name~1.00(load+schedules); shared-calls=7; shared-strings=3

## 3. 0.52  `ExploreMixin._find_physical_object` (engine_explore.py:2728)  ≟  `ScriptedGame._find_physical_object` (testing.py:2391)
- name~1.00(find+object+physical); shared-calls=1

## 4. 0.49  `AdversarialMixin._handle_mislead` (engine_adversarial.py:1162)  ≟  `ScriptedGame.mislead` (testing.py:2114)
- name~1.00(mislead); shared-strings=5; shared-calls=1

## 5. 0.49  `ProgressionMixin._check_chain_observation` (engine_progression.py:2254)  ≟  `ScriptedGame._check_chain_observation` (testing.py:1451)
- name~1.00(chain+observation); shared-calls=1; shared-strings=1

## 6. 0.45  `GameEngine._seed_all_manifests` (engine.py:825)  ≟  `ScriptedGame.seed_manifest_file` (testing.py:2290)
- shared-calls=8; shared-strings=10; name~0.20(seed)

## 7. 0.44  `ExploreMixin._refresh_location_refs` (engine_explore.py:5637)  ≟  `ScriptedGame._reveal_refs_for_location` (testing.py:464)
- shared-calls=11; name~0.50(location+refs); shared-strings=8

## 8. 0.42  `ReachMixin._handle_tear` (engine_reach.py:471)  ≟  `ScriptedGame.tear` (testing.py:2095)
- name~1.00(tear); shared-strings=4

## 9. 0.41  `GameEngine._record_suspicion` (engine.py:6680)  ≟  `ScriptedGame.inject_suspicion` (testing.py:2807)
- shared-writes=['world.suspicion']; name~0.33(suspicion)

## 10. 0.40  `SocialMixin.get_dialogue_options` (engine_social.py:864)  ≟  `ScriptedGame.get_dialogue_options` (testing.py:639)
- name~1.00(dialogue+options); shared-calls=2

## 11. 0.39  `AdversarialMixin._handle_warn` (engine_adversarial.py:776)  ≟  `ScriptedGame.warn` (testing.py:2068)
- name~1.00(warn); shared-strings=2; shared-calls=1

## 12. 0.38  `EquipmentMixin._handle_radio` (engine_equipment.py:41)  ≟  `ScriptedGame.radio` (testing.py:789)
- name~1.00(radio); shared-calls=4; shared-strings=7

## 13. 0.36  `AdversarialMixin._handle_betray` (engine_adversarial.py:517)  ≟  `ScriptedGame.betray` (testing.py:2007)
- name~1.00(betray); shared-calls=1

## 14. 0.35  `ExploreMixin._handle_move` (engine_explore.py:1254)  ≟  `ScriptedGame.move` (testing.py:363)
- name~1.00(move); shared-writes=['world.current_loc', 'world.current_zone']; shared-calls=10; shared-strings=2

## 15. 0.34  `ProgressionMixin._check_place_memory` (engine_progression.py:1748)  ≟  `ScriptedGame.fire_place_memory` (testing.py:1237)
- name~0.67(memory+place); shared-strings=2; shared-calls=1

## 16. 0.34  `EquipmentMixin._handle_dos` (engine_equipment.py:279)  ≟  `ScriptedGame.dos` (testing.py:763)
- name~1.00(dos); shared-calls=2; shared-strings=1

## 17. 0.34  `SnoopingMixin._check_tampering` (engine_snooping.py:331)  ≟  `ScriptedGame.inject_examine` (testing.py:2849)
- shared-writes=['ctx.examine_counts[]']; shared-calls=2

## 18. 0.34  `EquipmentMixin._handle_call` (engine_equipment.py:426)  ≟  `ScriptedGame.call` (testing.py:831)
- name~1.00(call); shared-strings=1
