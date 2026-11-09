package main

// Tournament statistics, match history, and Swiss standings (bound Wails
// methods), ported from the v4 fork's tournament_runner.py
// (get_tournament_stats, tournament_get_match_history,
// tournament_get_swiss_standings).
//
// v4 returned JSON strings; v5 returns typed structs that Wails marshals for
// the Svelte frontend. The per-player stat line (score/goals/assists/saves/
// shots/demolitions/own_goals) is captured per match in tournament_match.go and
// aggregated here onto the owning entity (a participant in 1v1, a team in team
// mode). Ranking uses the same multi-way head-to-head tiebreak as the bracket.

// TournamentEntityStat is one entity's (participant or team) aggregated stats.
type TournamentEntityStat struct {
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	MatchesPlayed  int     `json:"matches_played"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	Draws          int     `json:"draws"`
	GoalsFor       int     `json:"goals_for"`
	GoalsAgainst   int     `json:"goals_against"`
	GoalDifference int     `json:"goal_difference"`
	WinRate        float64 `json:"win_rate"`
	Score          int     `json:"score"`
	Assists        int     `json:"assists"`
	Saves          int     `json:"saves"`
	Shots          int     `json:"shots"`
	Demolitions    int     `json:"demolitions"`
	OwnGoals       int     `json:"own_goals"`
}

// TournamentStatsReport is the full statistics report for a tournament.
type TournamentStatsReport struct {
	TournamentName   string                 `json:"tournament_name"`
	Format           string                 `json:"format"`
	TeamSize         int                    `json:"team_size"`
	TotalMatches     int                    `json:"total_matches"`
	CompletedMatches int                    `json:"completed_matches"`
	TotalGoals       int                    `json:"total_goals"`
	Completed        bool                   `json:"completed"`
	Winner           string                 `json:"winner"`
	WinnerTeam       string                 `json:"winner_team"`
	Ranked           []TournamentEntityStat `json:"ranked"`
}

// TournamentHistoryEntry is one match in the match-history view.
type TournamentHistoryEntry struct {
	MatchID          string                      `json:"match_id"`
	RoundNum         int                         `json:"round_num"`
	Completed        bool                        `json:"completed"`
	IsBye            bool                        `json:"is_bye"`
	Team1Name        string                      `json:"team1_name"`
	Team2Name        string                      `json:"team2_name"`
	Participant1Name string                      `json:"participant1_name"`
	Participant2Name string                      `json:"participant2_name"`
	Score            []int                       `json:"score"`
	WinnerName       string                      `json:"winner_name"`
	PlayerStats      []TournamentMatchPlayerStat `json:"player_stats"`
	DurationSeconds  int                         `json:"duration_seconds"`
	HasDuration      bool                        `json:"has_duration"`
	IsOvertime       bool                        `json:"is_overtime"`
}

// TournamentHistoryRound is one round (bracket + round number) of history.
type TournamentHistoryRound struct {
	RoundNum int                        `json:"round_num"`
	Bracket  string                     `json:"bracket"`
	Matches  []TournamentHistoryEntry   `json:"matches"`
}

// TournamentHistoryReport is the full match-history report.
type TournamentHistoryReport struct {
	TournamentName   string                     `json:"tournament_name"`
	Format           string                     `json:"format"`
	TeamSize         int                        `json:"team_size"`
	TotalMatches     int                        `json:"total_matches"`
	CompletedMatches int                        `json:"completed_matches"`
	Completed        bool                       `json:"completed"`
	Winner           string                     `json:"winner"`
	WinnerTeam       string                     `json:"winner_team"`
	Rounds           []TournamentHistoryRound   `json:"rounds"`
}

// TournamentStandingsReport is the Swiss standings report.
type TournamentStandingsReport struct {
	Standings          []swissStanding `json:"standings"`
	PlayoffNeeded      bool            `json:"playoff_needed"`
	PlayoffParticipants []string       `json:"playoff_participants"`
	SwissRounds        int             `json:"swiss_rounds"`
	CurrentRound       int             `json:"current_round"`
	Completed          bool            `json:"completed"`
	Winner             string          `json:"winner"`
}

// TournamentPlayerStat is one individual player's aggregated stats across all
// completed matches. In team mode (team_size > 1) the main stats table aggregates
// by TEAM; this is the complementary per-player view the operator can toggle to.
type TournamentPlayerStat struct {
	Name        string `json:"name"`
	Team        string `json:"team"`
	IsBot       bool   `json:"is_bot"`
	MatchesPlayed int   `json:"matches_played"`
	Goals       int    `json:"goals"`
	OwnGoals    int    `json:"own_goals"`
	Assists     int    `json:"assists"`
	Saves       int    `json:"saves"`
	Shots       int    `json:"shots"`
	Demolitions int    `json:"demolitions"`
	Score       int    `json:"score"`
}

// TournamentGetPlayerStats aggregates per-PLAYER stats across all completed
// matches (keyed by player name). In team mode the main stats table aggregates by
// team; this exposes the individual stat lines underneath. In 1v1 it mirrors the
// per-entity table. Returns an empty slice when no match has recorded stats.
func (a *App) TournamentGetPlayerStats() ([]TournamentPlayerStat, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament

	// Map each player name to the team name they belong to (team mode) so the
	// per-player table can show which team a player scored for.
	nameToTeam := map[string]string{}
	if s.TeamSize > 1 && len(s.Teams) > 0 {
		for _, t := range s.Teams {
			for _, p := range t.Participants {
				nameToTeam[p.Name] = t.Name
			}
		}
	}

	agg := map[string]*TournamentPlayerStat{}
	order := []string{}
	for _, m := range s.allMatches() {
		if !m.Completed {
			continue
		}
		for _, ps := range m.PlayerStats {
			r, ok := agg[ps.Name]
			if !ok {
				r = &TournamentPlayerStat{Name: ps.Name, IsBot: ps.IsBot}
				if tn, ok := nameToTeam[ps.Name]; ok {
					r.Team = tn
				}
				agg[ps.Name] = r
				order = append(order, ps.Name)
			}
			r.MatchesPlayed++
			r.Goals += ps.Goals
			r.OwnGoals += ps.OwnGoals
			r.Assists += ps.Assists
			r.Saves += ps.Saves
			r.Shots += ps.Shots
			r.Demolitions += ps.Demolitions
			r.Score += ps.Score
		}
	}

	out := make([]TournamentPlayerStat, 0, len(order))
	for _, name := range order {
		out = append(out, *agg[name])
	}
	return out, nil
}

// TournamentGetStats aggregates per-entity stats across all completed matches.
// Mirrors v4's get_tournament_stats.
func (a *App) TournamentGetStats() (*TournamentStatsReport, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament

	isTeamMode := s.TeamSize > 1 && len(s.Teams) > 0

	entities := map[string]TournamentParticipant{}
	if isTeamMode {
		for _, t := range s.Teams {
			entities[t.TeamID] = TournamentParticipant{
				Name: t.Name, ParticipantID: t.TeamID, ParticipantType: "team",
			}
		}
	} else {
		for _, p := range s.Participants {
			entities[p.ParticipantID] = *p
		}
	}

	stats := map[string]*TournamentEntityStat{}
	for eid, info := range entities {
		stats[eid] = &TournamentEntityStat{Name: info.Name, Type: info.ParticipantType}
	}

	all := s.allMatches()
	totalGoals, completedMatches := 0, 0
	for _, m := range all {
		if !m.Completed || len(m.Score) < 2 {
			continue
		}
		if m.Participant1 == nil || m.Participant2 == nil {
			continue
		}
		completedMatches++
		p1, p2 := m.Participant1.ParticipantID, m.Participant2.ParticipantID
		s1, s2 := m.Score[0], m.Score[1]
		totalGoals += s1 + s2

		if r, ok := stats[p1]; ok {
			r.MatchesPlayed++
			r.GoalsFor += s1
			r.GoalsAgainst += s2
		}
		if r, ok := stats[p2]; ok {
			r.MatchesPlayed++
			r.GoalsFor += s2
			r.GoalsAgainst += s1
		}
		// Aggregate per-player stats onto the owning entity by TEAM INDEX, not by
		// name. In-game player names get " (1)", " (2)" suffixes when several bots
		// share a name, so name-based matching would drop or mis-attribute stats.
		// PlayerStats.Team already carries the 0/1 side, and each side maps to a
		// single entity (a team in team mode, a participant in 1v1).
		for _, ps := range m.PlayerStats {
			eid := sideEntityID(m, ps.Team)
			if eid == "" {
				continue
			}
			r, ok := stats[eid]
			if !ok {
				continue
			}
			r.Score += ps.Score
			r.Assists += ps.Assists
			r.Saves += ps.Saves
			r.Shots += ps.Shots
			r.Demolitions += ps.Demolitions
			r.OwnGoals += ps.OwnGoals
		}

		switch {
		case s1 > s2:
			if r, ok := stats[p1]; ok {
				r.Wins++
			}
			if r, ok := stats[p2]; ok {
				r.Losses++
			}
		case s2 > s1:
			if r, ok := stats[p2]; ok {
				r.Wins++
			}
			if r, ok := stats[p1]; ok {
				r.Losses++
			}
		default:
			if r, ok := stats[p1]; ok {
				r.Draws++
			}
			if r, ok := stats[p2]; ok {
				r.Draws++
			}
		}
	}

	for _, r := range stats {
		r.GoalDifference = r.GoalsFor - r.GoalsAgainst
		if r.MatchesPlayed > 0 {
			r.WinRate = float64(r.Wins) / float64(r.MatchesPlayed)
		}
	}

	// Rank entities using the same multi-way head-to-head tiebreak as the bracket.
	// (nameToEID is no longer needed for stat attribution — see sideEntityID above —
	// so nil is passed; computeEntityRecords now aggregates by team index.)
	var entityObjs []*TournamentParticipant
	for eid, info := range entities {
		entityObjs = append(entityObjs, &TournamentParticipant{
			Name: info.Name, ParticipantID: eid, ParticipantType: info.ParticipantType,
		})
	}
	ordered, _ := rankEntities(entityObjs, all, nil)
	ranked := make([]TournamentEntityStat, 0, len(ordered))
	for _, e := range ordered {
		if r, ok := stats[e.ParticipantID]; ok {
			ranked = append(ranked, *r)
		}
	}

	return &TournamentStatsReport{
		TournamentName:   s.Name,
		Format:           s.Format,
		TeamSize:         s.TeamSize,
		TotalMatches:     len(all),
		CompletedMatches: completedMatches,
		TotalGoals:       totalGoals,
		Completed:        s.Completed,
		Winner:           participantName(s.Winner),
		WinnerTeam:       teamNameOrEmpty(s.WinnerTeam),
		Ranked:           ranked,
	}, nil
}

// TournamentGetMatchHistory returns the full match history grouped by round.
// Mirrors v4's tournament_get_match_history.
func (a *App) TournamentGetMatchHistory() (*TournamentHistoryReport, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament

	all := s.allMatches()

	type group struct {
		bracket string
		round   int
		matches []*TournamentMatch
	}
	var groups []group
	if s.Format == "double_elimination" {
		wb := map[int][]*TournamentMatch{}
		lb := map[int][]*TournamentMatch{}
		for _, m := range all {
			if len(m.MatchID) > 0 && m.MatchID[0] == 'L' {
				lb[m.RoundNum] = append(lb[m.RoundNum], m)
			} else {
				wb[m.RoundNum] = append(wb[m.RoundNum], m)
			}
		}
		for _, rn := range sortedKeys(wb) {
			groups = append(groups, group{"winners", rn, wb[rn]})
		}
		for _, rn := range sortedKeys(lb) {
			groups = append(groups, group{"losers", rn, lb[rn]})
		}
	} else {
		rounds := map[int][]*TournamentMatch{}
		for _, m := range all {
			rounds[m.RoundNum] = append(rounds[m.RoundNum], m)
		}
		for _, rn := range sortedKeys(rounds) {
			groups = append(groups, group{"main", rn, rounds[rn]})
		}
	}

	completedMatches := 0
	roundsOut := make([]TournamentHistoryRound, 0, len(groups))
	for _, g := range groups {
		entries := make([]TournamentHistoryEntry, 0, len(g.matches))
		for _, m := range g.matches {
			side1 := m.Team1 != nil || m.Participant1 != nil
			side2 := m.Team2 != nil || m.Participant2 != nil
			isBye := side1 != side2
			winner := ""
			if m.Completed {
				winner = participantName(m.Winner)
				if m.WinnerTeam != nil {
					winner = m.WinnerTeam.Name
				}
				completedMatches++
			}
			entries = append(entries, TournamentHistoryEntry{
				MatchID:          m.MatchID,
				RoundNum:         m.RoundNum,
				Completed:        m.Completed,
				IsBye:            isBye,
				Team1Name:        sideName(m.Team1, m.Participant1),
				Team2Name:        sideName(m.Team2, m.Participant2),
				Participant1Name: participantName(m.Participant1),
				Participant2Name: participantName(m.Participant2),
				Score:            m.Score,
				WinnerName:       winner,
				PlayerStats:      m.PlayerStats,
				DurationSeconds:  m.DurationSeconds,
				HasDuration:      m.HasDuration,
				IsOvertime:       m.IsOvertime,
			})
		}
		roundsOut = append(roundsOut, TournamentHistoryRound{
			RoundNum: g.round, Bracket: g.bracket, Matches: entries,
		})
	}

	return &TournamentHistoryReport{
		TournamentName:   s.Name,
		Format:           s.Format,
		TeamSize:         s.TeamSize,
		TotalMatches:     len(all),
		CompletedMatches: completedMatches,
		Completed:        s.Completed,
		Winner:           participantName(s.Winner),
		WinnerTeam:       teamNameOrEmpty(s.WinnerTeam),
		Rounds:           roundsOut,
	}, nil
}

// TournamentGetSwissStandings returns live Swiss standings. Mirrors v4's
// tournament_get_swiss_standings.
func (a *App) TournamentGetSwissStandings() (*TournamentStandingsReport, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament
	if s.Format != "swiss" {
		return nil, errorf("tournament is not in Swiss format")
	}

	entities := s.swissEntities()
	var completed []*TournamentMatch
	for _, m := range s.Matches {
		if m.Completed {
			completed = append(completed, m)
		}
	}
	standings := calculateSwissStandings(entities, completed, s.swissNameToEID())

	allDone := s.CurrentRound > s.SwissRounds
	playoffNeeded := false
	var playoff []string
	if allDone && !s.Completed {
		res := determineSwissWinner(entities, completed, s.swissNameToEID())
		playoffNeeded = res.PlayoffNeeded
		for _, p := range res.PlayoffParticipants {
			playoff = append(playoff, p.Name)
		}
	}

	return &TournamentStandingsReport{
		Standings:           standings,
		PlayoffNeeded:       playoffNeeded,
		PlayoffParticipants: playoff,
		SwissRounds:         s.SwissRounds,
		CurrentRound:        s.CurrentRound,
		Completed:           s.Completed,
		Winner:              participantName(s.Winner),
	}, nil
}

// TournamentOpenRocketLeague launches Rocket League into a freeplay lobby so a
// human can host/join via the Rocket Plugin before a human match is started.
// Mirrors v4's open_rocket_league (which called connect_to_game). v5's protocol
// has no "launch game only" message, so we launch an empty freeplay match with
// the tournament's launcher, which brings the game up to a lobby.
func (a *App) TournamentOpenRocketLeague() error {
	a.tournamentMu.Lock()
	s := a.tournament
	a.tournamentMu.Unlock()
	if s == nil {
		return errorf("no tournament loaded")
	}
	settings := s.buildMatchSettings()
	settings.ExtraOptions.Freeplay = true
	match := a.buildTournamentMatchConfig(nil, nil, settings)
	return StartAndWaitForMatch(a.rlbotAddress, match)
}

// --- small helpers ---

func participantName(p *TournamentParticipant) string {
	if p == nil {
		return ""
	}
	return p.Name
}

func teamNameOrEmpty(t *TournamentTeam) string {
	if t == nil {
		return ""
	}
	return t.Name
}

// sideName returns the display name for one side of a match (team name in team
// mode, participant name in 1v1).
func sideName(t *TournamentTeam, p *TournamentParticipant) string {
	if t != nil {
		return t.Name
	}
	return participantName(p)
}

// sortedKeys returns the int keys of m in ascending order.
func sortedKeys(m map[int][]*TournamentMatch) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
