package main

import (
	"encoding/json"
	"testing"
)

// playAllCorrect plays every playable match in round order, always advancing
// participant1, until the bracket can't progress. Returns the ground-truth
// participant-id set per match id.
func playAllCorrect(t *testing.T, s *TournamentState) map[string][]string {
	t.Helper()
	for {
		progressed := false
		for _, m := range s.allMatches() {
			if !m.Completed && m.Participant1 != nil && m.Participant2 != nil {
				winner := m.Participant1
				loser := m.Participant2
				m.Completed = true
				m.Winner = winner
				if m.NextMatchID != "" {
					if n := s.findMatchByIDInState(m.NextMatchID); n != nil {
						if n.Participant1 == nil {
							n.Participant1 = winner
						} else if n.Participant2 == nil {
							n.Participant2 = winner
						}
					}
				}
				if m.LoserNextMatchID != "" {
					if n := s.findMatchByIDInState(m.LoserNextMatchID); n != nil {
						if n.Participant1 == nil {
							n.Participant1 = loser
						} else if n.Participant2 == nil {
							n.Participant2 = loser
						}
					}
				}
				progressed = true
				break
			}
		}
		if !progressed {
			break
		}
	}
	truth := map[string][]string{}
	for _, m := range s.allMatches() {
		truth[m.MatchID] = idSet(m.Participant1, m.Participant2)
	}
	return truth
}

// corruptMatch duplicates a single feeder's winner into both slots of matchID,
// mimicking the old pointer-identity bug.
func corruptMatch(s *TournamentState, matchID string) {
	for _, m := range s.allMatches() {
		if m.MatchID != matchID {
			continue
		}
		feeders := feedersOf(s.allMatches(), m.MatchID)
		if len(feeders) == 0 || feeders[0].Winner == nil {
			return
		}
		dup := feeders[0].Winner
		m.Participant1 = dup
		m.Participant2 = dup
		return
	}
}

func newTestDEBracket(t *testing.T, n int, id string) *TournamentState {
	t.Helper()
	parts := make([]*TournamentParticipant, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, &TournamentParticipant{
			Name: "Team" + itoa(i+1), ParticipantID: "P" + itoa(i+1),
			ParticipantType: "bot", Seed: i,
		})
	}
	wm, lm, gf, _ := generateDoubleEliminationBracket(parts)
	s := &TournamentState{
		Name: "t", TournamentID: id, Format: "double_elimination",
		Participants: parts, Matches: wm, LosersBracketMatches: lm, CurrentRound: 1,
	}
	if gf != nil {
		s.Matches = append(s.Matches, gf)
	}
	return s
}

// TestRepairBracketRestoresCorruption plays a bracket, corrupts the WB finals
// (W15) by duplicating a feeder winner into both slots, then repairs and asserts
// W15's participants are restored to the correct (distinct) set.
func TestRepairBracketRestoresCorruption(t *testing.T) {
	s := newTestDEBracket(t, 9, "R1")
	truth := playAllCorrect(t, s)

	// Sanity: ground truth for W15 must be two distinct teams.
	if len(truth["W15"]) != 2 || truth["W15"][0] == truth["W15"][1] {
		t.Fatalf("precondition: W15 truth not two distinct entrants: %v", truth["W15"])
	}

	corruptMatch(s, "W15")
	// Confirm it's now corrupted (both slots same id).
	for _, m := range s.allMatches() {
		if m.MatchID == "W15" {
			if !sameParticipant(m.Participant1, m.Participant2) {
				t.Fatalf("corruption did not take: W15 = %v", idSet(m.Participant1, m.Participant2))
			}
		}
	}

	app := &App{tournament: s, tournamentDir: t.TempDir()}
	if _, err := app.TournamentRepairBracket(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	for _, m := range s.allMatches() {
		if m.MatchID == "W15" {
			if !setsEqual(idSet(m.Participant1, m.Participant2), truth["W15"]) {
				t.Errorf("W15 not restored: got %v want %v",
					idSet(m.Participant1, m.Participant2), truth["W15"])
			}
		}
	}
}

// TestRepairBracketNoFalsePositives asserts repair leaves a healthy bracket
// unchanged (every match's set already equals its feeder-derived set).
func TestRepairBracketNoFalsePositives(t *testing.T) {
	s := newTestDEBracket(t, 7, "R3")
	before := map[string][]string{}
	for _, m := range s.allMatches() {
		before[m.MatchID] = idSet(m.Participant1, m.Participant2)
	}
	app := &App{tournament: s, tournamentDir: t.TempDir()}
	if _, err := app.TournamentRepairBracket(); err != nil {
		t.Fatalf("repair: %v", err)
	}
	for _, m := range s.allMatches() {
		if !setsEqual(idSet(m.Participant1, m.Participant2), before[m.MatchID]) {
			t.Errorf("healthy match %s changed: was %v now %v",
				m.MatchID, before[m.MatchID], idSet(m.Participant1, m.Participant2))
		}
	}
}

// TestRepairBracketByeAutoCompletes reproduces the user's reported gap: a
// losers-bracket BYE (single entrant) must auto-complete during repair and
// propagate its winner downstream, otherwise the LB stalls and the tournament
// can't continue. We build a 9-team bracket, play WB R1 so the LB feeders are
// decided, find an LB match that is a bye (one entrant), corrupt it by
// duplicating that entrant into both slots, then repair and assert the bye is
// auto-completed and its winner advanced to the next LB match.
func TestRepairBracketByeAutoCompletes(t *testing.T) {
	s := newTestDEBracket(t, 9, "R4")

	// Play WB round 1 only, so LB round-1 feeders are decided but LB matches are
	// still open. This is the state right before the LB byes need to resolve.
	for _, m := range s.Matches {
		if m.RoundNum == 1 && m.Participant1 != nil && m.Participant2 != nil {
			winner, loser := m.Participant1, m.Participant2
			m.Completed = true
			m.Winner = winner
			if n := s.findMatchByIDInState(m.NextMatchID); n != nil && n.Participant1 == nil {
				n.Participant1 = winner
			}
			if m.LoserNextMatchID != "" {
				if n := s.findMatchByIDInState(m.LoserNextMatchID); n != nil && n.Participant1 == nil {
					n.Participant1 = loser
				}
			}
		}
	}

	// Find an LB match that is currently a bye (exactly one entrant).
	var byeID string
	for _, m := range s.LosersBracketMatches {
		if (m.Participant1 != nil) != (m.Participant2 != nil) {
			byeID = m.MatchID
			break
		}
	}
	if byeID == "" {
		t.Skip("no LB bye in this bracket shape")
	}

	// Record the bye's single entrant, then corrupt it by duplicating into both
	// slots (the old bug's signature).
	var single *TournamentParticipant
	for _, m := range s.LosersBracketMatches {
		if m.MatchID == byeID {
			single = m.Participant1
			if single == nil {
				single = m.Participant2
			}
			m.Participant1 = single
			m.Participant2 = single
		}
	}

	app := &App{tournament: s, tournamentDir: t.TempDir()}
	if _, err := app.TournamentRepairBracket(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	// After repair the bye must be auto-completed with the single entrant as
	// winner, and that winner must have advanced into the next LB match.
	for _, m := range s.LosersBracketMatches {
		if m.MatchID == byeID {
			if !m.Completed || m.Winner == nil || !sameParticipant(m.Winner, single) {
				t.Fatalf("bye %s not auto-completed: completed=%v winner=%v", byeID, m.Completed, m.Winner)
			}
			if m.NextMatchID != "" {
				n := s.findMatchByIDInState(m.NextMatchID)
				if n == nil || !(sameParticipant(n.Participant1, single) || sameParticipant(n.Participant2, single)) {
					t.Fatalf("bye winner did not propagate into %s", m.NextMatchID)
				}
			}
		}
	}
}

// TestRepairBracketRoundTrip ensures a repaired state survives a save/load.
func TestRepairBracketRoundTrip(t *testing.T) {
	s := newTestDEBracket(t, 5, "R2")
	truth := playAllCorrect(t, s)
	corruptMatch(s, "W4") // a later WB match in a 5-team bracket

	app := &App{tournament: s, tournamentDir: t.TempDir()}
	if _, err := app.TournamentRepairBracket(); err != nil {
		t.Fatalf("repair: %v", err)
	}
	data, _ := json.Marshal(s)
	var re TournamentState
	if err := json.Unmarshal(data, &re); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range re.allMatches() {
		if m.MatchID == "W4" {
			if !setsEqual(idSet(m.Participant1, m.Participant2), truth["W4"]) {
				t.Errorf("W4 wrong after round-trip: got %v want %v",
					idSet(m.Participant1, m.Participant2), truth["W4"])
			}
		}
	}
}
