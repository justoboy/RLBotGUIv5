package main

import (
	"encoding/json"
	"testing"
)

// TestReloadMidTournamentNoDuplication reproduces the REAL user workflow that
// produced the corrupt "Wannabe SSL" save: play the early rounds, SAVE (marshal),
// RELOAD (unmarshal) into a fresh state, then play the later rounds. The earlier
// single-session reload test replays the whole bracket after one reload; this one
// splits play across the reload boundary, which is what actually happened.
//
// If the current (value-equality) code still corrupts W15 into "BP vs BP" here,
// the bug is LIVE in the reload path, not a stale artifact.
func TestReloadMidTournamentNoDuplication(t *testing.T) {
	n := 9
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
	app := &App{tournament: s, tournamentDir: t.TempDir()}

	// Phase 1: play every currently-playable match that is in WB rounds 1-2 and
	// the LB rounds they feed, i.e. everything except the WB finals. We stop once
	// W15 (the WB finals) is the only thing left to feed.
	playPlayableUpTo := func(roundMax int) {
		for {
			progressed := false
			for _, m := range allMatchesIn(app.tournament) {
				if !m.Completed && m.Participant1 != nil && m.Participant2 != nil && m.RoundNum <= roundMax {
					app.recordResultLocked(m, m.Participant1.Name, []int{1, 0}, nil, 0, false)
					progressed = true
					break
				}
			}
			if !progressed {
				return
			}
		}
	}

	playPlayableUpTo(3) // WB R1-R3 + LB feeders

	// Save + reload across the boundary.
	data, err := json.Marshal(app.tournament)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var reloaded TournamentState
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	app2 := &App{tournament: &reloaded, tournamentDir: t.TempDir()}

	// Phase 2: continue playing after the reload.
	for {
		progressed := false
		for _, m := range allMatchesIn(app2.tournament) {
			if !m.Completed && m.Participant1 != nil && m.Participant2 != nil {
				app2.recordResultLocked(m, m.Participant1.Name, []int{1, 0}, nil, 0, false)
				progressed = true
				break
			}
		}
		if !progressed || reloaded.Completed {
			break
		}
	}

	assertNoDuplication(t, &reloaded)
	if !reloaded.Completed || reloaded.Winner == nil {
		t.Fatalf("split-reload bracket did not complete cleanly (completed=%v winner=%v)", reloaded.Completed, reloaded.Winner)
	}
}
