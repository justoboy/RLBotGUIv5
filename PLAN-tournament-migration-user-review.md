# Dropshot
- You state that the v5 supports dropshot and no workaround is needed, but that directly contradicts the v5 wiki which states:
 > "Currently, RLBot v5 does not have access to the position or state information of individual dropshot tiles nor the ball energy level 
 > described on this page. These features were lost with the Psyoninx API. Long term, we hope to return this data to RLBot."
- Dropshot goes off of goals not score
- Goals are scored by getting the ball through the open ground after destroying tiles to make an opening. Tiles are either non-damaged, 
  damaged, or destroyed. So bots that rely on those tile locations/damage states to play dropshot will crash (like our 
  "D:\RLBotTraining\custombot")

# Human / LAN Staging Workflow
- I am not sure how we would make a richer staging UI. The process of allowing a second player to join requires rocket league to be open 
  with the rlbot flag, bakkes mod to be injected into it with the rockethost plugin installed, and the user to use that to host a server 
  for the gui to use with continue and spawn. I don't see any way to handle it better on our end than a button to easily open the game 
  for them incase it isn't running, then they can do their setup with the plugin and click start in our gui when ready. Do you see a way 
  of making that "richer"?
- I have never seen the v5 code before, so I am unsure of the "Continue and Spawn" value or of the player config mappings.

# Out of Scope
- These are correct, the original v5 gui is still in development, and they will probably add the story mode and custom map support 
  eventually. We can just merge those changes into our fork later on if they do rather than try to implement ourselves.

# Use of fork
- Just want to make sure we are saving the changes to our custom fork "https://github.com/justoboy/RLBotGUIv5" not the original 
  "https://github.com/RLBot/gui" so we don't accidentally try pushing to the original creator's repo.

---

# Assistant resolution (implementation complete, awaiting your review)

## Status: implemented and compiles
The full tournament feature is ported to Go + Svelte and the whole project builds:
- `go build ./...` → exit 0 (produces `gui.exe`)
- `wails3 generate bindings` → exit 0 (45 methods, 42 models bound)
- `pnpm run build` (frontend) → exit 0 (380 modules, no errors from tournament code)

Files added (Go backend): `tournament_state.go`, `tournament_bracket_single.go`,
`tournament_bracket_double.go`, `tournament_bracket_roundrobin.go`,
`tournament_bracket_swiss.go`, `tournament_team.go`, `tournament_team2.go`,
`tournament_match.go`, `tournament_run.go`, `tournament_advance.go`,
`tournament_maps.go`, `tournament_util.go`, `tournament.go`.
Frontend: `frontend/src/pages/Tournament.svelte`, wired into `App.svelte` nav.

## 1. Dropshot — RESOLVED: v5 core fully supports it; no v4 fallback needed
I previously claimed v5 had no Dropshot tile/energy data and called it a hard core
limitation. That was WRONG, and I've now confirmed it by cloning the v5 core
(https://github.com/RLBot/core, the C# "RLBotCS" server) and reading its source.

The wiki text you pasted is OUT OF DATE. The v5 core populates everything:
- `RLBotCS/Conversion/GameStateToFlat.cs`:
  - line 91: `ChargeLevel = ball.chargeLevel` → ball energy IS sent
    (-1 not dropshot, 0 no charge, 1 charged, 2 supercharged).
  - lines 139-143: builds `tileStates` from `gameState.Tiles` ordered by Y then X,
    casting `tile.DamageLevel` → `TileDamageLevel` (Start/Damaged/Broken).
  - line 238: `Tiles = tileStates` set on the GamePacket.
- `RLBotCS/Server/ServerMessage/DistributeFieldInfo.cs`: populates static
  `FieldInfo.Tiles` with each tile's `Location` (Vector3) + `Team`.
- `RLBotCS/ManagerTools/BallPredictor.cs`: even has a Dropshot prediction mode
  (`load_dropshot`), so ball prediction works in Dropshot too.

Conclusion: there is nothing for the GUI to "fall back" to. The v5 protocol AND the v5
core both carry tile state and ball energy. The GUI side is already correct
(`gameMode: "Dropshot"` → `flat.GameModeDropshot`, scoring off `Goals`).

The only remaining work is on the BOT side, not the GUI: your `custombot`
(`D:\RLBotTraining\custombot`) must read the v5 field names instead of v4's —
`packet.tiles` (the damage grid) and the ball's `charge_level` — instead of whatever
v4 API it used. That's a bot-porting task, separate from this GUI migration.

## 2. Human / LAN staging workflow
- Implemented exactly as you described: when a match contains a human,
  `TournamentStartMatch(matchID, useStaging=true)` does NOT launch; it returns the
  state so the operator can open the game (RLBot flag + BakkesMod + RocketHost
  plugin), host a server, then call `TournamentConfirmPlayersReady(matchID)` which
  launches with `ExistingMatchBehaviorContinueAndSpawn` already set. No richer UI is
  attempted, matching your assessment.

## 3. Continue And Spawn value + player config mappings (your open questions)
- `ExistingMatchBehaviorContinueAndSpawn = 1` (confirmed in
  `go-interface/flat/generated_ExistingMatchBehavior.go`).
- Player config mapping parity: `buildTournamentMatchConfig` calls
  `playerInfo.ToPlayer().ToPlayerConfig(teamIndex)` for each side, reusing the exact
  same `ToPlayerConfig` path the normal (non-tournament) match launch uses, so human
  and bot mappings are identical to v4's `create_player_config` behavior.

## 4. Out of scope (story mode, custom maps)
- Agreed. The v5 `StoryMode.svelte` is left as the upstream stub; we will merge
  upstream's eventual story-mode/custom-map work into our fork rather than port it.

## 4b. Launcher auto-detect — RESOLVED as "verify, don't implement"
- You said you doubt v5 didn't implement auto-detect, and that it's fine to just select
  the launcher manually; just record it on the testing checklist. Done — see checklist
  item G3.
- What I found in the v5 GUI: the launcher is a **manual** dropdown (`steam`/`epic`/
  `custom`/`nolaunch`) wired to `flat.Launcher` in `app.go`, `rockethost.go`, `loadout.go`,
  and `tournament_match.go`. Default is `LauncherNoLaunch` (no auto-launch). This differs
  from v4, where `DEFAULT_LAUNCHER_PREFERENCE = RocketLeagueLauncherPreference(EPIC, True)`
  auto-tries Epic-with-login-tricks then falls back to Steam (`setup_manager.py:88`,
  `epic_launch.py`).
- Decision: NOT implementing auto-detect per your call. If live testing (G3) shows v5
  requires manual selection and that's annoying, we can port v4's Epic-manifest/Steam
  registry auto-detect later — it's self-contained (registry reads + `*.item` manifest
  parsing) and would live in the Go backend, no upstream repo changes needed.

## 5. Fork safety
- Verified: `git remote -v` shows `origin → https://github.com/justoboy/RLBotGUIv5`
  (fetch AND push), branch `master`. All changes are on your fork. Nothing is wired
  to push to `RLBot/gui`.

## Build toolchain notes (for reproducing the build on this machine) — RESOLVED
Two separate root causes were blocking `wails3 build`/`wails3 dev`. Both are now fixed in
`run_gui.bat` and `setup_gui.bat`, and a full `wails3 build DEV=true` now succeeds end-to-end
(produces `bin/rlbotgui.exe`, ~41 MB).

### Root cause 1 — cgo used MSVC cl.exe (spaced path broke cgo)
- The OS-level env var `CC` is the MSVC `cl.exe` path **with a space**
  (`C:\Program Files\Microsoft Visual Studio\...\cl.exe`). Go's cgo splits `CC` on spaces, so it
  tried to exec `C:\Program` → `cgo.exe: exit status 2`. The OS env var also overrides `go env -w`.
- Fix: `run_gui.bat`/`setup_gui.bat` set `CC=gcc` (space-free) and prepend the CLion mingw gcc
  (`C:\PROGRA~1\JETBRA~1\CLION2~1.2\bin\mingw\bin`) to PATH, and clear the inherited
  `CGO_CFLAGS`/`CGO_LDFLAGS` (`-O2 -g`). Verified: a bogus `CC` path makes cgo complain about that
  exact path, proving the session override propagates.

### Root cause 2 — Wails build pipeline uses Unix tools not on PATH
- `build/Taskfile.yml` runs Unix-style commands: a `sh:` precondition (`sh -c "pnpm --version"`),
  `mkdir -p darwin` (generate:icons), and `rm -f *.syso`. Under cmd.exe, `sh`/`mkdir`/`rm` are not
  on PATH, and `pnpm` lives in `C:\Users\justo\AppData\Local\npm-global` which is also not on PATH.
  That's why the build failed with `"mkdir": executable file not found` and `Looks like pnpm isn't
  installed.`
- Fix: both scripts now prepend `C:\Program Files\Git\bin` + `C:\Program Files\Git\usr\bin`
  (for `sh.exe`/`mkdir.exe`/`rm.exe`) and `C:\Users\justo\AppData\Local\npm-global` (for `pnpm.cmd`)
  to PATH. After this, `wails3 build DEV=true` completed: go mod tidy → pnpm install → generate
  icons → generate bindings (45 methods/42 models) → vite build (380 modules) → syso → go build.

### Note on the `mkdir -p darwin` step
- This is upstream's cross-platform bug (Unix `mkdir` in a Windows build path). We did NOT patch
  `build/Taskfile.yml`; instead we just put a real `mkdir.exe` (Git's `usr\bin`) on PATH, which is
  the least-invasive fix and keeps our fork diff minimal for future upstream merges.

---

# Round 2 — the 5 gaps you reported (all implemented, builds green)

You listed 5 concrete differences between my first v5 port and the v4 fork. All five are now
addressed. Bindings regenerated to **56 methods / 41 enums / 44 models**; `wails3 build DEV=true`
reproduces `bin/rlbotgui.exe` (~41.8 MB).

## Gap 1 — missing `makora_off_v5` — DONE
Created `D:\RLBotTraining\custombot\bots\makora_off_v5\` (`main.py` sets `BOT = "makora_off"`,
`bot.toml` reuses makora_v5's venv via a relative `run_command`). "off" = player-profiling feature
disabled in the model (different checkpoint + obs layout), resolved from `deploy.yaml` by the BOT
key. Same deps as makora_v5, so it shares the venv.

## Gap 2 — no tournament homepage (v5 started at the create form) — DONE
`frontend/src/pages/Tournament.svelte` is now a 3-view state machine: `landing` → `create` →
`bracket`. The **landing** view (`components/Tournament/Landing.svelte`) mirrors v4's `landing.html`:
saved-tournaments list + templates section, plus header buttons **Create New Tournament**,
**Import Tournament**, **Import Template**. Each template row has **Use / Export / Delete**.

## Gap 3 — create form missing scripts / full mutator presets / human count — DONE
`components/Tournament/Create.svelte` embeds `MutatorSettings.svelte` (map dropdown with
**human-readable** names via `MAPS_STANDARD` keys, game mode, full mutator grid + presets from
`rlmodes.ts`, randomize-map toggle) and a scripts checkbox list + human-count selector (0–10 with
custom usernames).

## Gap 4 — forced bot names — DONE
Bots are added by **clicking a card** in the pool (no name entry); only humans prompt for a
username. Matches v4's pool-select behavior.

## Gap 5 — map code names shown instead of human-readable — DONE
`MutatorSettings.svelte` renders display names (e.g. "DFH Stadium") from `arena-names.ts`, not
`Stadium_P`.

## Bug fix — game-mode string mismatch (silent Soccar fallback) — DONE
`validGameModes`, `mapPools`, and the defaults in `TournamentNew`/`buildMatchSettings` used
`"Soccer"`/`"Hockey"`, but v5 core expects `"Soccar"`/`"Snowday"` (per `rlmutators.ts`/`app.go`).
Hockey/Snowday tournaments silently fell back to Soccar. Fixed in all three Go files. (Checklist
B12.)

## Backend — saved-tournament library + templates + import/export — DONE
New `tournament_library.go` adds 9 bound methods: `TournamentSaveToLibrary`, `TournamentListSaved`,
`TournamentLoadFromID`, `TournamentDeleteFromList`, `TournamentSaveTemplate`, `TournamentListTemplates`,
`TournamentDeleteTemplate`, `TournamentExportJSON`, `TournamentImportJSON`, plus (this round)
`TournamentImportTemplateJSON` and `TournamentExportTemplateJSON`. `persistLocked()` auto-syncs
progress back into the library entry (mirrors v4's QSettings auto-sync).

## UI fix — mutator modal stacking — DONE
The mutator popup was trapped behind the Scripts panel (CSS stacking context from a parent with a
transform/opacity). Fixed by rendering `MutatorSettings.svelte`'s popup through the shared
`Modal.svelte`, which uses `use:portal={"body"}` (`svelte-portal`) to render at `<body>`, escaping
all parent stacking contexts.

## v4 ↔ v5 template cross-compatibility — INVESTIGATED (answer to your question)
Core tournament state is **fully cross-compatible**: v4's `tournament_state.py` `to_dict()` and v5's
`tournament_state.go` JSON tags use identical field names (`name`, `tournament_id`, `format`,
`participants`, `matches`, `team_size`, `teams`, `winner_team`, `allow_duplicates`, `swiss_rounds`,
`map`, `game_mode`, `randomize_map`, `scripts`, `mercy_rule`). v5 adds extra fields (`launcher`,
`launcher_arg`, `mutators`, `extra_options`) that serialize to `null`/empty in v4 exports and are
safely ignored on import. So: a v4-exported JSON imports cleanly into v5 (missing extras use
defaults); a v5 template exports cleanly and v4 ignores the extras it doesn't understand.

---

# Round 3 — bracket rendering + two bugs (all implemented, builds green)

You tested the Round 2 build and reported 4 issues. All four are fixed. No Go API changed, so
bindings stayed at 56 methods; `wails3 build DEV=true` reproduces `bin/rlbotgui.exe` (~41.9 MB).

## Bug — round-robin tournament crashed the GUI — DONE
`tournament_bracket_roundrobin.go` used Go's raw `%` in the circle-method rotation. Go's `%` returns
a negative result for negative dividends (e.g. `(1-3) % 7 == -2`), so the rotation produced a negative
array index → `index out of range [-1]` panic. Added a `mod(a, b)` helper in `tournament_util.go` that
wraps to a non-negative result and used it in the rotation formula.

## Bug — Delete from the bracket view didn't remove the tournament — DONE
`TournamentDelete()` only removed `current.json`, so the tournament reappeared on the landing page
(which lists `saved/<id>.json`). It now also `os.Remove`s the matching `saved/<id>.json`, so the
tournament disappears from the library too.

## Feature — bracket now shows match-to-match connectors (butterfly layout) — DONE
The old v5 bracket just stacked matches by round with no visual link between them. Ported v4's
butterfly layout into a new `components/Tournament/Bracket.svelte`:
- Winners bracket flows outer→inner into a centered Finals column; each wing's round-1 matches spread
  evenly across the column height and later rounds sit at the midpoint of their feeders (via
  `next_match_id`), so the tree visually converges like v4.
- SVG elbow connectors are drawn per section from each match's DOM rect to its `next_match_id` target,
  using the scroll container's top-left as origin so connectors scroll with the content.

## Feature — double elimination now renders a real Losers Bracket — DONE
`Bracket.svelte` renders `losers_bracket_matches` (plus the L-prefixed Grand Final) as a **separate
section below** the winners bracket, with its own amber connectors. LB positioning handles the case
where an LB round gains matches from WB losers: matches with LB feeders sit at the midpoint of their
feeders; "entry point" matches (no LB feeder) are placed at the midpoint of their WB feeders (via
`loser_next_match_id`) or, failing that, in the largest vertical gap.

## Note — Svelte 5 rune/prop name collision (gotcha)
The component prop was originally named `state`, which collides with the `$state` rune — Svelte 5
treats `$state` as a store subscription when a local binding named `state` exists, silently breaking
reactivity. Renamed the prop to `tournament` (call site passes `tournament={state}`).

## Still needs your input (unchanged from before)
- Live end-to-end match play against a real Rocket League launch (LAN staging gate, auto-record).
- Confirm whether v5 auto-detects the launcher, or whether the launcher selector must always be set.

---

# Round 6 — bracket polish + seeding editor (all implemented, builds green)

You tested the bracket build and reported six issues. All six are fixed. Bindings regenerated to
**59 methods / 41 enums / 44 models** (added `TournamentSaveCurrentAsTemplate`, `TournamentRenameTeam`,
`TournamentFormTeamsWithPairings`); `go build ./...` → exit 0; `pnpm run build` → clean;
`wails3 build DEV=true` reproduces `bin/rlbotgui.exe` (~41.9 MB).

## Fix — losers-bracket round naming (Quarter→Semi→Finals, not Semi→Finals→Round 3) — DONE
Root cause was a **display/naming** bug, not a bracket-generation bug. `getRoundName`'s losers branch
computed the total round count from `tournament.losers_bracket_matches` alone, which excludes the
Grand Final (the GF lives in `matches` under an `L` prefix). That made the LB look like it had fewer
rounds than it rendered, so the names shifted. Changed the losers branch to compute the total from
`lbMatches` (LB matches + the L-prefixed GF), matching v4's getter exactly. For a 4-bot DE bracket it
now correctly shows Quarter→Semi→Finals.

## Fix — contrast bubbles on section/round titles — DONE
`.section-title` and `.round-title` now render as dark pills (background + border glow +
`backdrop-filter: blur`) so labels stay legible over the SVG connector lines behind them.

## Fix — WB/LB width consistency — DONE
Removed the `max-width` caps (220px/260px) from `.butterfly-wing-left/right` and `.butterfly-center`
so the winners bracket flexes to full width like the losers bracket instead of collapsing narrower.

## Fix — removed force win/loss buttons — DONE
Stripped the manual force-win / force-loss button elements from the bracket match card; only the
**Play** button remains in `.matchActions`.

## Feature — "Save as Template" from the bracket header — DONE
New bound method `TournamentSaveCurrentAsTemplate(name)` in `tournament_library.go` reconstructs a
`TournamentConfig` from the current live state and persists it via the shared `saveTemplateLocked()`
helper. `Tournament.svelte` gained a **Save as Template** button (between Save and Export) that prompts
for a name and shows a toast on success.

## Feature — seeding / team editor ported — DONE
New `components/Tournament/SeedingEditor.svelte`, wired into the bracket header:
- **Click-to-swap** seeding reorder (click two participants → swap positions) → `TournamentSwapSeeds`.
- **Randomize** shuffle → `TournamentRandomizeSeeding`.
- **Manual team pairing** (team mode): click two participants to pair them, then **Form Teams** →
  `TournamentFormTeamsWithPairings(pairings, teamNames)` which runs the graph-connected-components
  port of v4's `form_teams_with_manual_pairings`/`pair_participants` and regenerates the bracket.
- **Editable team names** commit on blur/enter → `TournamentRenameTeam(teamIndex, newName)` (renames the
  team and updates stand-in participant names across all matches, enforcing name uniqueness).

## Gotcha reminder (Svelte 5)
The bracket/seeding components take a prop named `tournament` (NOT `state`) — a local binding named
`state` collides with the `$state` rune and silently breaks reactivity.

---

# Round 7/8 — match-start diagnosis + team roster panel (all implemented, builds green)

You reported two missing UI features and a match-start error. All three are addressed. Bindings
regenerated to **63 methods / 41 enums / 44 models** (added `TournamentRandomizeTeamNames`,
`TournamentReformTeams`, `TournamentReorderTeamMember`, `TournamentTeamBalance`); `go build ./...` →
exit 0; `pnpm run build` → clean; `wails3 build DEV=true` reproduces `bin/rlbotgui.exe` (~42 MB).

## Diagnosis — "Failed to reconnect to RLBotServer at 127.0.0.1:23234" — NOT a code bug, RESOLVED
This was never a bug in our code. The v5 GUI is a **pure TCP client**: `go-interface`'s `Connect()`
just does `net.Dial("tcp", addr)`. It never starts the core server — it only dials `127.0.0.1:23234`.
`netstat` confirmed nothing was listening on 23234, so every match start failed to connect.

In the official setup, the **Rust launcher** (`RLBot/launcher`, v1.0.5) is what downloads
`RLBotServer.exe` from `RLBot/core` into `%LOCALAPPDATA%\RLBot5\bin\`, spawns it as a background
process (it then binds `0.0.0.0:23234` and waits for the GUI), and kills it when the GUI exits. We
don't use that launcher, so nothing was running on 23234.

**Fix (no upstream repo change needed):** `run_gui.bat` now replicates the launcher's job — it
downloads `RLBotServer.exe` (from `RLBot/core` latest release) into `%LOCALAPPDATA%\RLBot5\bin\` if
missing, starts it in the background **only if nothing is already listening on 23234**, and on GUI
exit kills the server **only if we were the ones who started it** (so it never kills the official
launcher's server). `setup_gui.bat` pre-downloads the binary too. Verified by hand: running the
downloaded `RLBotServer.exe` logs "Server will use port 23234, expecting Rocket League on port 23233"
and `netstat` shows it LISTENING on `0.0.0.0:23234`.

> Note: this makes the GUI connect successfully. Actually launching a match still requires Rocket
> League + BakkesMod + the RocketHost plugin to be installed and RL pointed at the server — that's
> the live-testing gate (checklist G1/G3), not a code issue.

## Feature — team roster panel above the bracket (team_size > 1) — DONE
New `components/Tournament/TeamRoster.svelte`, rendered above `<Bracket>` in `Tournament.svelte`
whenever `team_size > 1`. Mirrors v4's `active.html` team panel:
- One card per team: editable name (commit on blur/enter → `TournamentRenameTeam`) + W-L record.
- Each member shows its in-game slot (`S1`, `S2`, …) with ↑/↓ arrows to reassign seats →
  `TournamentReorderTeamMember(teamIndex, fromSlot, toSlot)` (slot order = in-game seat).
- A balance indicator (bots=1.0, humans=0.5 spread) → `TournamentTeamBalance`.

## Feature — randomize team NAMES without re-seeding — DONE
The roster header has two distinct actions so the distinction is explicit:
- **Randomize Names** → `TournamentRandomizeTeamNames()` — re-draws every team's name from the
  15×15 descriptor+noun pool (unique within the tournament) and syncs the stand-in participant names
  in every match. **Membership and seeding are untouched.**
- **Re-form Teams** → `TournamentReformTeams()` — re-shuffles participants into fresh random teams
  and regenerates the bracket (this DOES change membership).

Both were already backed by the ported `team_manager.py` logic (`assign_random_team_names` etc.);
this round just exposed them as bound methods and wired the UI.

---

# Round 9/10 — match-start STILL failing (real root cause) + balance indicator removed

You re-tested and still hit `Failed to reconnect to RLBotServer at 127.0.0.1:23234`, and said the
balance indicator "always seems the same" and wasn't useful. Both addressed. Bindings now **62 methods
/ 41 enums / 44 models** (removed `TournamentTeamBalance`); `go build ./...` → exit 0;
`pnpm run build` → clean; `wails3 build DEV=true` reproduces `bin/rlbotgui.exe`.

## Root cause of the match-start error — a findstr regex bug in run_gui.bat (NOT the GUI code)
The Round 7/8 fix made `run_gui.bat` start `RLBotServer.exe` only when "nothing is already listening on
23234". But the port check was:

```
netstat -ano | findstr /r ":23234[  ].*LISTENING"
```

`findstr /r` splits its search string on **spaces into multiple OR-patterns**. So that pattern was
really three patterns: `:23234[` OR `]` OR `.*LISTENING`. The third (`.*LISTENING`) matches **every**
listening socket on the machine, so the check always matched → the script always printed "Port 23234
already in use … Not starting a new one" → our server never started → the GUI had nothing to dial.
Confirmed by running the exact command: it returned `:80`, `:135`, `:445`, … (none contain 23234).

**Fix:** chain two `findstr` with a literal `/c:` pattern (no spaces inside it):

```
netstat -ano | findstr LISTENING | findstr /c:":23234"
```

Verified: with no server running, this now correctly returns nothing (exit 1), so the script starts our
server. This is the actual reason match start kept failing — the GUI code was correct all along.

## Balance indicator — REMOVED (it was never useful)
You said you never understood it and it always seemed the same. You're right, and it's a faithful port
of v4's `team_strength`/`team_balance_report`: it scores each team by **composition only** (each bot
= 1.0, each human = 0.5), then reports `spread = max(team strength) - min(team strength)`. Because
every team in a normal tournament has the same bot/human mix, every team scores identically, spread is
always 0, and it always reads "balanced". It measures nothing about actual skill.

Per your call ("not sure why it was ever added"), I removed it entirely: deleted the `$state`/`$effect`
that fetched it and the markup/CSS in `TeamRoster.svelte`, and deleted the dead Go code
(`TournamentTeamBalance`, `teamBalanceReport`, `teamStrength`). If you later want a *real* skill-based
balance (e.g. from per-bot skill ratings), that's a new feature and would need a skill source defined.

---

# Round 13 — auto-start + Open Rocket League + Human Info modal + stats/rankings (all implemented, builds green)

You raised two things: (1) the todo list wasn't rendering (markdown formatting on my end — fixed, it's a
single markdown checklist string now), and (2) v5 was missing v4's **auto-start** controls, the **Open
Rocket League** button, and the **Human Match Info** modal. You also asked me to double-check that all
match/player **stats and rankings** are recorded and displayed like v4 (and grab more stats if v5's API
exposes any). All addressed. Bindings regenerated (the four new methods are present in
`frontend/bindings/gui/app.js`); `go build ./...` → exit 0; `pnpm run build` → clean (398 modules).

## Feature — auto-start scheduler (ported from v4 `tournament-vue.js`) — DONE
v4's auto-start is **entirely frontend-driven** (no backend changes). Ported into `Tournament.svelte`:
- State: `autoStartEnabled`, `autoStartInterval` (0/10/30/60/120s), `autoStartHumanBehavior`
  (`continue`/`pause`/`skip`), `autoStartCountdown`, `autoStartTimer`.
- `findNextAutoStartMatch(ignoreHumanBehavior)`: for double-elim it scans **LB rounds first, then WB**
  (LB matches only become "ready" once their WB feeders finish, so this interleaves correctly — matches
  v4 exactly; v4's `_getInterleavedRoundOrder` is display-only and not needed for scheduling). Respects
  human behavior: `pause` → return null (wait for manual), `skip` → continue past human matches,
  `continue` → proceed. Manual starts pass `ignoreHumanBehavior=true` so `pause`/`skip` never block them.
- `startAutoStartTimer()`: countdown via `setInterval`; on completion calls `launchMatch`. After a match
  completes, `launchMatch` re-schedules the next match with a 1s delay (mirrors v4).
- UI in `components/Tournament/AutoStartToolbar.svelte`: Start Match / Open Rocket League / Human Match
  Info buttons + auto-start toggle + interval dropdown + human-behavior dropdown + countdown display.

## Feature — Open Rocket League + Human Match Info modal — DONE
- `TournamentOpenRocketLeague()` (Go, `tournament_stats.go`): v5's flat protocol has **no "launch game
  only" message** (only `MatchConfigurationT`/`StartCommandT`), so it launches an **empty freeplay match**
  with the tournament's launcher (`settings.ExtraOptions.Freeplay = true`), bringing RL up to a lobby so
  humans can host/join via the Rocket Plugin. This is the closest faithful equivalent to v4's
  `open_rocket_league()` → `connect_to_game()`.
- `components/Tournament/HumanInfoModal.svelte`: explains the staging flow (Open RL → Home→Host in Rocket
  Plugin → ensure Continue-and-Spawn → humans Join → Start Match), shows current game mode / map (display
  name via reverse `arena-names` lookup) / mercy rule / all mutators (enum index → label via
  `rlmutators.ts`), and the next match's player count.

## Stats / rankings — VERIFIED at parity, display layer added
Confirmed `flat.ScoreInfoT` exposes **exactly** the same fields as v4 (Score, Goals, OwnGoals, Assists,
Saves, Shots, Demolitions) — **no extra stats** are available from the v5 API, so parity is automatic.
The per-player capture (baseline-delta from first live packet) already lives in `tournament_match.go`.
Added the **display** layer that was missing:
- `TournamentGetStats()` → ranked per-entity table (W-L-D, Win%, GF/GA/±, score, G/A/Sv/Sh/Dem/OG),
  ordered by wins → multi-way head-to-head → goal diff → goals for → score (same tiebreak as the bracket).
- `TournamentGetMatchHistory()` → per-round expandable per-player stat lines (grouped WB/LB/main).
- `TournamentGetSwissStandings()` → exposes the previously-internal Swiss ranking helpers as a bound method.
- `components/Tournament/StatsPanel.svelte`: two tabs (Stats / Match History) wired into the bracket view.

## Still needs your input (live testing — you're at work)
- Live end-to-end: auto-start actually advancing the bracket, Open RL freeplay lobby, human staging gate.
- Confirm the freeplay-launch approximation for "Open Rocket League" is acceptable vs. v4's behavior.

---

# Round 14 — stats contrast + full names + stats persistence + mercy-rule end-match (all implemented, builds green)

You tested the Round 13 build and reported four issues. All four fixed. No new bound methods (the mercy
fix reuses the existing `StopMatch`; the other three are frontend-only), so bindings unchanged.
`go build ./...` → exit 0; `pnpm run build` → clean (398 modules).

## Fix — stats/match-history hard to read over the background image — DONE
The stats panel had no background of its own, so the grey table text sat directly on the page background
image. Wrapped the whole `StatsPanel.svelte` in a `.panel` container with a solid dark background
(`rgba(10,14,20,0.82)` + border + slight blur), matching the contrast treatment used elsewhere.

## Fix — abbreviated stat columns unclear — DONE
Replaced the abbreviated headers (W-L-D, GF, GA, +/−, G, A, Sv, Sh, Dem, OG) with v4's full column names:
Played, Wins, Draws, Losses, Win Rate, Goals For, Goals Against, Goal Difference, Score, Assists, Saves,
Shots, Demolitions, Own Goals (and the per-player history table: Player, Team, Score, Goals, Assists,
Saves, Shots, Demolitions, Own Goals).

## Fix — stats/match-history empty after reopening the GUI — DONE (root cause: frontend)
The per-player stats ARE persisted (inside each match's `player_stats`), but the aggregated Stats/History
reports are computed live by `TournamentGetStats`/`TournamentGetMatchHistory`. On mount, `refresh()` only
set `state` and never re-fetched those reports, so the panels stayed empty after a reload. Fixed:
`refresh()` now calls `refreshDerived()` after loading state, so stats/history repopulate on reopen.

## Fix — mercy rule recorded the result but the RL match kept running — DONE
Root cause: when the mercy threshold was reached, `captureMatchResult` returned the snapshot, but nothing
sent a stop command, so the in-game match kept playing. v4 used `set_game_state(end_match=True)`; v5's
protocol has no "end match" message, so the runner must send `StopCommandT`. Fixed: `TournamentStartMatch`
now calls `a.StopMatch(false)` when `result.MercyTriggered` is true, which ends the live match. (A normally
-ended match is already over, so we only stop on the mercy path.)

---

# Round 15 — bracket overlap + per-player stats + mercy replay + custombot ball crash (all implemented, builds green)

You tested the Round 14 build and reported four more issues. All four fixed. Bindings regenerated to
**63 methods / 41 enums / 44 models** (added `TournamentGetPlayerStats`); `go build ./...` → exit 0;
`pnpm run build` → clean (398 modules).

## Fix — bracket cards overlap as more bots are added — DONE
The butterfly cards are absolutely positioned with `top:${pos}%` inside a fixed `min-height: 360px`
column. As the participant count grows, round 1 has more matches, so the percentage gap between stacked
cards shrinks below the card height (~70-95px) and they collide. Fix: `Bracket.svelte` now computes a
dynamic per-section min-height from the busiest column. `CARD_SLOT = 104px` (tallest card ~95px + gap);
`wbMaxPerColumn`/`lbMaxPerColumn` = max matches in any single wing column; `wbMinHeight =
wbMaxPerColumn * CARD_SLOT + 24` (and same for LB), applied as inline `min-height` on each section's
column container. Now the column grows tall enough that the % spacing always exceeds card height.

## Feature — per-player stats view when team_size > 1 — DONE
In team mode the main Stats tab aggregates by TEAM, so individual players were hidden. Added
`TournamentGetPlayerStats()` (Go, `tournament_stats.go`) returning `[]TournamentPlayerStat` (Name, Team,
IsBot, MatchesPlayed, Goals, OwnGoals, Assists, Saves, Shots, Demolitions, Score), aggregating every
completed match's `player_stats` keyed by player name (team name resolved via the team roster). Frontend:
`Tournament.svelte` fetches it in `refreshDerived()` into `playerStats` and passes it to `<StatsPanel>`;
`StatsPanel.svelte` shows a **Player Stats** tab (only when `team_size > 1`) with the per-player table.

## Fix — mercy rule ended the match before the goal replay finished — DONE
The Round 14 fix stopped the match the instant the mercy threshold was hit, cutting off the in-game goal
replay/celebration. v4 waited until the next kickoff countdown before sending the stop. v5 equivalent:
`captureMatchResult` now sets `mercySettled = mercyTriggered && phase == MatchPhaseCountdown` and only
returns once the phase reaches **Countdown** (the post-replay 3-2-1 before the next kickoff). The phase
sequence after a goal is GoalScored → Replay → Countdown, so waiting for Countdown guarantees the replay
has finished before the runner calls `StopMatch`.

## Fix — custombot IndexError on packet.balls[0] during replays — DONE
`rlbot_bridge_v5.py` dereferenced `packet.balls[0]` unconditionally; during replays/celebration the v5
server drops the ball from the packet, so the list is empty → `IndexError: list index out of range`.
Fix: `packet_to_gamestate_v5` now returns a 3-tuple `(state, new_prev, ball_present)` where
`ball_present = len(balls) > 0`, and only populates `state.ball` when present. `rlbot_agent_v5.py` unpacks the 3-tuple at both call sites; in `get_output` it early-returns idle (zero) controls when
`not ball_present`, so the bot stops acting once the match is over. (The empty-ball signal is also the
future hook for a post-match celebration behavior, if you want it later.)

---

# Round 16–22 — bracket duplication (save/load), overlap, stats refresh, winner bubble, stop match (all implemented, builds green)

You tested the Round 15 build across several rounds and reported a batch of issues. All are fixed.
Bindings regenerated to **68 methods / 41 enums / 52 models** (added `TournamentStopMatch`);
`go build ./...` → exit 0; `go test ./...` → `ok gui`; `pnpm run build` → clean (398 modules).

## Fix — a team that LOST still advanced in the winners bracket (duplicated across WB and LB) — DONE
Root cause (your hypothesis was exactly right): a **save/load pointer-identity bug**. v4's `Participant` is a
`@dataclass`, so `completed_match.participant1 != winner` is a **value** comparison (dataclass auto-generates
`__eq__`), and after a save→load round-trip each match's deep-copied participant still compares equal by value.
My Go port used **pointer identity** (`completed.Participant1 != winner`, `match.Participant1 != f.Winner`, etc.).
Within one session the pointers are shared so it works — but after closing/reopening the GUI, `json.Unmarshal`
deep-copies each participant into each match, so the WB match's participant and the LB/next match's participant
become *different pointers with the same `ParticipantID`*. Then `findCompletedUnprocessedFeeder` thinks the
winner "hasn't been placed yet" and re-advances it, and `populateLosersBracketMatch` mis-detects the loser — so a
loser gets pushed into the winners bracket too, duplicating across the bracket.
Fix: added `sameParticipant(a, b)` (compares `ParticipantID`) and replaced every identity comparison in
`tournament_advance.go` and `tournament_bracket_single.go`. Regression guard: `tournament_advance_test.go`
(`TestHandleByeSituationAfterReload` fails with pointer equality, passes with value equality).

## Fix — bracket cards still overlap in larger brackets — DONE
The gap between stacked cards is `height/(n+1)` (cards spread at `spacing*(i+1)`, `spacing = 100/(n+1)`), so to
guarantee gap ≥ card height we need `height ≥ (n+1)*CARD_SLOT`. The old formula used `maxPerColumn * CARD_SLOT`
(under-sized by one slot). Fixed to `(maxPerColumn + 1) * CARD_SLOT + 24` for both WB and LB sections.

## Fix — losers-bracket spacing wrong when a later LB round has MORE matches than round 1 — DONE
`computeLosersBracketPositions` always spread the **first** LB round evenly, so when a later LB round gained more
matches (it receives losers from several WB rounds) those extra matches collided. Fixed to spread the round with
the **most** matches (mirrors what the WB wing already does); every other round's matches sit at their feeders'
midpoint or the largest vertical gap.

## Fix — stats / match-history panels empty after opening a saved tournament — DONE
`refreshDerived()` was only called in `onMount(refresh)`, not in `openFromLanding()`, `importJSON()`, or
`createTournament()`, so opening a saved tournament from the landing page left the panels empty. Added
`await refreshDerived()` to all three.

## Fix — winner bubble showed "Winner: " with no name — DONE
The Go `TournamentStatsReport`/`TournamentHistoryReport` return `Winner`/`WinnerTeam` as **strings**, but the
frontend read `stats.winner_team?.name || stats.winner?.name` (treating them as objects → `.name` undefined).
Changed to `stats.winner_team || stats.winner` in both the stats and history tabs of `StatsPanel.svelte`.

## Feature — Stop Match button (v4 parity) — DONE
v4 had a Stop button to abort a live match (e.g. when something didn't load right) so you can re-start it. v5
had `App.StopMatch(shutdownServer bool)` but it wasn't bound for tournament use. Added `TournamentStopMatch()`
(Go): sets an `atomic.Bool` stop flag that the blocking `captureMatchResult` loop checks each iteration, so it
returns a `Stopped` result that skips recording (the match stays incomplete and can be re-started), then sends
`StopCommandT{ShutdownServer:false}` to tear down the live game. Frontend: the bracket card shows a red **⏹ Stop**
button while a match is in progress; clicking it calls `TournamentStopMatch`, clears `startingMatchId`, and stops
the auto-start timer.

---

# Round 23 — LB connectors clarity + toast.info crash + Stop button moved to toolbar (all implemented, builds green)

You tested the Round 16–22 build and reported three more issues. All three fixed. No Go API changed (the
stop-flag backend from Round 16–22 is unchanged), so bindings stay at **68 methods**; `go build ./...` → exit 0;
`go test ./...` → `ok gui`; `pnpm run build` → clean (398 modules).

## Fix — losers-bracket connectors hard to follow (couldn't tell which R1 match feeds which R2 match) — DONE
The connectors were drawn in a single uniform color per section (cyan for WB, amber for LB), so where several
round-1 matches feed one round-2 match you couldn't trace which line came from which card. Fix: each connector
is now **color-coded by its source match**. `Bracket.svelte` builds a per-section color map (`wbColorMap`/
`lbColorMap`) assigning each match a palette color by its index, and `drawSectionConnectors` strokes each elbow
in its source match's color. A matching colored dot (`.connector-dot`) is drawn on each card so you can follow a
line from card to card by color.

## Fix — Stop button threw "toast.info is not a function" — DONE
`svelte-5-french-toast`'s toast object exposes `success`/`error`/`loading`/`custom` and is itself callable, but
has **no** `.info()` (and no `.default()`). `stopMatch()` called `toast.info("Match stopped")` → TypeError. Fixed
to call the default export directly: `toast("Match stopped")` (a neutral/blank toast).

## Fix — Stop button moved from the match card to the match-control toolbar — DONE
You preferred the Stop control up in the toolbar (with Start Match / Open Rocket League / auto-start) rather than
hunting for the in-progress card. Added an `onStop` prop to `AutoStartToolbar.svelte` (red **⏹ Stop Match** button
shown only while `matchInProgress`), wired `onStop={stopMatch}` in `Tournament.svelte`, and removed the per-card
Stop button + its `onStop` prop from `Bracket.svelte` (and the now-dead `.matchActions button.stop` CSS).

## Still needs your input (live testing — you're at work)
- Live end-to-end: confirm the color-coded connectors read clearly in a real 8-team double-elim bracket, and that
  the toolbar Stop button actually aborts a live RL match and lets you re-start it.

---

# Round 24 — LB chain layout (horizontal connectors) + per-card match IDs (color-coding reverted) — implemented, builds green

You tested the Round 23 build. The color-coded connectors were **too noisy** ("too noisy if they don't make it
clear so should revert to single color"), and the LB spacing was still wrong. Your key insight reframed the whole
problem: this was never a frontend layout bug — it's that each LB round-2 match has exactly **one** LB round-1
feeder (the other input is a WB loser), so every LB→LB connector should be a flat **horizontal** line. The Go
generator already links LB R1 match *i* → LB R2 match *i* one-to-one; the frontend was the only thing breaking it
by anchoring on the *widest* round and using midpoint math (which produced the confusing crossings).

## Fix — LB laid out as vertical "chains" so connectors are horizontal and never cross — DONE
Rewrote `computeLosersBracketPositions` in `Bracket.svelte`: anchor on the **first** LB round (spread evenly across
the full column height), then for each later round a match **copies the position of its single LB feeder** (so the
connector is a flat horizontal line). Matches with no LB feeder (entry points fed only by WB losers via
`loser_next_match_id`) are dropped into the largest remaining vertical gaps. Result: every LB→LB connector is
horizontal and non-crossing; the WB→LB feeds are intentionally not drawn (they're the "invisible" cross-section
connectors).

## Fix — reverted connector color-coding to a single color per section — DONE
Removed `CONNECTOR_PALETTE`, `buildColorMap`, `wbColorMap`, `lbColorMap`. `drawSectionConnectors` now takes a single
`stroke` color (`WB_CONNECTOR_COLOR` cyan / `LB_CONNECTOR_COLOR` amber). The colored `.connector-dot` is gone.

## Fix — per-card match ID + target match ID labels — DONE
Each card now shows its own `match_id` (top-left) and the match it feeds, `next_match_id` (top-right), in a small
`.match-id-row`. This replaces the colored dot as the way to trace a connection: read the ID on the left of a card
and find the card whose right-side target matches it.

## Fix — WB R1 slight overlap + LB R2 overlap — DONE
The match-id row made cards taller, so WB R1 cards touched. Bumped `WB_CARD_SLOT` to 116px. For the LB, an R2 card
must fit *between* two R1 cards, so `LB_CARD_SLOT = WB_CARD_SLOT + 104` (a full extra card height per slot). Column
min-height stays `(maxPerColumn + 1) * slot + 24`.

## Build status
No Go API changed (bindings stay at **68 methods**). `go build ./...` → exit 0; `go test ./...` → `ok gui`;
`pnpm run build` → clean (398 modules).

## Still needs your input (live testing — you're at work)
- Live end-to-end: confirm the chain layout reads clearly (horizontal LB connectors, no crossings) and that the
  per-card match-ID labels make each connection traceable, in a real 33-team double-elim bracket.

---

# Round 25 — LB layout reworked to spread-each-round-evenly (deep-round overlap fixed) — implemented, builds green

You tested the Round 24 build. Two problems: (1) the LB spacing was too tall — you wanted `WB spacing × 2`, not
`WB + one card`; and (2) the **last few LB rounds still overlapped**.

## Root cause of the deep-round overlap
The Round 24 "chain" approach copied each LB match's *feeder's* position. But WB-fed "entry point" matches (no LB
feeder) were all dumped into the single largest center gap, and that pile-up propagated down the chain into the
deep rounds (L42–L48), stacking them on top of each other.

## Fix — spread every LB round evenly, sorted by feeder position — DONE
Rewrote `computeLosersBracketPositions` again. It now takes `wbPositions` as a second argument and, for **every**
LB round, sorts that round's matches by their feeder's vertical position (LB feeder via `next_match_id`, or WB
feeder via `loser_next_match_id` → `wbPositions`) and spreads them **evenly across the full column height** — exactly
like a WB wing round. Spreading every round evenly guarantees no column overlaps regardless of how round sizes
grow/shrink; sorting by feeder position keeps the connectors from crossing.

## Fix — LB slot is now WB slot × 2 — DONE
`LB_CARD_SLOT = WB_CARD_SLOT * 2` (was `WB_CARD_SLOT + 104`).

## Build status
No Go API changed (bindings stay at **68 methods**). `go build ./...` → exit 0; `go test ./...` → `ok gui`;
`pnpm run build` → clean (398 modules).

## Still needs your input (live testing — you're at work)
- Live end-to-end: confirm the deep LB rounds no longer overlap and the connectors still read clearly (sorted by
  feeder position, so they shouldn't cross), and that `WB × 2` LB spacing looks right.

---

# Round 26 — "Wannabe SSL" corruption: definitive diagnosis + one-click Repair Bracket (implemented, builds green)

You pushed back on my earlier "stale artifact" theory: WB R2+ was played *after* the Round-18 fix, so if the fix
worked, how did the later rounds still come out corrupted (`W15 = BP vs BP`, `L1 = SE vs SE`, etc.)? That was a
fair challenge, so I settled it definitively rather than guessing.

## Diagnosis — the fix is genuinely complete; the corrupt file predates it
1. **The current code cannot reproduce the corruption.** I added `tournament_reload_test.go`
   (`TestReloadMidTournamentNoDuplication`) that replays your *exact* workflow — play the early rounds, `json.Marshal`
   to disk, `json.Unmarshal` back into a fresh state, then keep playing — and it **passes**: no team is ever placed on
   both sides of a match, no team is duplicated across WB and LB. The value-equality (`sameParticipant`) fix from
   Round 18 is complete.
2. **The corrupt save is a pre-fix artifact.** The file's `created_at` is `2026-11-08T20:13:44Z`, before the fix was
   in the running `gui.exe`. Crucially, the file's **links** (`next_match_id` / `loser_next_match_id`) are all
   *correct* (e.g. `W13.next=W15`, `W14.next=W15`); only the **participant slots** are wrong. That's the signature of
   the old pointer-identity bug: the structure was always right, only the per-match entrants got mis-copied on reload.

So it was never "baked into the links at creation" — the links were fine. The old code just wrote the wrong entrants
into the slots when you reloaded. A repair is therefore possible: re-derive each match's entrants from its (intact)
feeder links.

## Feature — 🔧 Repair Bracket button (re-derive entrants from feeder results) — DONE
New bound method `App.TournamentRepairBracket()` (`tournament_repair.go`). It walks every match in topological
(round-ascending) order and re-derives each match's two participants from its feeders: a feeder whose `next_match_id`
points here contributes its **winner**; a feeder whose `loser_next_match_id` points here contributes its **loser**.
- If the re-derived set **equals** what's stored (the corruption was only a cosmetic duplicate, e.g. a bye rendered
  as "SE vs SE"), it fixes the slots and **keeps** the recorded result.
- If the re-derived set **differs**, the recorded result was made against the wrong entrants, so it **un-completes**
  the match for re-play; downstream matches re-derive from the now-empty slot, cascading the correction.

It also recomputes team W/L records from the surviving completed matches and re-declares the winner if the grand
final is complete. Frontend: a **🔧 Repair Bracket** button in the always-visible header (shown for double-elim
tournaments) calls it and refreshes stats/history.

Regression guards (`tournament_repair_test.go`): `TestRepairBracketRestoresCorruption` (corrupt W15 → restored to the
correct distinct pair), `TestRepairBracketNoFalsePositives` (a healthy bracket is left byte-for-byte unchanged), and
`TestRepairBracketRoundTrip` (a repaired state survives another save/load).

## Build status
Bindings regenerated to **69 methods** (added `TournamentRepairBracket`). `go build ./...` → exit 0;
`go test ./...` → `ok gui`; `pnpm run build` → clean (398 modules).

## Still needs your input (live testing — you're at work)
- Open the corrupt "Wannabe SSL" save, click **🔧 Repair Bracket**, and confirm the duplicated entrants collapse back
  to the correct teams and the bracket continues cleanly from there.

---

# Round 27 — Repair Bracket now auto-completes losers-bracket BYES (the stall you hit) — implemented, builds green

You tested the Round 26 Repair Bracket and it "almost worked" — but it didn't handle the **LB R1 bye**, so the
tournament couldn't continue. Exactly right: my first repair pass re-derived each match's entrants but **never
re-ran the bye auto-completion**. A bye match (e.g. `L1 = Silver Eagles` alone) has only one entrant; the old code
saw the corrected set `[SE]` differed from the stored `[SE, SE]` and **un-completed** it — leaving a one-entrant match
that can never be played, so `L2` never received Silver Eagles and the whole LB stalled.

## Fix — repair auto-completes byes and cascades the winner downstream — DONE
Rewrote the per-match logic in `TournamentRepairBracket` (`tournament_repair.go`):
- A match with **one** entrant whose **feeders are all completed** is a **bye** → auto-complete it (winner = the lone
  entrant) so the winner propagates into the next match.
- A match with one entrant but a **pending** feeder stays open (it's not a real bye yet).
- A match with **two** entrants behaves as before (cosmetic-duplicate fix keeps the result; wrong entrants un-complete
  for re-play).

I also fixed a **same-round ordering hazard**: a WB match's loser feeds an LB match in the *same* round number, so the
sort now orders **WB before LB within each round** (then by id), guaranteeing every feeder is finalized before its
child re-derives.

Regression guard: `TestRepairBracketByeAutoCompletes` builds a 9-team bracket, plays WB R1, corrupts an LB bye by
duplicating its lone entrant into both slots, repairs, and asserts the bye auto-completes **and** its winner advanced
into the next LB match.

## Build status
No new bound methods (still **69 methods**). `go build ./...` → exit 0; `go test ./...` → `ok gui`;
`pnpm run build` → clean (398 modules).

## Still needs your input (live testing — you're at work)
- Re-open the corrupt "Wannabe SSL" save and click **🔧 Repair Bracket** again: the LB R1 bye should now auto-advance
  (Silver Eagles → L2) and the bracket should continue cleanly instead of stalling.