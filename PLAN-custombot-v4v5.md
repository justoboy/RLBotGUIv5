# PLAN — Make `custombot` run on BOTH RLBot v4 and v5

Status: **IMPLEMENTED (v5 bridge + agent + bot package written and syntax-checked). End-to-end live
test pending (needs Rocket League).** Scope: `D:\RLBotTraining\custombot`. This doc lives in the GUI
workspace only so all planning docs sit together.

> **Correction to earlier drafts of this doc:** v5 DOES have an official Python bot library —
> `github.com/RLBot/python-interface` (`pip install rlbot`, base class `rlbot.managers.bot.Bot`). The
> earlier claim that "v5 has no Python library, hand-roll a socket client / use a Go shim" was WRONG.
> The Gemini-sourced claim that `jumped`/`double_jumped` were removed and Dropshot tiles were lost was
> also WRONG — the real v5 schema has both (plus a richer `air_state` enum). This doc now reflects the
> real, verified schema.

---

## 1. What `custombot` actually is (two halves)

| Half | Lives in | Talks to RLBot? | Engine |
|------|----------|-----------------|--------|
| **Training** | `main.py`, `rollout.py`, `agent.py`, `obs.py`, `reward.py`, `fsp.py`, `profile_tracker.py` | **NO** | RLGym + **RocketSim** (custom forks). Pure offline sim. |
| **Deployment** | `rlbot_agent.py` (`CheckpointBot`, v4) + `rlbot_bridge.py` (v4) and `rlbot_agent_v5.py` (`CheckpointBotV5`) + `rlbot_bridge_v5.py` (v5) | **YES** | v4: `rlbot.agents.standalone` ctypes. v5: `rlbot.managers.bot.Bot` flat. |

**Training is version-neutral** — it never loads RLBot core; it runs entirely in RocketSim. The
v4↔v5 problem is **100% a deployment/bridge problem**.

---

## 2. The decisive constraint: v4 and v5 `rlbot` COLLIDE BY NAME

Both the v4 Python bot library and the v5 `python-interface` are distributed as a package literally
named **`rlbot`**. They therefore **cannot coexist in one venv**. This is the single fact that dictates
the whole architecture:

- The repo-root training venv (`custombot/.venv`) keeps the **v4** `rlbot` (1.68.0) — training + v4
  deploy use it. **Left untouched.**
- Each v5 bot gets its **own** venv (e.g. `bots/makora_v5/venv`) that installs the **v5** `rlbot`
  (editable from `D:/RLBotPythonInterface`) + `rlbot_flatbuffers` (Rust backend) + the same torch/
  rlgym/rocket_learn/EARL stack.

This is exactly the pattern the project already uses for v4 (`bots/makora/venv` is a junction to the
training `.venv`). The match config selects which bot (and therefore which venv) runs: the v4 GUI
reads `bots/<name>/<name>.cfg`; the v5 GUI reads `bots/<name>/bot.toml`.

**v4 vs v5 launch difference (verified in `RLBotCS/ManagerTools/LaunchManager/BotLauncher.cs`):**
- v4 auto-activates the bot venv via `use_virtual_environment = True` in the `.cfg`.
- v5 has **no** such flag. The core runs `cmd.exe /c <run_command>` with `WorkingDirectory = root_dir`
  and injects `RLBOT_AGENT_ID` + `RLBOT_SERVER_PORT`. So the v5 `run_command` MUST point explicitly at
  the v5 venv's python: `venv\Scripts\python.exe main.py`.

---

## 3. Exact v4 → v5 field mapping (implemented in `rlbot_bridge_v5.py`)

### 3.1 Packet-level
| v4 (ctypes) | v5 (flat `GamePacket`) | note |
|---|---|---|
| `packet.num_cars` | `len(packet.players)` | v5 has no `num_*` scalars |
| `packet.num_boost` | `len(packet.boost_pads)` | |
| `packet.num_teams` | `len(packet.teams)` | |
| `packet.num_tiles` | `len(packet.tiles)` | |
| `packet.game_cars[i]` | `packet.players[i]` | |
| `packet.game_boosts[j]` | `packet.boost_pads[j]` | state only; static loc in field_info |
| `packet.dropshot_tiles[t].tile_state` | `packet.tiles[t]` (already a `TileDamageLevel`) | flat byte list |
| `packet.game_info.*` | `packet.match_info.*` | |
| `packet.teams[t].team_index/.score` | `packet.teams[t].team_index/.score` | same names |

### 3.2 Car (`game_cars[i]` → `players[i]`)
| v4 | v5 | note |
|---|---|---|
| `pc.team` / `pc.boost` | `p.team` / `p.boost` | |
| `pc.jumped` / `pc.double_jumped` | `p.has_jumped` / `p.has_double_jumped` | |
| `pc.is_demolished` (bool) | `p.demolished_timeout > 0` (float) | **type change** |
| `pc.has_wheel_contact` (bool) | `int(p.air_state) == 0` (OnGround) | v5 richer `air_state` enum |
| `pc.physics.*` | `p.physics.*` | same sub-fields |
| ball touch via ball | `p.latest_touch.game_seconds` | see §3.3 |

### 3.3 Ball + touch (the biggest semantic change)
- **v4:** one ball `packet.game_ball`; toucher = `game_ball.latest_touch.player_index`; edge via
  `latest_touch.time_seconds`.
- **v5:** **each `players[i]` carries its own `latest_touch.game_seconds`.** The toucher is simply the
  player whose `latest_touch.game_seconds` changed this frame. So the v5 bridge tracks a **per-player
  list** of previous touch times (vs v4's single `prev_touch_time`) and sets `ball_touches=1` on the
  player whose time changed.
- v5 also exposes `packet.balls[0].charge_level` (dropshot energy: -1 not dropshot, 0 none, 1 charged,
  2 supercharged), which the v5 bridge writes into `ball.dropshot_charge_level` (v4 left it `None`).

### 3.4 Match settings → ModeHolder
| v4 | v5 |
|---|---|
| `match_settings.GameMode()` | `match_config.game_mode` (enum `GameMode`) |
| `ms.GravityOption()` | `match_config.mutators.gravity` |
| `ms.BoostStrengthOption()` | `match_config.mutators.boost_strength` |
| `ms.BallMaxSpeedOption()` | `match_config.mutators.ball_max_speed` |
| `ms.BallSizeOption()` | `match_config.mutators.ball_size` |
| `ms.BallBouncinessOption()` | `match_config.mutators.ball_bounciness` |
| `ms.BoostOption()` | `match_config.mutators.boost_amount` (enum) |

v5 `GameMode` enum: `Soccar=0, Hoops=1, Dropshot=2, Snowday=3, Rumble=4, Heatseeker=5, Gridiron=6,
Knockout=7`. **Behavior note:** the v4 `_GAME_MODE_MAP` treated enum 3 (Hockey in v4) as SOCCAR; in v5
enum 3 is **Snowday**, which custombot HAS a dedicated mode for (`MODE_SNOWDAY`), so the v5 map maps
it correctly. Rumble(4)/Knockout(7) → SOCCAR family. **Verify against BC/RL training data that Snowday
was actually trained before relying on this** (flagged for review).

---

## 4. Auto-detect v4 vs v5 (implemented)

```python
def detect_protocol(packet) -> str:
    return "v5" if hasattr(packet, "players") and not hasattr(packet, "num_cars") else "v4"
```

In practice the two bot classes are **separate** (`CheckpointBot` for v4, `CheckpointBotV5` for v5),
each wired to its own GUI/config, so the discriminator is mostly a safety check; the real dispatch is
"which bot package did the match config launch."

---

## 5. What was implemented (this pass)

- `custombot/rlbot_bridge_v5.py` — `packet_to_gamestate_v5`, `read_scoreline_v5`,
  `match_settings_to_holder_v5`, `detect_protocol`. Reuses the protocol-agnostic helpers from the v4
  bridge (`_vec`, `_physics`, `BumpDetector`, mutator dicts). `field_info` is passed **explicitly**
  (the v5 packet is a packed Rust object we do not monkeypatch, unlike v4's ctypes packet).
- `custombot/rlbot_agent_v5.py` — `CheckpointBotV5(rlbot.managers.bot.Bot)`. `initialize()` loads the
  actor; `get_output(packet) -> flat.ControllerState` mirrors the v4 tick-skip + obs + infer + decode.
- `bots/makora_v5/{bot.toml,main.py,requirements.txt}` — the v5 bot package. `main.py` constructs
  `MakoraV5Bot().run()`; `bot.toml` sets `run_command = 'venv\Scripts\python.exe main.py'` and
  `agent_id = "custombot/makora"`.

Both new Python files pass `py_compile`. The v5 venv has NOT been created yet (see §6).

---

## 6. Remaining steps (mostly yours — needs Rocket League + a real v5 venv)

1. Create the v5 venv and install the v5 interface + training stack:
   ```
   py -3.11 -m venv "D:\RLBotTraining\custombot\bots\makora_v5\venv"
   "D:\RLBotTraining\custombot\bots\makora_v5\venv\Scripts\python.exe" -m pip install -r "D:\RLBotTraining\custombot\bots\makora_v5\requirements.txt"
   ```
   (v5 `rlbot` requires Python ≥3.11; the training `.venv` may be a different version — that's fine,
   they're separate venvs.)
2. Point the v5 GUI's bot search path at `D:\RLBotTraining\custombot\bots` so `makora_v5/bot.toml` is
   discovered (the GUI's `recursiveTomlSearch` skips `venv` dirs, so the bot's own venv won't be parsed).
3. Launch a v5 match with `makora_v5` and confirm the policy plays (watch `logs/live_makora.log`).
4. **Review flag:** confirm the v5 `_GAME_MODE_MAP_V5` Snowday→MODE_SNOWDAY mapping matches what the
   BC/RL checkpoints were actually trained on (v4 treated that enum slot as SOCCAR).

---

## 7. Cleanup decision: the cloned v5 core at `D:/RLBotCore`

**KEEP it.** It is the v5 core (C# `RLBotCS`) and the source of truth for the v5 protocol (used to
verify Dropshot tiles/charge_level and the launcher behavior). `D:/RLBotPythonInterface` and
`D:/RLBotFlatbuffersPython` are likewise the source of truth for the v5 Python API and the `.pyi`
field names; keep them until the v5 path is end-to-end verified.

---

## 8. Checklist

- [x] Confirm training half is RLBot-free (RLGym/RocketSim) → nothing to "v5-upgrade" there
- [x] Confirm deploy half is v4 (`rlbot` StandaloneBot + ctypes bridge)
- [x] Produce exact v4→v5 field mapping (§3)
- [x] Produce reliable auto-detect discriminator (§4)
- [x] DECISION RESOLVED: v5 HAS an official Python interface (`rlbot.managers.bot.Bot`) → Option A via official lib
- [x] Implement `rlbot_bridge_v5.py` + `rlbot_agent_v5.py` (`CheckpointBotV5`)
- [x] Create v5 bot package (`bots/makora_v5`: `bot.toml` + `main.py` + `requirements.txt`)
- [ ] **YOU:** create the v5 venv + `pip install -r bots/makora_v5/requirements.txt` (§6 step 1)
- [ ] **YOU:** point the v5 GUI bot search at `custombot/bots` (§6 step 2)
- [ ] **YOU:** end-to-end test a checkpoint bot in a live v5 match (§6 step 3)
- [ ] **YOU:** review the Snowday mode mapping (§6 step 4)
