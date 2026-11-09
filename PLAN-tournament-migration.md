# Tournament Feature Migration Plan: RLBotGUI v4 → v5

> Living document. Check off `[x]` items as they are implemented in the v5 fork
> (`D:/RLBotGUIv5`). Items marked **[REVIEW]** need operator input before
> proceeding. Items marked **[DONE]** are implemented.

## 0. TL;DR / Status

The custom v4 fork added a full-featured **tournament system** (single/double
elimination, round robin, Swiss; 1v1–5v5 teams; human LAN participants; mercy
rule; per-player stat capture; match history; statistics; templates; seeding
editor). This feature lives entirely inside the GUI (Python backend + Vue
frontend). It does **not** require changes to `go-interface` or RLBot core — every
packet primitive the v4 code reads (`GameTickPacket.game_cars[].score_info.*`,
`game_info.is_match_ended`, `is_overtime`, `seconds_elapsed`) already exists in
the v5 `go-interface/flat` package (`GamePacketT.Players[].ScoreInfo`,
`MatchInfoT.MatchPhase == MatchPhaseEnded`, `MatchInfoT.IsOvertime`,
`MatchInfoT.SecondsElapsed`).

**Conclusion: the entire tournament feature can be ported into the v5 GUI fork
alone.** No upstream repo changes are required. The only genuinely ambiguous
area is the human/LAN "Continue And Spawn" staging workflow (see §7) — that is
marked for review.

Implementation strategy:
- **Go backend** (`tournament.go`, `bracket.go`, `team.go`, `tournament_match.go`):
  port the Python state machine + bracket algorithms + team formation + result
  capture. Expose as Wails-bound methods returning the serialized state.
- **Svelte frontend** (`pages/Tournament.svelte` + components): port the Vue
  tournament UI. Reuse the existing `StoryMode` page slot (currently a stub).

---

## 1. Architecture: v4 vs v5

| Concern | v4 (custom fork) | v5 (this repo) |
|---|---|---|
| Backend language | Python | Go |
| Frontend↔backend bridge | `eel.expose` (WebSocket JSON) | Wails v3 bound methods + `app.Event.Emit` events |
| Frontend framework | Vue 2 + BootstrapVue | Svelte 5 (runes: `$state`/`$derived`/`$effect`) |
| Match launch | `SetupManager` (Python RLBot lib) | `StartAndWaitForMatch` → `rlbot.Connect` + `flat.MatchConfigurationT` |
| Packet reading | `GameTickPacket` via `game_interface.fresh_live_data_packet` | `conn.RecvPacket()` → `flat.GamePacketT` (goroutine reader, see `sandbox.go`) |
| Persistence | `QSettings("rlbotgui","tournament_save")` | `localStorage` (frontend) — see §8 |
| Page routing | Vue router | `activePage` state in `App.svelte` |

Key v5 reference files:
- `app.go` — `StartMatch`, `StartAndWaitForMatch`, `WaitForMatchReady`, `ReadAllMessages`.
- `sandbox.go` — the **packet-reader goroutine pattern** to copy for result capture.
- `players.go` — `PlayerJs`, `BotInfo`, `PsyonixBotInfo`, `HumanInfo`, `ToPlayerConfig()`.
- `frontend/src/App.svelte` — page routing + nav buttons.
- `frontend/src/pages/StoryMode.svelte` — stub page to replace with Tournament.

---

## 2. v4 Tournament Feature Inventory (what must be ported)

### 2.1 State model (`tournament_state.py`) → `tournament_state.go`
- `Participant { name, participant_id, participant_type ('bot'|'human'), bot_config, seed }`
- `Team { team_id, name, participants[], wins, losses, goals_for, goals_against }`
- `Match { match_id, round_num, participant1/2, winner, score [2]int, completed,
  next_match_id, loser_next_match_id, team1/2, winner_team, player_stats[],
  duration_seconds, is_overtime }`
- `TournamentState { name, tournament_id, format, participants[], matches[],
  current_round, completed, winner, created_at, losers_bracket_matches[],
  match_settings{}, team_size, teams[], winner_team, allow_duplicates,
  swiss_rounds, swiss_playoff_scheduled, map, game_mode, randomize_map,
  scripts[], mercy_rule }`

### 2.2 Bracket generation (`bracket_generator.py`) → `bracket.go`
- `generate_single_elimination_bracket` — power-of-2 padding + standard seeding
  (`get_seed_positions` for 2/4/8/16 + recursive for larger) + cascading byes.
- `generate_double_elimination_bracket` — WB (single-elim style) + LB rounds
  (`num_lb_rounds = 2*num_rounds - 2`) + Grand Final; WB→LB loser linking.
- `generate_round_robin_bracket` — circle method.
- Swiss: `calculate_swiss_rounds = ceil(log2(n))`, `generate_swiss_round1`
  (seed-order pairs), `generate_swiss_next_round` (rank by wins, greedy
  non-rematch pairing), `rank_entities` (wins desc → multi-way head-to-head →
  goal diff → goals for → score), `determine_swiss_winner` (playoff if top-2
  indistinguishable).

### 2.3 Team formation (`team_manager.py`) → `team.go`
- `validate_team_formation`, `form_teams_random` (shuffle+chunk),
  `form_teams_seeded` (snake draft), `form_teams_manual`,
  `_chunk_into_duplicate_teams` (party-mode dup), random team names
  (15×15 descriptor×noun pool), `team_balance_report`, `pair_participants`
  (connected-components manual pairing), `build_match_bot_list` (team0→slots
  0-4, team1→slots 5-9... actually team0 slots 0.., team1 slots 0..).

### 2.4 Runner / orchestration (`tournament_runner.py`) → `tournament.go`
- `tournament_new` (create + auto-form teams + generate bracket)
- `tournament_form_teams` / `tournament_set_team_members` /
  `tournament_reorder_team_member` / `tournament_randomize_team_names` /
  `tournament_rename_team`
- `tournament_start_match` (build bot_list + match_settings, launch)
- `tournament_record_result` (set winner, advance, populate LB, swiss/RR
  progression, completion)
- `advance_winner` + bye handling (`_handle_bye_situation`,
  `_populate_losers_bracket_match`, `_check_and_handle_lb_bye`)
- `tournament_get_statistics`, `tournament_get_match_history`,
  `tournament_get_swiss_standings`
- seeding editor (`tournament_set_participant_seed`, `tournament_swap_seeds`)
- saved-tournaments list + templates (persistence — see §8)
- map pools (`MAP_POOLS`, weighted `pick_random_map`)

### 2.5 Result capture (`match_runner.py::start_match_helper`) → `tournament_match.go`
- Launch match, poll packets until `is_match_ended` (or mercy rule).
- Capture **baseline** per-player stats on first packet (keyed by car index).
- Compute per-team **goal deltas** (not raw team score) so a stale scoreboard
  carried over via Continue-And-Spawn cannot pollute the result.
- Return `{ team_scores[2], players[], duration_seconds, is_overtime }`.
- Mercy rule: when `abs(goal_delta[0]-goal_delta[1]) >= mercy_rule`, end match
  (state-setting) or shut down game (no state setting).

---

## 3. Go Backend Implementation Plan

### 3.1 `tournament_state.go` **[DONE]**
Port the four dataclasses as Go structs with JSON tags matching the v4 keys so
the frontend contract is identical. Use pointer types for optional fields.

### 3.2 `bracket.go` **[DONE]**
Port all bracket algorithms verbatim. Pure functions over `[]Participant` →
`[]Match`. No I/O.

### 3.3 `team.go` **[DONE]**
Port team formation + names + pairings. Pure functions.

### 3.4 `tournament.go` **[DONE]**
- `App` holds `currentTournament *TournamentState` (guarded by a mutex).
- Methods mirror the `@eel.expose` functions but return the serialized state
  (or a typed struct) directly. Wails auto-serializes return values.
- `TournamentNew`, `TournamentFormTeams`, `TournamentSetTeamMembers`,
  `TournamentRecordResult`, `TournamentStartMatch`, `TournamentStopMatch`,
  `TournamentGetStatistics`, `TournamentGetMatchHistory`,
  `TournamentGetSwissStandings`, seeding, templates.

### 3.5 `tournament_match.go` **[DONE]**
- Port `start_match_helper` result capture using the `sandbox.go` reader pattern:
  connect, send `ConnectionSettingsT{CloseBetweenMatches:true}`, read packets,
  baseline stats on first `GamePacketT`, poll until `MatchInfo.MatchPhase ==
  MatchPhaseEnded`, compute goal deltas, emit `tournament:match-complete` event
  with the result, then `TournamentRecordResult`.
- Mercy rule: on goal-delta diff ≥ threshold, send `DesiredGameStateT` with
  `MatchInfo` end flag if state setting enabled, else stop match.

---

## 4. Frontend (Svelte) Implementation Plan

### 4.1 Page slot
Replace `pages/StoryMode.svelte` stub with `pages/Tournament.svelte`, OR add a
new `activePage == "tournament"` route + nav button in `App.svelte`. **Decision:
add a dedicated "Tournaments" nav button** (Story Mode may still be wanted later).

### 4.2 Components
- `Tournament.svelte` — top-level: create form, bracket view, match controls.
- `TournamentBracket.svelte` — render WB/LB/rounds from state.
- `TournamentSetup.svelte` — name, format, team size, human count/names, map,
  game mode, mutators, mercy rule, randomize map, scripts.
- `TournamentHistory.svelte` — round-grouped expandable match stats + export.
- `TournamentStats.svelte` — ranked statistics table.
- `TournamentTemplates.svelte` — saved templates list.

### 4.3 Data flow
- Frontend calls bound methods (`App.TournamentNew(...)`, etc.) via the generated
  `frontend/bindings/gui` wrappers.
- Backend emits `tournament:match-complete` (result) → frontend calls
  `App.TournamentRecordResult(...)` (or backend records internally and emits
  `tournament:state` with the new state).
- State is the single source of truth; frontend re-renders from returned state.

---

## 5. Game Modes & Maps

- v5 `StartMatch` already supports `Soccar/Hoops/Dropshot/Snowday/Rumble/
  Heatseeker/Gridiron/Knockout` via `flat.GameMode*`. The v4 `game_mode` strings
  map 1:1 (`Soccer→Soccar`, `Hockey→Snowday`).
- **Dropshot special handling**: v4 had no special code beyond the game mode +
  its single map (`DropShot_Core707`). v5 already has `GameModeDropshot`. So
  **no workaround needed** — dropshot is natively supported in v5. The v4
  "workaround" was simply that dropshot uses a different scoring model; the
  goal-delta capture still works because Dropshot still reports per-player
  `goals`/`score_info`. **[RESOLVED]** Dropshot scores off `goals` (ball through
  the opened gap), not `score` — confirmed in
  [`PLAN-tournament-migration-user-review.md`](PLAN-tournament-migration-user-review.md:43).
  The v5 core populates tile damage states (`GamePacketT.Tiles`) and ball energy
  (`BallInfoT.ChargeLevel`); the wiki claim that this data was lost is out of
  date. The only remaining Dropshot work is on the BOT side (custombot must read
  the v5 field names), not the GUI.

---

## 6. Persistence

v4 used `QSettings`. v5 has no settings store wired for tournaments. **Decision:
persist tournament list + templates + current save in `localStorage`** (frontend),
mirroring how `BOT_SEARCH_PATHS` is already stored. The Go backend stays
stateless w.r.t. disk; the frontend owns serialization. This keeps the Go port
simpler and matches v5's existing localStorage patterns.

---

## 7. Human / LAN Staging Workflow **[REVIEW]**

v4 Phase 3: for human matches, open RL, let the host create a Rocket Plugin LAN
lobby and humans join, then launch the real match with `Continue And Spawn` so
bots are injected without tearing down the lobby. The v4 code ultimately just
"opens RL and lets the user handle the rest manually" (staging was disabled
because it broke Rocket Plugin LAN hosting).

**Open questions for the operator:**
1. Do we keep the "open RL only, user hosts LAN manually, then Continue And
   Spawn" flow as-is, or wire a richer staging UI?
2. v5 `StartMatchOptions.ExtraOptions.ExistingMatchBehavior` is a `byte` — need
   the numeric value for "Continue And Spawn" to inject bots into a live lobby.
   **[REVIEW]** confirm the enum value (see `flat.ExistingMatchBehavior`).
3. How are humans represented in v5? `HumanInfo` exists in `players.go`. Confirm
   human player config mapping matches v4's `create_player_config` (human_index
   incrementing, `rlbot_controlled=false`).

---

## 8. Out of Scope / Deferred

- Story Mode (still a stub in v5) — not part of tournament, leave stub.
- Custom `.upk` map loading (`custom_maps.py`) — v5 may not need it; **[REVIEW]**.

---

## 9. Implementation Order (execution checklist)

1. [x] `tournament_state.go` — data model.
2. [x] `bracket.go` — all bracket generators.
3. [x] `team.go` — team formation + names + pairings.
4. [x] `tournament_match.go` — result capture (reader goroutine + goal deltas).
5. [x] `tournament.go` — orchestration methods (new/form/record/advance/stats/
   history/swiss/seeding/templates).
6. [x] Wire `App` to hold tournament state; register methods (auto via Wails).
7. [x] `pages/Tournament.svelte` + subcomponents; nav button in `App.svelte`.
8. [x] localStorage persistence layer in frontend.
9. [x] LAN staging / Continue-And-Spawn enum confirmation — see §12 and
    [`PLAN-tournament-migration-user-review.md`](PLAN-tournament-migration-user-review.md:77)
    (`ExistingMatchBehaviorContinueAndSpawn = 1`).
10. [x] Dropshot scoring semantics for mercy rule — goal-delta capture works
    because Dropshot still reports per-player `goals`; see §5 / user-review §1.
11. [x] Map pools / mutator presets parity — `MutatorSettings.svelte` reuses the
    upstream `MatchSettings` mutator grid + `rlmodes.ts` presets; map dropdown
    shows human-readable names via `MAPS_STANDARD` keys. See §12.
12. [ ] End-to-end test: create 4-player single-elim, run a match, verify
    advancement + history + stats. **[REVIEW — needs live RL + bots]**

---

## 10. Notes on Faithful Porting

- The v4 goal-delta baseline logic is **critical**: it is what makes the mercy
  rule and result capture correct under Continue-And-Spawn. Port it exactly.
- Swiss ranking tiebreak chain (wins → multi-way H2H → goal diff → goals for →
  score) must be preserved for standings correctness.
- Double-elim LB round sizing (`2*num_rounds-2`) and WB→LB loser linking is the
  most intricate part; port verbatim and unit-test with 4/8 participants.
- `build_match_bot_list`: team0 members → blue (team index 0), team1 → orange
  (team index 1); slot = index within team.

---

## 11. Build Toolchain (cgo) Fix **[DONE]**

The v5 GUI uses cgo (`diff.go` → `bob-diff`, plus the `go-interface` flatbuffers
package). `bob-diff/libbob_diff_win.a` is a **mingw-w64 / Rust gnu-target** static
lib (its objects are `pe-x86-64` COFF `.rcgu.o`), so cgo **must** compile with
mingw gcc, not MSVC `cl.exe`.

**Root cause of the build failure (pre-existing, unrelated to tournament code):**
this machine has OS-level env vars:
- `CC` = `C:\Program Files\Microsoft Visual Studio\2022\...\cl.exe` (MSVC). Go's
  cgo splits `CC` on spaces, so it tried to exec `"C:\Program"` → not found. Even
  when quoted, `cl.exe` rejects gcc-style flags (`D8021: invalid numeric argument
  '/Werror'`) that Go passes to `runtime/cgo`.
- `CGO_CFLAGS` / `CGO_LDFLAGS` = `-O2 -g` (gcc-only flags, also break `cl.exe`).

**Fix (applied in `run_gui.bat`, `setup_gui.bat`, and `build/windows/Taskfile.yml`):**
1. Prepend a mingw-w64 `bin` dir to `PATH` (here: CLion's bundled mingw at
   `C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin`, which has `gcc.exe`).
2. `set CC=gcc` (no spaces → no arg-splitting problem).
3. `set CGO_CFLAGS=` and `set CGO_LDFLAGS=` (clear the inherited gcc flags).
4. `set CGO_ENABLED=1`.

Verified: `go build .` → `gui.exe` (43.5 MB), `wails3 generate bindings` → 45
methods / 42 models, `pnpm run build` → frontend `dist/`. All exit 0.

> Note: `build/windows/Taskfile.yml` previously hardcoded
> `CC: x86_64-w64-mingw32-gcc` (a wrapper not on this machine's PATH). Changed to
> `CC: gcc` to match the scripts. If you install a standalone mingw-w64 and put it
> on PATH, `gcc` resolves to that instead and everything still works.

---

## 12. Gap Review Round 2 (operator feedback) — all resolved **[DONE]**

After the first working port, the operator compared the v5 tournament UI against
the v4 fork and reported five concrete gaps. All five are now addressed:

1. **Missing `makora_off_v5` bot variant** — **RESOLVED.** Created
   `D:/RLBotTraining/custombot/bots/makora_off_v5/` (`main.py` sets `BOT =
   "makora_off"`, `bot.toml` reuses `makora_v5`'s venv via a relative
   `..\makora_v5\venv\Scripts\python.exe main.py`). "off" means the
   player-profiling feature is disabled in the model (different checkpoint + obs
   layout), not "offline". The checkpoint is resolved from `deploy.yaml` by the
   `makora_off` key.

2. **No tournament homepage** — v5 previously opened straight on the create form.
   **RESOLVED.** Added `frontend/src/components/Tournament/Landing.svelte`: a
   landing view listing saved tournaments + templates with create/import buttons,
   mirroring v4's `landing.html`. `Tournament.svelte` is now a three-view state
   machine (`landing` → `create` → `bracket`).

3. **Create form missing options** — **RESOLVED.** `Create.svelte` now has: bot
   pool cards (click-to-select), a human-count selector (0–10) with custom
   usernames, a scripts checkbox list, an embedded `MutatorSettings.svelte`
   (map / game-mode / full mutator grid / presets), and a launcher options modal.

4. **Forced bot names** — v5 required typing a name for every participant.
   **RESOLVED.** Bots are now chosen from the scanned bot pool by clicking a card
   (name comes from `bot.toml`); only *human* participants prompt for a name.

5. **Maps shown by code name** (e.g. `Stadium_P`) — **RESOLVED.**
   `MutatorSettings.svelte` iterates `MAPS_STANDARD` **keys** (display names like
   "DFH Stadium") and maps to the code name internally, matching v4's
   `formatMapName` behavior.

### 12.1 Game-mode string bug (root cause of "wrong mode" reports) **[DONE]**
`validGameModes`, `mapPools`, and the defaults in `TournamentNew`/
`buildMatchSettings` used `"Soccer"`/`"Hockey"`, but v5 core expects `"Soccar"` /
`"Snowday"` (per `rlmutators.ts` and `app.go`). This made Hockey/Snowday
tournaments silently fall back to Soccar. Fixed in `tournament_util.go`,
`tournament_run.go`, `tournament_maps.go`, `tournament.go`.

### 12.2 TournamentConfig missing launcher/extra-options fields **[DONE]**
`TournamentConfig` was missing `ExtraOptions`, `Launcher`, `LauncherArg`, so
tournament matches defaulted to `LauncherNoLaunch` and never started the game.
Added all three; `buildTournamentMatchConfig` now honors them.

### 12.3 Saved-tournament library + templates backend **[DONE]**
Added `tournament_library.go` with 9 bound methods (`TournamentSaveToLibrary`,
`TournamentListSaved`, `TournamentLoadFromID`, `TournamentDeleteFromList`,
`TournamentSaveTemplate`, `TournamentListTemplates`, `TournamentDeleteTemplate`,
`TournamentExportJSON`, `TournamentImportJSON`). `persistLocked()` now auto-syncs
progress back into library entries (mirrors v4's QSettings behavior). Bindings
regenerated: **54 methods / 41 enums / 44 models** (up from 45/42).

### 12.4 Build verification (round 2) **[DONE]**
- `go build ./...` → exit 0.
- `wails3 generate bindings` → 54 methods / 41 enums / 44 models.
- `pnpm run build` (frontend) → exit 0 (386 modules; only pre-existing a11y/CSS
  warnings from other components, none from tournament code).
- `wails3 build DEV=true` → exit 0 → `bin/rlbotgui.exe` (~41.8 MB).

---

## 13. Bracket duplication after save/load (pointer-identity bug) **[DONE]**

**Symptom (operator report):** in a double-elimination tournament, "Iron Knights"
*lost* their match but still advanced in the winners bracket AND appeared in the
losers bracket — the same team duplicated and spread across the bracket. The
operator's own hypothesis — "could it be a saving/loading bug that messed up the
links when I closed and re-opened the GUI?" — was exactly correct.

**Root cause:** the advance/bye logic decides *who* to advance by comparing
participants by **identity**.

- In v4, `Participant` is a Python `@dataclass`, so `completed_match.participant1
  != winner` is a **value** comparison (dataclasses auto-generate `__eq__`). After
  a save→load round-trip each match holds a fresh deep-copied participant object,
  but they still compare equal by value, so loser-detection and the "already
  placed" bye checks keep working.
- My Go port used **pointer identity** (`completed.Participant1 != winner`,
  `match.Participant1 != f.Winner`, `next.Participant1 == match.Winner`). Within a
  single session the same `*TournamentParticipant` pointer is shared across the
  matches that reference it, so pointer equality works. But after a save→load,
  `json.Unmarshal` deep-copies each participant into each match, so the WB match's
  participant and the LB/next match's participant become *different pointers with
  the same `ParticipantID`*. The code then thinks the winner "hasn't been placed
  yet" and re-advances it, and loser-detection mis-fires — so a loser gets pushed
  into the winners bracket too, duplicating across the bracket.

**Fix:** added a value-equality helper and replaced every pointer-identity
comparison in the advance/bye logic:

```go
func sameParticipant(a, b *TournamentParticipant) bool {
    if a == nil || b == nil {
        return a == b
    }
    return a.ParticipantID == b.ParticipantID
}
```

- `tournament_advance.go`: `sameParticipant()` helper; fixed loser detection in
  `populateLosersBracketMatch`, the feeder check in `findCompletedUnprocessedFeeder`,
  and the team-attach checks in `advanceWinner`/`populateLosersBracketMatch`.
- `tournament_bracket_single.go`: fixed the already-placed check in
  `processByeWinners` and the standalone `findCompletedUnprocessedFeeder`.

**Regression test (`tournament_advance_test.go`):**
- `TestHandleByeSituationAfterReload` — focused unit test that directly exercises
  the bye logic after a reload (a match containing a deep-copied participant with
  the same `ParticipantID`). **Proven to fail with pointer equality** ("wrongly
  re-advanced the feeder winner into slot 2") and pass with value equality — so it
  is a genuine guard, not a tautology.
- `TestAdvanceAfterReloadNoDuplication` — full 6-team double-elim simulation that
  plays the whole bracket after a reload and asserts no team is duplicated across
  brackets or on both sides of a match.
- `TestSameParticipantValueEquality` — locks in the helper's semantics.

All three pass with the fix; `go test .` is green.

---

## 14. Bracket card overlap + stats/history not refreshing **[DONE]**

Two more operator-reported issues, both frontend-only (no Go changes).

### 14.1 Bracket cards still overlap **[DONE]**
**Symptom:** even after the earlier dynamic min-height work, cards still collided
in a real 8-team double-elimination bracket.

**Root cause:** butterfly cards are absolutely positioned with `top:${pos}%` inside
`.bracket-round-matches`. Round-1 cards are spread at `spacing*(i+1)` where
`spacing = 100/(n+1)`, so the **gap between two stacked cards is `height/(n+1)`**,
not `height/n`. The old formula sized the column as `maxPerColumn * CARD_SLOT`,
which under-sized it by one slot — so as `n` grew the gap dropped below the ~95px
card height and cards overlapped.

**Fix:** size the column so the gap always clears the card height:
`minHeight = (maxPerColumn + 1) * CARD_SLOT + 24` (the `+1` matches the `n+1`
spacing). Applied to both `wbMinHeight` and `lbMinHeight` in
`frontend/src/components/Tournament/Bracket.svelte`.

### 14.2 Stats / Match History don't refresh when opening a tournament **[DONE]**
**Symptom:** after loading a saved tournament from the landing page (or importing
JSON), the Stats and Match History panels stayed empty.

**Root cause:** the aggregated stats/history reports are computed live by the Go
methods (`TournamentGetStats`/`TournamentGetMatchHistory`/`TournamentGetPlayerStats`),
not stored on the loaded state. `onMount(refresh)` correctly called
`refreshDerived()`, but `openFromLanding()` and `importJSON()` set `state` and
switched to the bracket view **without** re-fetching those reports.

**Fix:** `openFromLanding`, `importJSON`, and `createTournament` now all call
`await refreshDerived()` after setting `state`.
