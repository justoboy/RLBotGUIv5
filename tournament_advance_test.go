package main

import (
	"encoding/json"
	"testing"
)

// buildTestDoubleElim constructs an 8-team double-elimination state directly
// (bypassing TournamentNew) so the test does not depend on the Wails runtime.
func buildTestDoubleElim(n int) *TournamentState {
	parts := make([]*TournamentParticipant, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, &TournamentParticipant{
			Name:            "Team" + itoa(i+1),
			ParticipantID:   "P" + itoa(i+1),
			ParticipantType: "bot",
			Seed:            i + 1,
		})
	}
	wm, lm, gf, _ := generateDoubleEliminationBracket(parts)
	s := &TournamentState{
		Name:                 "test",
		TournamentID:         "TEST",
		Format:               "double_elimination",
		Participants:         parts,
		Matches:              wm,
		LosersBracketMatches: lm,
		CurrentRound:         1,
	}
	if gf != nil {
		s.Matches = append(s.Matches, gf)
	}
	return s
}

// allMatchesIn returns every match in both brackets.
func allMatchesIn(s *TournamentState) []*TournamentMatch {
	return append(append([]*TournamentMatch{}, s.Matches...), s.LosersBracketMatches...)
}

// simulateBracket plays the whole bracket: repeatedly find any incomplete match
// that has both participants and record participant1 as the winner, until the
// tournament completes or no progress can be made.
func simulateBracket(a *App) {
	for {
		progressed := false
		for _, m := range allMatchesIn(a.tournament) {
			if !m.Completed && m.Participant1 != nil && m.Participant2 != nil {
				a.recordResultLocked(m, m.Participant1.Name, []int{1, 0}, nil, 0, false)
				progressed = true
				break
			}
		}
		if !progressed {
			return
		}
		if a.tournament.Completed {
			return
		}
	}
}

// assertNoDuplication checks the invariants the pointer-identity bug violated:
// (1) no match has the same team on both sides, and (2) every team occupies at
// most one not-yet-completed match at a time. A re-advancement bug makes a team
// occupy two active matches at once.
func assertNoDuplication(t *testing.T, s *TournamentState) {
	t.Helper()
	activeCount := map[string]int{}
	for _, m := range allMatchesIn(s) {
		if m.Participant1 != nil && m.Participant2 != nil &&
			m.Participant1.ParticipantID == m.Participant2.ParticipantID {
			t.Errorf("match %s has the same team on both sides: %s", m.MatchID, m.Participant1.Name)
		}
		if m.Completed {
			continue
		}
		for _, p := range []*TournamentParticipant{m.Participant1, m.Participant2} {
			if p != nil {
				activeCount[p.ParticipantID]++
			}
		}
	}
	for id, c := range activeCount {
		if c > 1 {
			t.Errorf("team %s occupies %d active matches at once (should be at most 1)", id, c)
		}
	}
}

// TestAdvanceAfterReloadNoDuplication is the regression test for the
// pointer-identity bug. It plays the full bracket once in a single session (the
// reference), then plays an identical bracket after a save/load round-trip (which
// replaces the shared participant pointers with per-match deep copies sharing
// only the ParticipantID). Both must complete cleanly with exactly one winner and
// no team duplicated across the bracket.
func TestAdvanceAfterReloadNoDuplication(t *testing.T) {
	// 6 teams (not a power of two) so the bracket has real byes, which exercise
	// the bye-cascade path that the pointer-identity bug corrupts after a reload.
	// Reference run: single session, no reload.
	ref := buildTestDoubleElim(6)
	refApp := &App{tournament: ref, tournamentDir: t.TempDir()}
	simulateBracket(refApp)
	if !ref.Completed || ref.Winner == nil {
		t.Fatalf("reference bracket did not complete cleanly")
	}
	assertNoDuplication(t, ref)

	// Reload run: serialize then deserialize before playing. With pointer
	// equality the bye logic mis-detects already-placed winners and re-advances
	// them, duplicating teams across the bracket and corrupting the result.
	s := buildTestDoubleElim(6)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var reloaded TournamentState
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	a := &App{tournament: &reloaded, tournamentDir: t.TempDir()}
	simulateBracket(a)

	assertNoDuplication(t, &reloaded)
	if !reloaded.Completed || reloaded.Winner == nil {
		t.Fatalf("reloaded bracket did not complete cleanly (completed=%v winner=%v)", reloaded.Completed, reloaded.Winner)
	}
	if reloaded.Winner.ParticipantID != ref.Winner.ParticipantID {
		t.Errorf("winner mismatch after reload: got %s want %s", reloaded.Winner.ParticipantID, ref.Winner.ParticipantID)
	}
}

// TestHandleByeSituationAfterReload directly exercises the bye logic that the
// pointer-identity bug corrupts. A match `next` already contains participant X
// (as a deep copy with the same id but a different pointer than the completed
// feeder's winner). handleByeSituation must recognize X is already placed and NOT
// re-advance the feeder's winner into the empty slot. With pointer equality,
// findCompletedUnprocessedFeeder wrongly returns the feeder and X is duplicated.
func TestHandleByeSituationAfterReload(t *testing.T) {
	// A completed feeder whose winner is X.
	x := &TournamentParticipant{Name: "X", ParticipantID: "PX", ParticipantType: "bot"}
	feeder := &TournamentMatch{
		MatchID:      "F",
		Completed:    true,
		Winner:       x,
		NextMatchID:  "N",
		Participant1: x,
	}
	// The next match already has X in slot 1, but as a DIFFERENT pointer with the
	// same id (exactly what a save/load round-trip produces).
	xCopy := &TournamentParticipant{Name: "X", ParticipantID: "PX", ParticipantType: "bot"}
	next := &TournamentMatch{
		MatchID:      "N",
		Participant1: xCopy, // already placed (deep copy)
		Participant2: nil,   // empty slot
	}
	s := &TournamentState{
		Format:  "double_elimination",
		Matches: []*TournamentMatch{feeder, next},
	}

	s.handleByeSituation(next)

	// Because X is already in next (slot 1) and the only feeder is already
	// completed and processed, next must NOT be re-fed. Slot 2 must stay empty and
	// next must not be auto-completed by re-advancing X.
	if next.Participant2 != nil {
		t.Errorf("handleByeSituation wrongly re-advanced the feeder winner into slot 2 (duplication bug)")
	}
}

// TestNineTeamDoubleElimNoDuplication reproduces the "Wannabe SSL" scenario: a
// 9-team double-elimination bracket (non-power-of-two, so WB R1 is mostly byes).
// The saved save file showed several LB matches with the SAME team on both sides
// (e.g. "Silver Eagles vs Silver Eagles"). This test builds the same shape and
// simulates it, asserting no match ever ends up with one team on both sides and
// no team occupies two active matches at once.
func TestNineTeamDoubleElimNoDuplication(t *testing.T) {
	s := buildTestDoubleElim(9)
	app := &App{tournament: s, tournamentDir: t.TempDir()}
	simulateBracket(app)
	assertNoDuplication(t, s)
	if !s.Completed || s.Winner == nil {
		t.Fatalf("9-team bracket did not complete cleanly (completed=%v winner=%v)", s.Completed, s.Winner)
	}
}

// TestSameParticipantValueEquality locks in the value-equality helper.
func TestSameParticipantValueEquality(t *testing.T) {
	a := &TournamentParticipant{Name: "X", ParticipantID: "P1"}
	b := &TournamentParticipant{Name: "X", ParticipantID: "P1"} // distinct pointer, same id
	if !sameParticipant(a, b) {
		t.Errorf("sameParticipant should compare by id, not pointer")
	}
	c := &TournamentParticipant{Name: "Y", ParticipantID: "P2"}
	if sameParticipant(a, c) {
		t.Errorf("different ids must not be equal")
	}
	if !sameParticipant(nil, nil) {
		t.Errorf("two nils must be equal")
	}
	if sameParticipant(a, nil) {
		t.Errorf("value vs nil must not be equal")
	}
}
