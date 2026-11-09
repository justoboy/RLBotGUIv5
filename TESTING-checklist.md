# End-to-End Test Checklist (v4 + v5 custombot + tournament)

> Use this to verify the ported tournament feature (v5 GUI) and the v4→v5 custombot
> deployment both work. Work top-to-bottom; check each box as you confirm it.
>
> **Prereqs before any live test:**
> 1. Rocket League installed (Epic Games version — the Steam version was discontinued; this is the
>    only version new installs have). The Epic Games Launcher must be running so RL can launch.
> 2. v5 GUI built/running: double-click `run_gui.bat` (runs `wails3 dev`). First time only:
>    `setup_gui.bat` installs Go/wails3/pnpm/gcc + frontend deps.
> 3. v5 bot venv ready: run `D:\RLBotTraining\custombot\bots\makora_v5\install_v5_deps.bat`
>    once (already done in this session — verified all imports OK).
> 4. In the GUI, add the bot source folder to the bot search path:
>    **Settings → add `D:\RLBotTraining\custombot\bots`** (you said you'd do this in the GUI).
>    This makes `makora_v5/bot.toml` show up in the bot picker.

---

## A. v5 GUI smoke test (no match needed)

- [x] A1. `run_gui.bat` launches the window (no crash, nav bar visible).
- [x] A2. "Tournament" nav button is present and opens the Tournament page.
- [x] A3. Tournament **landing/homepage** renders first (not the create form): shows the saved-
      tournaments list + templates section + "Create" and "Import" buttons. (Landing header now has
      "Import Template" + "Import Tournament" buttons; each template row has an "Export" button.)
- [x] A4. Clicking "Create" opens the create form: name, format dropdown (single/double/round
      robin/swiss), team size (1v1–5v5), embedded mutator settings (map dropdown showing
      **human-readable** names like "DFH Stadium", not `Stadium_P`), game mode, randomize-map
      toggle, mercy-rule field, launcher modal.
- [x] A5. Bot picker lists `Makora` and `Makora Off` (from `custombot/bots/makora_v5` and
      `makora_off_v5`). Bots are added by **clicking a card** (no forced name entry); only humans
      prompt for a username.
- [x] A6. Human-count selector (0–10) adds human participants with custom usernames.
- [x] A7. Scripts checkbox list populates from the configured script folders.
- [x] A8. Mutator presets (from `rlmodes.ts`) + full mutator grid render in the create form.

## B. Tournament — bracket generation (no live match needed)

- [x] B1. **Single elimination**: create a 4-participant single-elim tourney → bracket shows 2
      semis + 1 final with correct seeding (1 vs 4, 2 vs 3 style).
- [x] B2. **Non-power-of-2 byes**: 3-participant single-elim → top seed gets a bye (cascading bye).
- [x] B3. **Double elimination**: 4-participant → WB + LB + Grand Final render; WB loser drops into LB.
- [x] B4. **Round robin**: 4 participants → circle-method schedule, each plays each once.
- [x] B5. **Swiss**: 4 participants → ceil(log2(4))=2 rounds; standings table shows wins/H2H/goal-diff.
- [x] B6. **Team size >1**: 2v2 single-elim → teams form (random/seeded), names generated, bracket by team.
- [x] B7. **Seeding editor**: randomize seeding + manually set/swap a seed → bracket re-seeds.
- [x] B8. **Persistence**: create a tournament, close+reopen the GUI → tournament still listed (localStorage).
- [x] B9. **Saved library**: save a tournament to the library → it appears on the landing page;
      load it by ID → state restored; delete it → removed from list.
- [x] B10. **Templates**: save current config as a template → listed under templates; create a new
      tournament from a template; delete a template.
- [x] B11. **Export/Import JSON**: export a tournament to JSON → re-import the same file → identical
      state restored.
- [x] B12. **Game-mode regression**: create a Snowday (Hockey) tournament → confirm it actually plays
      Snowday (not silently Soccar). This was the `"Hockey"`→`"Snowday"` string bug. Same check for
      Soccar (was `"Soccer"`).

## C. Tournament — live match + result capture (needs Rocket League)

- [x] C1. Start a tournament match → RL launches, bots spawn, match plays.
- [x] C2. On match end, the winner is auto-recorded and the bracket advances to the next match.
- [x] C3. **Goal-delta correctness**: scoreboard carried over between matches does NOT pollute the
      next result (baseline-delta logic). Verify by running two matches back-to-back via
      Continue-And-Spawn; second match's recorded score = only that match's goals.
- [x] C4. **Mercy rule**: set mercy=2, play until a team leads by 2 → match ends early, winner recorded.
- [x] C5. **Per-player stats**: match history shows per-player goals/assists/saves/demos for that match.
- [x] C6. **Overtime flag**: a match that goes to OT shows `is_overtime` in history.

## D. Tournament — human / LAN (REVIEW-flagged area)

- [x] D1. Add a human participant (custom username) → appears in pool and in the match's player list.
- [x] D2. "Players Ready?" gate: staging match → humans join → confirm → real match spawns bots into
      the live LAN lobby (Continue-And-Spawn). **If this is the part you wanted to skip, leave unchecked.**

## E. v5 custombot deployment (the makora checkpoint playing live)

- [x] E1. Add `Makora` to a normal (non-tournament) match → bot joins and **plays** (drives, boosts,
      jumps). If it sits idle at kickoff, the bridge/obs path is broken.
- [x] E2. **Obs parity spot-check**: open `custombot/logs/live_makora.log`; compare a few frames'
      `ball_pos`/`me_pos`/`ctrl` against what you'd expect. Controls should be non-zero and sensible.
- [ ] E3. **Dropshot**: start a Dropshot match with Makora → bot moves toward the enemy goal tile and
      charges (charge_level path). Tiles update in obs (mode-gated).
- [ ] E4. **Snowday**: start a Snowday match → bot plays (this is the mode whose mapping was reviewed;
      enum 3 → MODE_SNOWDAY). Confirm it doesn't behave like plain soccar if your checkpoints differ.
- [ ] E5. **Heatseeker / Hoops / Gridiron**: quick sanity that the bot still plays in each mode.
- [ ] E6. **Tick-skip**: bot decisions land at ~15Hz (not 120Hz) — smooth, not jittery.

## F. v4 regression (make sure the old path still works)

- [ ] F1. v4 GUI (`D:\RLBotGUI`) still launches via `runRLBotInsideEnv.bat`.
- [ ] F2. v4 custombot (`CheckpointBot` / `StandaloneBot`) still plays a v4 match unchanged.
- [ ] F3. v4 tournament still works end-to-end (unchanged, but confirm nothing regressed).

## H. Auto-start + Open RL + stats (Round 13 — needs Rocket League)

- [x] H1. **Auto-start toggle**: enable auto-start → next ready match auto-launches after the interval
      countdown; bracket advances; schedules the following match (1s delay). Toggle off stops it.
- [x] H2. **Auto-start interval**: Immediate (0) starts instantly; 10/30/60/120s show a live countdown.
- [x] H3. **Human behavior = Pause**: with a human match next, auto-start pauses (waits for manual Start).
- [x] H4. **Human behavior = Skip**: auto-start jumps past human matches to the next bot-only match.
- [x] H5. **Human behavior = Start Match (continue)**: auto-start plays human matches like any other.
- [x] H6. **Manual Start ignores behavior**: with behavior=pause, clicking Start Match still starts the
      selected/next match (manual override).
- [x] H7. **Open Rocket League**: button launches RL into a freeplay lobby (no bots spawned) so a human can
      host/join via Rocket Plugin. Confirm this is the staging behavior you want.
- [x] H8. **Human Match Info modal**: shows staging steps + current game mode/map (display name)/mercy rule/
      all mutators + next match player count.
- [x] H9. **Stats tab**: ranked table shows W-L-D, Win%, GF/GA/±, score, G/A/Sv/Sh/Dem/OG per entity; ordered
      by wins → H2H → goal-diff → goals-for → score. **Round 14 fix:** opening a saved tournament from the
      landing page (or importing JSON) now re-fetches the live stats/history reports, so the panels populate.
- [x] H10. **Match History tab**: per-round expandable list; expanding a match shows per-player stat lines.

## J. Round 20 — bracket duplication after save/load (pointer-identity bug)

> **Bug you reported:** "Iron Knights lost, but moved on in the winners and got added to losers so
> they duplicated and spread all over the bracket." This only happened after you closed and re-opened
> the GUI (i.e. after a save→load round-trip), which is the key clue.
>
> **Root cause:** the advance/bye logic compares participants by **identity**. In v4, `Participant` is
> a Python `@dataclass`, so `completed_match.participant1 != winner` is a **value** comparison (the
> dataclass auto-generates `__eq__`). After a save→load, each match gets a fresh deep-copied
> participant object, but they still compare equal by value, so loser-detection and the "already
> placed" bye checks keep working. My Go port used **pointer identity** (`completed.Participant1 !=
> winner`). Within one session the pointers are shared so it works — but after a reload,
> `json.Unmarshal` deep-copies each participant into each match, so the WB match's participant and the
> LB/next match's participant become *different pointers with the same `ParticipantID`*. The code then
> thinks the winner "hasn't been placed yet" and re-advances it, and loser-detection mis-fires — so a
> loser gets pushed into the winners bracket too, duplicating across the bracket.
>
> **Fix:** added `sameParticipant(a, b)` (compares `ParticipantID`, i.e. value) and replaced every
> pointer-identity comparison in the advance/bye logic (`tournament_advance.go`,
> `tournament_bracket_single.go`).

- [x] J1. **Regression test (automated):** `go test -run TestHandleByeSituationAfterReload` passes with
      the fix and **fails** when `sameParticipant` is switched back to pointer equality — proving the
      test is a genuine guard, not a tautology. Full suite green (`go test .`).
- [ ] J2. **Live reproduction:** create a double-elim tournament, play a match, **close the GUI and
      reopen it** (forces a save→load), then continue advancing. A loser must NOT appear in the winners
      bracket, and no team should appear in two matches at once. (This is the exact scenario that broke
      before the fix.)
- [ ] J3. **Bye after reload:** with a non-power-of-2 bracket, complete a feeder match, reload the GUI,
      then advance — the already-placed winner must NOT be re-advanced into the next match's second slot.

## G. Cross-cutting

- [x] G1. v4 and v5 bots do NOT cross-contaminate: a v5 match runs the v5 `rlbot` (python-interface),
      a v4 match runs v4 `rlbot`. (They can't share a venv — that's why makora_v5 has its own.)
- [x] G2. `deploy.yaml` maps `makora` → the correct checkpoint file; bot loads it (no KeyError).

## I. Round 15 — bracket overlap + per-player stats + mercy replay + custombot ball (needs Rocket League)

- [ ] I1. **Bracket no-overlap**: create a large tournament (e.g. 16 participants) → match cards in the
      busiest column never overlap; the column grows tall enough that spacing always exceeds card height.
      **Round 14 fix:** the column min-height now uses `(maxPerColumn + 1) * CARD_SLOT` (the `+1` matches
      the `100/(n+1)` spacing), so the gap between stacked cards always clears the card height.
- [x] I2. **Per-player stats (team mode)**: in a 2v2+ tournament, open the Stats panel → a **Player Stats**
      tab appears (only when team_size > 1) listing each individual player's aggregated Goals/Assists/
      Saves/Shots/Demolitions/Own Goals/Score across all completed matches, with their team name.
- [x] I3. **Mercy rule waits for replay**: set mercy=2, play until a team leads by 2 → the goal replay
      plays out fully; the match only stops after the post-replay kickoff countdown (not instantly).
- [x] I4. **Custombot no-crash on replay**: during a goal replay/celebration (ball removed from packet),
      the makora bot does NOT throw `IndexError: list index out of range` on `packet.balls[0]`; it idles.

---

### What I already verified for you (no action needed)
- v5 bot venv installs cleanly (torch 2.14.0+cu126, rlgym@ece64c8, RocketSim, EARL, v5 rlbot 2.0.0b56).
- `custombot.rlbot_agent_v5` + `custombot.rlbot_bridge_v5` import cleanly in the v5 venv.
- v5 `GameMode` enum ordering confirmed (Soccar=0…Knockout=7); Snowday=3 → MODE_SNOWDAY matches
  training's `detect_mode` ("snow"→MODE_SNOWDAY).
- v5 core populates Dropshot tiles + ball charge_level (confirmed in RLBotCS source).
