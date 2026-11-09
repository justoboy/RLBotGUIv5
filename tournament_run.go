package main

// Tournament orchestration (bound Wails methods), ported from the v4 fork's
// tournament_runner.py (tournament_start_match, tournament_confirm_players_ready,
// tournament_record_result, launch_tournament_match auto-record path).
//
// v4 used eel.spawn to launch a match on a worker thread and auto-record the
// winner once the packet poll returned. v5 Wails methods are synchronous, so
// TournamentStartMatch blocks for the whole match and returns the updated state;
// the frontend awaits the promise. The per-player baseline delta logic in
// tournament_match.go is what makes the "Continue And Spawn" LAN workflow safe.

// matchHasHumans reports whether a match contains any human participant.
func matchHasHumans(match *TournamentMatch) bool {
	return countHumansInMatch(match) > 0
}

// countHumansInMatch counts human participants across both sides (team-aware).
func countHumansInMatch(match *TournamentMatch) int {
	count := 0
	for _, p := range []*TournamentParticipant{match.Participant1, match.Participant2} {
		if p != nil && p.ParticipantType == "human" {
			count++
		}
	}
	if match.Team1 != nil {
		for _, p := range match.Team1.Participants {
			if p.ParticipantType == "human" {
				count++
			}
		}
	}
	if match.Team2 != nil {
		for _, p := range match.Team2.Participants {
			if p.ParticipantType == "human" {
				count++
			}
		}
	}
	return count
}

// buildMatchSettings assembles the per-match launch settings from the live
// tournament state, mirroring v4's match_settings dict.
func (s *TournamentState) buildMatchSettings() TournamentMatchSettings {
	gameMode := s.GameMode
	if gameMode == "" {
		gameMode = "Soccar"
	}
	selectedMap := s.Map
	if s.RandomizeMap {
		selectedMap = pickRandomMap(gameMode)
	}
	if selectedMap == "" {
		selectedMap = "Stadium_P"
	}
	return TournamentMatchSettings{
		Map:             selectedMap,
		GameMode:        gameMode,
		Scripts:         s.Scripts,
		MutatorSettings: s.Mutators,
		ExtraOptions:    s.ExtraOptions,
		Launcher:        s.Launcher,
		LauncherArg:     s.LauncherArg,
		MercyRule:       s.MercyRule,
	}
}

// TournamentStartMatch launches a specific match, waits for it to finish, and
// auto-records the winner. When the match has humans and useStaging is true, it
// does NOT launch; it returns the state with a guidance message so the operator
// can host via Rocket Plugin and then call TournamentStartMatch again with the
// Continue And Spawn behavior already set on the state.
func (a *App) TournamentStartMatch(matchID string, useStaging bool) (*TournamentState, error) {
	a.tournamentMu.Lock()
	s := a.tournament
	a.tournamentMu.Unlock()

	if s == nil {
		return nil, errorf("no tournament loaded")
	}
	match := s.findMatchByIDInState(matchID)
	if match == nil {
		return nil, errorf("match %s not found", matchID)
	}
	if match.Completed {
		return nil, errorf("match %s already completed", matchID)
	}
	if match.Team1 == nil || match.Team2 == nil {
		if match.Participant1 == nil || match.Participant2 == nil {
			return nil, errorf("match %s has no participants", matchID)
		}
	}

	if matchHasHumans(match) && useStaging {
		// Staging: do not launch. The operator hosts via Rocket Plugin and
		// re-invokes with the Continue And Spawn behavior set on the state.
		return s, nil
	}

	blue, orange := buildMatchPlayers(match)
	settings := s.buildMatchSettings()

	// Clear any prior stop request so this match starts clean.
	a.tournamentMatchStopped.Store(false)

	// Blocking launch + capture (mirrors v4's eel.spawn worker thread).
	result := a.StartTournamentMatch(blue, orange, settings)
	if result.Error != "" {
		return s, errorf("%s", result.Error)
	}
	// Operator stopped the match before it finished: leave it incomplete so it can
	// be re-started (v4 parity: stop_match cleared the in-progress match state).
	if result.Stopped {
		return s, nil
	}

	// v5's protocol has no "end match" message, so when the mercy rule tripped we
	// must tear the live match down ourselves (v4 used set_game_state(end_match=True)).
	// StopMatch sends StopCommandT, which ends the in-game match. Only do this when
	// mercy triggered; a normally-ended match is already over.
	if result.MercyTriggered {
		a.StopMatch(false)
	}

	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	a.autoRecordResultLocked(match, result)
	a.persistLocked()
	return a.tournament, nil
}

// TournamentConfirmPlayersReady is the human LAN workflow entry point: it
// launches the real match (Continue And Spawn already set on the state) and
// records the result.
func (a *App) TournamentConfirmPlayersReady(matchID string) (*TournamentState, error) {
	return a.TournamentStartMatch(matchID, false)
}

// TournamentStopMatch aborts the currently-running tournament match (v4 parity:
// v4's stop_match button). It sets the stop flag so the blocking capture loop
// returns a Stopped result (no recording), then sends StopCommandT to tear down
// the live in-game match. The match stays incomplete so the operator can fix
// whatever went wrong and re-start it.
func (a *App) TournamentStopMatch() error {
	a.tournamentMatchStopped.Store(true)
	res := a.StopMatch(false)
	if !res.Success {
		return errorf("%s", res.Message)
	}
	return nil
}

// autoRecordResultLocked derives the winner from the captured team scores and
// records it. Caller holds the lock.
func (a *App) autoRecordResultLocked(match *TournamentMatch, res TournamentMatchResult) {
	if len(res.TeamScores) == 0 {
		return
	}
	s0, s1 := 0, 0
	for _, ts := range res.TeamScores {
		switch ts.TeamIndex {
		case 0:
			s0 = ts.Score
		case 1:
			s1 = ts.Score
		}
	}
	winningTeamIndex := 0
	if s1 > s0 {
		winningTeamIndex = 1
	}

	var winnerName string
	if match.Team1 != nil && match.Team2 != nil {
		if winningTeamIndex == 0 {
			winnerName = match.Team1.Name
		} else {
			winnerName = match.Team2.Name
		}
	} else {
		if winningTeamIndex == 0 && match.Participant1 != nil {
			winnerName = match.Participant1.Name
		} else if match.Participant2 != nil {
			winnerName = match.Participant2.Name
		}
	}
	if winnerName == "" {
		return
	}
	a.recordResultLocked(match, winnerName, []int{s0, s1}, res.Players, res.DurationSeconds, res.IsOvertime)
}

// TournamentRecordResult records a result manually (frontend override) and
// advances the bracket.
func (a *App) TournamentRecordResult(matchID, winnerName string, score []int, stats []TournamentMatchPlayerStat, durationSeconds int, isOvertime bool) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	match := a.tournament.findMatchByIDInState(matchID)
	if match == nil {
		return nil, errorf("match %s not found", matchID)
	}
	a.recordResultLocked(match, winnerName, score, stats, durationSeconds, isOvertime)
	a.persistLocked()
	return a.tournament, nil
}

// recordResultLocked records a completed match and advances the bracket. Caller
// holds the lock.
func (a *App) recordResultLocked(match *TournamentMatch, winnerName string, score []int, stats []TournamentMatchPlayerStat, durationSeconds int, isOvertime bool) {
	s := a.tournament

	var winner *TournamentParticipant
	if match.Participant1 != nil && match.Participant1.Name == winnerName {
		winner = match.Participant1
	} else if match.Participant2 != nil && match.Participant2.Name == winnerName {
		winner = match.Participant2
	}
	if winner == nil {
		return
	}

	match.Winner = winner
	match.Score = score
	match.Completed = true
	if len(stats) > 0 {
		match.PlayerStats = stats
	}
	match.DurationSeconds = durationSeconds
	match.HasDuration = true
	match.IsOvertime = isOvertime

	if s.Format == "double_elimination" && match.LoserNextMatchID != "" {
		s.populateLosersBracketMatch(match, winner)
	}

	if len(s.Teams) > 0 {
		var winningTeam, losingTeam *TournamentTeam
		for _, t := range s.Teams {
			if t.TeamID == winner.ParticipantID {
				winningTeam = t
				break
			}
		}
		if winningTeam != nil {
			match.WinnerTeam = winningTeam
			winningTeam.Wins++
			if match.Team1 != nil && match.Team2 != nil {
				if match.Team1.TeamID == winningTeam.TeamID {
					losingTeam = match.Team2
				} else {
					losingTeam = match.Team1
				}
				if losingTeam != nil {
					losingTeam.Losses++
				}
			}
		}
	}

	if s.Format == "swiss" {
		s.handleSwissProgression(match)
		return
	}
	if s.Format == "round_robin" {
		s.handleRoundRobinProgression()
		return
	}

	if isTournamentFinalMatch(match) {
		s.Winner = winner
		if match.WinnerTeam != nil {
			s.WinnerTeam = match.WinnerTeam
		}
		s.Completed = true
	} else {
		s.advanceWinner(match, winner)
	}
}

// TournamentUpdateSettings updates the live tournament's match settings (map,
// game mode, mutators, extra options, mercy rule, launcher) without regenerating
// the bracket.
func (a *App) TournamentUpdateSettings(cfg TournamentConfig) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament
	if cfg.Map != "" {
		s.Map = cfg.Map
	}
	if validGameModes[cfg.GameMode] {
		s.GameMode = cfg.GameMode
	}
	s.RandomizeMap = cfg.RandomizeMap
	s.Mutators = cfg.Mutators
	s.MercyRule = cfg.MercyRule
	a.persistLocked()
	return s, nil
}
