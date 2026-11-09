package main

import (
	"math"

	rlbot "github.com/RLBot/go-interface"
	"github.com/RLBot/go-interface/flat"
)

// Tournament match launch + result capture, ported from the v4 fork's
// match_runner.start_match_helper.
//
// v4 launched a match via the Python SetupManager, then polled GameTickPacket
// in a loop: it baselined every player's score_info on the first packet of the
// match and computed every later value as a delta from that baseline. The
// baseline is what makes the "Continue And Spawn" LAN workflow safe: when a
// human is in the match the RLBotServer is NOT restarted, so the in-game
// scoreboard (and the packet's per-player stats) can still carry the PREVIOUS
// match's final numbers. Differencing against a per-player baseline captured
// once the new match is live cancels that stale value out, so only goals scored
// in THIS match are counted and a stale scoreboard can never flip a result.
//
// v5's go-interface has no way to end a match in-game (DesiredMatchInfoT only
// exposes WorldGravityZ and GameSpeed, no EndMatch). So the mercy rule here
// cannot call set_game_state(end_match=True) like v4 did; instead it snapshots
// the scores at the moment the threshold is reached and returns, and the runner
// tears the match down with StopMatch. See PLAN-tournament-migration-user-review.md.

// playerBaseline is a per-player stat snapshot captured at match start; every
// reported stat is a delta from this.
type playerBaseline struct {
	score       uint32
	goals       uint32
	ownGoals    uint32
	assists     uint32
	saves       uint32
	shots       uint32
	demolitions uint32
}

// TournamentMatchResult is the captured outcome of a single tournament match,
// mirroring v4's start_match_helper return dict.
type TournamentMatchResult struct {
	TeamScores     []TournamentTeamScore       `json:"team_scores"`
	Players        []TournamentMatchPlayerStat `json:"players"`
	DurationSeconds int                        `json:"duration_seconds"`
	HasDuration    bool                        `json:"has_duration"`
	IsOvertime     bool                         `json:"is_overtime"`
	MercyTriggered bool                         `json:"mercy_triggered"`
	Stopped        bool                         `json:"stopped"`
	Error          string                       `json:"error,omitempty"`
}

// TournamentTeamScore is one team's final score for a match.
type TournamentTeamScore struct {
	TeamIndex int `json:"team_index"`
	Score     int `json:"score"`
}

// TournamentMatchSettings carries everything needed to launch a tournament
// match, mirroring the subset of StartMatchOptions the runner needs plus the
// mercy-rule threshold.
type TournamentMatchSettings struct {
	Map           string                `json:"map"`
	GameMode      string                `json:"gameMode"`
	Scripts       []BotInfo             `json:"scripts"`
	MutatorSettings flat.MutatorSettingsT `json:"mutatorSettings"`
	ExtraOptions  ExtraOptions          `json:"extraOptions"`
	Launcher      string                `json:"launcher"`
	LauncherArg   string                `json:"launcherArg"`
	MercyRule     int                   `json:"mercyRule"`
}

// buildTournamentMatchConfig assembles the flat match config from a match's
// blue/orange players and the tournament settings. Mirrors App.StartMatch's
// body so tournament matches launch identically to normal ones.
func (a *App) buildTournamentMatchConfig(blue, orange []PlayerJs, settings TournamentMatchSettings) *flat.MatchConfigurationT {
	var gameMode flat.GameMode
	switch settings.GameMode {
	case "Soccar":
		gameMode = flat.GameModeSoccar
	case "Hoops":
		gameMode = flat.GameModeHoops
	case "Dropshot":
		gameMode = flat.GameModeDropshot
	case "Snowday":
		gameMode = flat.GameModeSnowday
	case "Rumble":
		gameMode = flat.GameModeRumble
	case "Heatseeker":
		gameMode = flat.GameModeHeatseeker
	case "Gridiron":
		gameMode = flat.GameModeGridiron
	case "Knockout":
		gameMode = flat.GameModeKnockout
	default:
		gameMode = flat.GameModeSoccar
	}

	var launcher flat.Launcher
	switch settings.Launcher {
	case "steam":
		launcher = flat.LauncherSteam
	case "epic":
		launcher = flat.LauncherEpic
	case "legendary", "heroic":
		launcher = flat.LauncherCustom
		settings.LauncherArg = settings.Launcher
	case "custom":
		launcher = flat.LauncherCustom
	case "nolaunch":
		launcher = flat.LauncherNoLaunch
	default:
		launcher = flat.LauncherNoLaunch
	}

	playerConfigs := make([]*flat.PlayerConfigurationT, len(blue)+len(orange))
	for i, playerInfo := range blue {
		playerConfigs[i] = playerInfo.ToPlayer().ToPlayerConfig(0)
	}
	for i, playerInfo := range orange {
		playerConfigs[i+len(blue)] = playerInfo.ToPlayer().ToPlayerConfig(1)
	}

	scriptConfigs := make([]*flat.ScriptConfigurationT, len(settings.Scripts))
	for i, info := range settings.Scripts {
		scriptConfigs[i] = info.ToScriptConfig()
	}

	return &flat.MatchConfigurationT{
		AutoStartAgents:       settings.ExtraOptions.AutoStartAgents,
		WaitForAgents:         settings.ExtraOptions.WaitForAgents,
		GameMapUpk:            settings.Map,
		PlayerConfigurations:  playerConfigs,
		ScriptConfigurations:  scriptConfigs,
		GameMode:              gameMode,
		Mutators:              &settings.MutatorSettings,
		Freeplay:              settings.ExtraOptions.Freeplay,
		EnableRendering:       settings.ExtraOptions.EnableRendering,
		EnableStateSetting:    settings.ExtraOptions.EnableStateSetting,
		PerformanceMonitor:    settings.ExtraOptions.PerformanceMonitor,
		InstantStart:          settings.ExtraOptions.InstantStart,
		SkipReplays:           settings.ExtraOptions.SkipReplays,
		AutoSaveReplay:        settings.ExtraOptions.AutoSaveReplay,
		Launcher:              launcher,
		LauncherArg:           settings.LauncherArg,
		ExistingMatchBehavior: flat.ExistingMatchBehavior(settings.ExtraOptions.ExistingMatchBehavior),
	}
}

// StartTournamentMatch launches a tournament match and blocks until it ends,
// returning the captured result. When settings.ExtraOptions.ExistingMatchBehavior
// is Continue And Spawn (a human is present), the RLBotServer is not restarted,
// which is exactly why the per-player baseline delta logic is required.
func (a *App) StartTournamentMatch(blue, orange []PlayerJs, settings TournamentMatchSettings) TournamentMatchResult {
	match := a.buildTournamentMatchConfig(blue, orange, settings)

	// Launch and wait until the match is in an active phase. This reuses the
	// existing tested launch dance (including the reconnect that distinguishes
	// the new match's packets from the previous match's).
	if err := StartAndWaitForMatch(a.rlbotAddress, match); err != nil {
		return TournamentMatchResult{Error: err.Error()}
	}

	return a.captureMatchResult(settings.MercyRule)
}

// captureMatchResult connects to the (already-running) match, baselines player
// stats on the first live packet, and polls until the match ends or the mercy
// rule triggers, then returns the per-team and per-player deltas.
func (a *App) captureMatchResult(mercyRule int) TournamentMatchResult {
	conn, err := rlbot.Connect(a.rlbotAddress)
	if err != nil {
		return TournamentMatchResult{Error: "Failed to connect to RLBotServer: " + err.Error()}
	}

	// CloseBetweenMatches=false: we connect mid-match and want to keep reading
	// through to the Ended phase without the server tearing the connection down.
	conn.SendPacket(&flat.ConnectionSettingsT{
		AgentId:              "",
		WantsBallPredictions: false,
		WantsComms:           false,
		CloseBetweenMatches:  false,
	})

	var baseline map[int]playerBaseline
	var final *flat.GamePacketT
	mercyTriggered := false

	for final == nil {
		// Operator pressed Stop: tear down the live match and return a Stopped
		// result (no recording), so the match stays incomplete and can be re-started.
		if a.tournamentMatchStopped.Load() {
			conn.SendPacket(&flat.DisconnectSignalT{})
			return TournamentMatchResult{Stopped: true}
		}
		pkt, err := conn.RecvPacket()
		if err != nil {
			conn.SendPacket(&flat.DisconnectSignalT{})
			return TournamentMatchResult{Error: "Connection closed while capturing result: " + err.Error()}
		}

		switch p := pkt.Value.(type) {
		case *flat.FieldInfoT:
			conn.SendPacket(&flat.InitCompleteT{})
		case *flat.GamePacketT:
			if baseline == nil {
				baseline = captureBaseline(p)
			}
			goalDelta := goalTotalsByTeam(p, baseline)
			var phase flat.MatchPhase
			if p.MatchInfo != nil {
				phase = p.MatchInfo.MatchPhase
			}
			if mercyRule > 0 && !mercyTriggered {
				if int(math.Abs(float64(goalDelta[0]-goalDelta[1]))) >= mercyRule {
					mercyTriggered = true
				}
			}
			ended := phase == flat.MatchPhaseEnded
			// When mercy triggers, do NOT capture immediately (that would cut off the goal replay,
			// exactly the v4 bug). The match is still live, so after the goal it plays GoalScored ->
			// Replay -> then Countdown (the next kickoff's 3-2-1). Waiting for Countdown is the v5
			// equivalent of v4's is_kickoff_pause wait: the replay has finished and it is safe to stop.
			mercySettled := mercyTriggered && phase == flat.MatchPhaseCountdown
			if ended || mercySettled {
				final = p
			}
		case *flat.DisconnectSignalT:
			conn.SendPacket(&flat.DisconnectSignalT{})
			if final != nil {
				// already handled above; ensure loop exits
			}
			return TournamentMatchResult{Error: "Match connection closed before result captured"}
		}
	}

	conn.SendPacket(&flat.DisconnectSignalT{})
	return buildResult(final, baseline, mercyTriggered)
}

// captureBaseline snapshots every player's stats from the first live packet.
func captureBaseline(gp *flat.GamePacketT) map[int]playerBaseline {
	base := make(map[int]playerBaseline, len(gp.Players))
	for i, player := range gp.Players {
		if player == nil || player.ScoreInfo == nil {
			continue
		}
		si := player.ScoreInfo
		base[i] = playerBaseline{
			score:       si.Score,
			goals:       si.Goals,
			ownGoals:    si.OwnGoals,
			assists:     si.Assists,
			saves:       si.Saves,
			shots:       si.Shots,
			demolitions: si.Demolitions,
		}
	}
	return base
}

// goalTotalsByTeam sums per-player goal deltas by team for THIS match. The
// game's 'goals' stat already includes own goals in the scoring team's count,
// so no separate own-goals handling. Keyed by car index (stable within a match)
// so duplicate player names cannot collide in the baseline.
func goalTotalsByTeam(gp *flat.GamePacketT, baseline map[int]playerBaseline) [2]int {
	totals := [2]int{0, 0}
	for i, player := range gp.Players {
		if player == nil || player.ScoreInfo == nil {
			continue
		}
		goals := int(player.ScoreInfo.Goals) - int(baseline[i].goals)
		team := int(player.Team)
		if team == 0 || team == 1 {
			totals[team] += goals
		}
	}
	return totals
}

// buildResult converts the final packet into the result struct, as deltas from
// the baseline.
func buildResult(gp *flat.GamePacketT, baseline map[int]playerBaseline, mercyTriggered bool) TournamentMatchResult {
	final := goalTotalsByTeam(gp, baseline)
	teamScores := []TournamentTeamScore{
		{TeamIndex: 0, Score: final[0]},
		{TeamIndex: 1, Score: final[1]},
	}

	players := make([]TournamentMatchPlayerStat, 0, len(gp.Players))
	for i, player := range gp.Players {
		if player == nil || player.ScoreInfo == nil {
			continue
		}
		si := player.ScoreInfo
		base := baseline[i]
		players = append(players, TournamentMatchPlayerStat{
			Name:        player.Name,
			Team:        int(player.Team),
			IsBot:       player.IsBot,
			Score:       int(si.Score) - int(base.score),
			Goals:       int(si.Goals) - int(base.goals),
			OwnGoals:    int(si.OwnGoals) - int(base.ownGoals),
			Assists:     int(si.Assists) - int(base.assists),
			Saves:       int(si.Saves) - int(base.saves),
			Shots:       int(si.Shots) - int(base.shots),
			Demolitions: int(si.Demolitions) - int(base.demolitions),
		})
	}

	res := TournamentMatchResult{
		TeamScores:     teamScores,
		Players:        players,
		MercyTriggered: mercyTriggered,
	}
	if gp.MatchInfo != nil {
		res.DurationSeconds = int(gp.MatchInfo.SecondsElapsed)
		res.HasDuration = true
		res.IsOvertime = gp.MatchInfo.IsOvertime
	}
	return res
}
