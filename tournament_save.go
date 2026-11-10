package main

import "encoding/json"

// Tournament save codec.
//
// The live TournamentState is a graph of shared pointers: one participant/team
// object is referenced by every match that places it (the bracket generator and
// the advance logic both rely on that sharing). The DEFAULT Go marshalling of
// TournamentState deep-copies each participant into every match that references
// it, so a single bot's full config (including its base64 logo and display-only
// details) is written out once per reference. That is the bulk of the save-file
// bloat, and it is also what silently broke pointer sharing across a save/load
// round-trip (the "BP vs BP" corruption the repair tool exists to mop up).
//
// The slim wire format below fixes both problems:
//   - participants are serialized exactly ONCE (in the participants array);
//     matches and teams reference them by id, so the graph is reconstructed with
//     real shared pointers on load (no value-equality fallback needed);
//   - display-only bot data (base64 icon, details block, base64 logo) is stripped
//     on save, since launching a match only needs the run command / agent id /
//     root dir / loadout.
//
// decodeTournamentState still understands the OLD full format (where a match's
// participant1 is an object, not an id string) so existing saves keep loading.

// savedTournament is the on-disk wire form of TournamentState.
type savedTournament struct {
	Name                  string                   `json:"name"`
	TournamentID          string                   `json:"tournament_id"`
	Format                string                   `json:"format"`
	Participants          []*TournamentParticipant `json:"participants"`
	Matches               []*savedMatch            `json:"matches"`
	LosersBracketMatches  []*savedMatch            `json:"losers_bracket_matches"`
	CurrentRound          int                      `json:"current_round"`
	Completed             bool                     `json:"completed"`
	WinnerID              string                   `json:"winner"`
	CreatedAt             string                   `json:"created_at"`
	TeamSize              int                      `json:"team_size"`
	Teams                 []*savedTeam             `json:"teams"`
	WinnerTeamID          string                   `json:"winner_team"`
	AllowDuplicates       bool                     `json:"allow_duplicates"`
	SwissRounds           int                      `json:"swiss_rounds"`
	SwissPlayoffScheduled bool                     `json:"swiss_playoff_scheduled"`
	Map                   string                   `json:"map"`
	GameMode              string                   `json:"game_mode"`
	RandomizeMap          bool                     `json:"randomize_map"`
	Scripts               []BotInfo                `json:"scripts"`
	MercyRule             int                      `json:"mercy_rule"`
	Launcher              string                   `json:"launcher"`
	LauncherArg           string                   `json:"launcher_arg"`
	Mutators              flatMutator              `json:"mutators"`
	ExtraOptions          ExtraOptions             `json:"extra_options"`
}

// savedTeam is the wire form of TournamentTeam: members are referenced by id.
type savedTeam struct {
	TeamID         string   `json:"team_id"`
	Name           string   `json:"name"`
	ParticipantIDs []string `json:"participants"`
	Wins           int      `json:"wins"`
	Losses         int      `json:"losses"`
	GoalsFor       int      `json:"goals_for"`
	GoalsAgainst   int      `json:"goals_against"`
}

// savedMatch is the wire form of TournamentMatch: participants/teams are
// referenced by id string instead of embedded by value.
type savedMatch struct {
	MatchID          string                      `json:"match_id"`
	RoundNum         int                         `json:"round_num"`
	Participant1ID   string                      `json:"participant1,omitempty"`
	Participant2ID   string                      `json:"participant2,omitempty"`
	WinnerID         string                      `json:"winner,omitempty"`
	Score            []int                       `json:"score,omitempty"`
	Completed        bool                        `json:"completed"`
	NextMatchID      string                      `json:"next_match_id,omitempty"`
	LoserNextMatchID string                      `json:"loser_next_match_id,omitempty"`
	Team1ID          string                      `json:"team1,omitempty"`
	Team2ID          string                      `json:"team2,omitempty"`
	WinnerTeamID     string                      `json:"winner_team,omitempty"`
	PlayerStats      []TournamentMatchPlayerStat `json:"player_stats,omitempty"`
	DurationSeconds  int                         `json:"duration_seconds,omitempty"`
	HasDuration      bool                        `json:"has_duration,omitempty"`
	IsOvertime       bool                        `json:"is_overtime,omitempty"`
}

// toSaved converts the live state into the slim wire form, stripping display-only
// bot data from every participant and script.
func (s *TournamentState) toSaved() savedTournament {
	sv := savedTournament{
		Name:                  s.Name,
		TournamentID:          s.TournamentID,
		Format:                s.Format,
		CurrentRound:          s.CurrentRound,
		Completed:             s.Completed,
		CreatedAt:             s.CreatedAt,
		TeamSize:              s.TeamSize,
		AllowDuplicates:       s.AllowDuplicates,
		SwissRounds:           s.SwissRounds,
		SwissPlayoffScheduled: s.SwissPlayoffScheduled,
		Map:                   s.Map,
		GameMode:              s.GameMode,
		RandomizeMap:          s.RandomizeMap,
		Scripts:               slimScripts(s.Scripts),
		MercyRule:             s.MercyRule,
		Launcher:              s.Launcher,
		LauncherArg:           s.LauncherArg,
		Mutators:              s.Mutators,
		ExtraOptions:          s.ExtraOptions,
	}
	for _, p := range s.Participants {
		sv.Participants = append(sv.Participants, slimParticipant(p))
	}
	for _, m := range s.Matches {
		sv.Matches = append(sv.Matches, m.toSaved())
	}
	for _, m := range s.LosersBracketMatches {
		sv.LosersBracketMatches = append(sv.LosersBracketMatches, m.toSaved())
	}
	for _, t := range s.Teams {
		st := savedTeam{
			TeamID:       t.TeamID,
			Name:         t.Name,
			Wins:         t.Wins,
			Losses:       t.Losses,
			GoalsFor:     t.GoalsFor,
			GoalsAgainst: t.GoalsAgainst,
		}
		for _, mp := range t.Participants {
			st.ParticipantIDs = append(st.ParticipantIDs, mp.ParticipantID)
		}
		sv.Teams = append(sv.Teams, &st)
	}
	if s.Winner != nil {
		sv.WinnerID = s.Winner.ParticipantID
	}
	if s.WinnerTeam != nil {
		sv.WinnerTeamID = s.WinnerTeam.TeamID
	}
	return sv
}

// toSaved converts a live match into its wire form.
func (m *TournamentMatch) toSaved() *savedMatch {
	sm := &savedMatch{
		MatchID:          m.MatchID,
		RoundNum:         m.RoundNum,
		Score:            m.Score,
		Completed:        m.Completed,
		NextMatchID:      m.NextMatchID,
		LoserNextMatchID: m.LoserNextMatchID,
		PlayerStats:      m.PlayerStats,
		DurationSeconds:  m.DurationSeconds,
		HasDuration:      m.HasDuration,
		IsOvertime:       m.IsOvertime,
	}
	if m.Participant1 != nil {
		sm.Participant1ID = m.Participant1.ParticipantID
	}
	if m.Participant2 != nil {
		sm.Participant2ID = m.Participant2.ParticipantID
	}
	if m.Winner != nil {
		sm.WinnerID = m.Winner.ParticipantID
	}
	if m.Team1 != nil {
		sm.Team1ID = m.Team1.TeamID
	}
	if m.Team2 != nil {
		sm.Team2ID = m.Team2.TeamID
	}
	if m.WinnerTeam != nil {
		sm.WinnerTeamID = m.WinnerTeam.TeamID
	}
	return sm
}

// toState reconstructs the live state from the slim wire form, rebuilding the
// shared-pointer graph: every reference to a participant/team points at the SAME
// object, exactly like a freshly-generated bracket.
func (sv savedTournament) toState() *TournamentState {
	partByID := make(map[string]*TournamentParticipant, len(sv.Participants))
	for _, p := range sv.Participants {
		partByID[p.ParticipantID] = p
	}
	teamByID := make(map[string]*TournamentTeam, len(sv.Teams))
	standIns := make(map[string]*TournamentParticipant, len(sv.Teams))
	teams := make([]*TournamentTeam, 0, len(sv.Teams))
	for _, st := range sv.Teams {
		t := &TournamentTeam{
			TeamID:       st.TeamID,
			Name:         st.Name,
			Wins:         st.Wins,
			Losses:       st.Losses,
			GoalsFor:     st.GoalsFor,
			GoalsAgainst: st.GoalsAgainst,
		}
		for _, pid := range st.ParticipantIDs {
			if p := partByID[pid]; p != nil {
				t.Participants = append(t.Participants, cloneParticipant(p))
			}
		}
		teams = append(teams, t)
		// The team's bracket stand-in (ParticipantID == TeamID) is reconstructed
		// here so every match referencing it shares the same object.
		standIns[t.TeamID] = &TournamentParticipant{
			Name:            t.Name,
			ParticipantID:   t.TeamID,
			ParticipantType: "team",
			Seed:            0,
		}
	}
	resolve := func(id string) *TournamentParticipant {
		if id == "" {
			return nil
		}
		if p, ok := partByID[id]; ok {
			return p
		}
		return standIns[id]
	}
	resolveTeam := func(id string) *TournamentTeam {
		if id == "" {
			return nil
		}
		return teamByID[id]
	}
	build := func(sm *savedMatch) *TournamentMatch {
		return &TournamentMatch{
			MatchID:          sm.MatchID,
			RoundNum:         sm.RoundNum,
			Participant1:     resolve(sm.Participant1ID),
			Participant2:     resolve(sm.Participant2ID),
			Winner:           resolve(sm.WinnerID),
			Score:            sm.Score,
			Completed:        sm.Completed,
			NextMatchID:      sm.NextMatchID,
			LoserNextMatchID: sm.LoserNextMatchID,
			Team1:            resolveTeam(sm.Team1ID),
			Team2:            resolveTeam(sm.Team2ID),
			WinnerTeam:       resolveTeam(sm.WinnerTeamID),
			PlayerStats:      sm.PlayerStats,
			DurationSeconds:  sm.DurationSeconds,
			HasDuration:      sm.HasDuration,
			IsOvertime:       sm.IsOvertime,
		}
	}
	s := &TournamentState{
		Name:                  sv.Name,
		TournamentID:          sv.TournamentID,
		Format:                sv.Format,
		Participants:          sv.Participants,
		CurrentRound:          sv.CurrentRound,
		Completed:             sv.Completed,
		CreatedAt:             sv.CreatedAt,
		TeamSize:              sv.TeamSize,
		Teams:                 teams,
		AllowDuplicates:       sv.AllowDuplicates,
		SwissRounds:           sv.SwissRounds,
		SwissPlayoffScheduled: sv.SwissPlayoffScheduled,
		Map:                   sv.Map,
		GameMode:              sv.GameMode,
		RandomizeMap:          sv.RandomizeMap,
		Scripts:               sv.Scripts,
		MercyRule:             sv.MercyRule,
		Launcher:              sv.Launcher,
		LauncherArg:           sv.LauncherArg,
		Mutators:              sv.Mutators,
		ExtraOptions:          sv.ExtraOptions,
	}
	for _, sm := range sv.Matches {
		s.Matches = append(s.Matches, build(sm))
	}
	for _, sm := range sv.LosersBracketMatches {
		s.LosersBracketMatches = append(s.LosersBracketMatches, build(sm))
	}
	s.Winner = resolve(sv.WinnerID)
	s.WinnerTeam = resolveTeam(sv.WinnerTeamID)
	return s
}

// encodeTournamentState serializes a live state into the slim wire format.
func encodeTournamentState(s *TournamentState) ([]byte, error) {
	return json.Marshal(s.toSaved())
}

// decodeTournamentState parses a save file. It first tries the slim wire format;
// if that fails (e.g. an OLD save where a match's participant1 is an embedded
// object rather than an id string), it falls back to the legacy full format.
func decodeTournamentState(data []byte) (*TournamentState, error) {
	var sv savedTournament
	if err := json.Unmarshal(data, &sv); err == nil && sv.TournamentID != "" {
		return sv.toState(), nil
	}
	var full TournamentState
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, err
	}
	return &full, nil
}

// slimParticipant strips display-only data from a participant's player config.
func slimParticipant(p *TournamentParticipant) *TournamentParticipant {
	return &TournamentParticipant{
		Name:            p.Name,
		ParticipantID:   p.ParticipantID,
		ParticipantType: p.ParticipantType,
		Player:          slimPlayer(p.Player),
		Seed:            p.Seed,
	}
}

// slimPlayer removes the base64 icon / details / logo from an rlbot player's
// BotInfo. Other sorts (psyonix/human) carry no such data and pass through.
func slimPlayer(p *PlayerJs) *PlayerJs {
	if p == nil || p.Sort != "rlbot" {
		return p
	}
	var info BotInfo
	if err := json.Unmarshal(p.Player, &info); err != nil {
		return p
	}
	info.Icon = ""
	info.Config.Details = BotDetails{}
	info.Config.Settings.LogoFile = ""
	raw, err := json.Marshal(info)
	if err != nil {
		return p
	}
	return &PlayerJs{Sort: p.Sort, Player: raw}
}

// slimScripts strips display-only data from every script config.
func slimScripts(scripts []BotInfo) []BotInfo {
	if len(scripts) == 0 {
		return nil
	}
	out := make([]BotInfo, 0, len(scripts))
	for _, b := range scripts {
		b.Icon = ""
		b.Config.Details = BotDetails{}
		b.Config.Settings.LogoFile = ""
		out = append(out, b)
	}
	return out
}
