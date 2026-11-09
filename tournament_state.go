package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/RLBot/go-interface/flat"
)

// Tournament data model, ported from the v4 fork's tournament_state.py.
//
// The v4 fork added a full tournament system (single/double elimination,
// round robin, Swiss; team sizes 1v1..5v5; human participants; per-match
// result capture; match history; templates). This file mirrors that data
// model in Go. JSON field names intentionally match the v4 Python dicts so
// the frontend logic ports cleanly.
//
// Slices of pointers are used deliberately: a participant/team/match object
// is referenced from multiple places (a team appears in several matches, a
// participant is referenced by the match that produced it and the match it
// advances to). Sharing pointers reproduces v4's Python object-sharing
// semantics, so mutating one reference updates every place it is used.

// TournamentParticipant is a single entrant: a bot, a human, or a synthetic
// "team" stand-in used when team_size > 1.
type TournamentParticipant struct {
	Name            string    `json:"name"`
	ParticipantID   string    `json:"participant_id"`
	ParticipantType string    `json:"participant_type"` // "bot", "human", "team"
	// Player carries the underlying v5 player config so the match can be
	// launched with the same BotInfo/PsyonixBotInfo/HumanInfo the rest of the
	// GUI uses. Nil for synthetic "team" stand-ins.
	Player *PlayerJs `json:"player,omitempty"`
	Seed   int       `json:"seed"`
}

// TournamentTeam is a group of participants that competes as one side.
type TournamentTeam struct {
	TeamID       string                   `json:"team_id"`
	Name         string                   `json:"name"`
	Participants []*TournamentParticipant `json:"participants"`
	Wins         int                      `json:"wins"`
	Losses       int                      `json:"losses"`
	GoalsFor     int                      `json:"goals_for"`
	GoalsAgainst int                      `json:"goals_against"`
}

// TournamentMatchPlayerStat is one player's per-match stat line, captured as a
// delta from the baseline recorded at match start (so a stale scoreboard carried
// over via "Continue And Spawn" cannot pollute the recorded result).
type TournamentMatchPlayerStat struct {
	Name        string `json:"name"`
	Team        int    `json:"team"`
	IsBot       bool   `json:"is_bot"`
	Score       int    `json:"score"`
	Goals       int    `json:"goals"`
	OwnGoals    int    `json:"own_goals"`
	Assists     int    `json:"assists"`
	Saves       int    `json:"saves"`
	Shots       int    `json:"shots"`
	Demolitions int    `json:"demolitions"`
}

// TournamentMatch is a single bracket match.
type TournamentMatch struct {
	MatchID          string                       `json:"match_id"`
	RoundNum         int                          `json:"round_num"`
	Participant1     *TournamentParticipant       `json:"participant1"`
	Participant2     *TournamentParticipant       `json:"participant2"`
	Winner           *TournamentParticipant       `json:"winner"`
	Score            []int                        `json:"score"` // [team0, team1] or nil
	Completed        bool                         `json:"completed"`
	NextMatchID      string                       `json:"next_match_id"`       // "" = none
	LoserNextMatchID string                       `json:"loser_next_match_id"` // "" = none
	Team1            *TournamentTeam              `json:"team1"`
	Team2            *TournamentTeam              `json:"team2"`
	WinnerTeam       *TournamentTeam              `json:"winner_team"`
	PlayerStats      []TournamentMatchPlayerStat  `json:"player_stats"`
	DurationSeconds  int                          `json:"duration_seconds"`
	HasDuration      bool                         `json:"has_duration"`
	IsOvertime       bool                         `json:"is_overtime"`
}

// TournamentState is the complete state of a tournament.
type TournamentState struct {
	Name                  string                   `json:"name"`
	TournamentID          string                   `json:"tournament_id"`
	Format                string                   `json:"format"` // single_elimination, double_elimination, round_robin, swiss
	Participants          []*TournamentParticipant `json:"participants"`
	Matches               []*TournamentMatch       `json:"matches"`
	CurrentRound          int                      `json:"current_round"`
	Completed             bool                     `json:"completed"`
	Winner                *TournamentParticipant   `json:"winner"`
	CreatedAt             string                   `json:"created_at"`
	LosersBracketMatches  []*TournamentMatch       `json:"losers_bracket_matches"`
	TeamSize              int                      `json:"team_size"`
	Teams                 []*TournamentTeam        `json:"teams"`
	WinnerTeam            *TournamentTeam          `json:"winner_team"`
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
	Mutators              flat.MutatorSettingsT    `json:"mutators"`
	ExtraOptions          ExtraOptions             `json:"extra_options"`
}

// newID returns a short random hex id, matching v4's uuid4().hex[:8].
func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newTeamID returns a team id with the v4 "T" prefix.
func newTeamID() string {
	return "T" + newID()
}

// cloneParticipant deep-copies a participant (used when duplicating for party
// mode and when attaching teams to matches).
func cloneParticipant(p *TournamentParticipant) *TournamentParticipant {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// sideEntityID returns the entity id (participant id) occupying a match's side
// (0 = blue/participant1, 1 = orange/participant2). In team mode the side's
// participant is the team stand-in (whose ParticipantID is the team id); in 1v1
// it is the participant itself. This is the value-equality, name-independent
// way to attribute a per-player stat line (which carries a 0/1 team index) to
// the owning entity, immune to the in-game " (1)"/" (2)" name-suffix collisions.
func sideEntityID(m *TournamentMatch, team int) string {
	switch team {
	case 0:
		if m.Participant1 != nil {
			return m.Participant1.ParticipantID
		}
	case 1:
		if m.Participant2 != nil {
			return m.Participant2.ParticipantID
		}
	}
	return ""
}

// rawToPlayerJs wraps a raw JSON player payload into a PlayerJs.
func rawToPlayerJs(sort string, raw json.RawMessage) *PlayerJs {
	return &PlayerJs{Sort: sort, Player: raw}
}
