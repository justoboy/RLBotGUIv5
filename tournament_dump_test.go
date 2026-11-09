package main

import (
	"fmt"
	"testing"
)

// TestDumpNineTeamBracket is a diagnostic (not an assertion) that prints the v5
// Go 9-team double-elimination bracket in the SAME format as _probe_v4_bracket.py
// so the two implementations can be diffed by eye. Run with:
//   go test -run TestDumpNineTeamBracket -v
// It mirrors the v4 simulate: play each real match, participant1 wins.
func TestDumpNineTeamBracket(t *testing.T) {
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

	pn := func(p *TournamentParticipant) string {
		if p == nil {
			return "None"
		}
		return fmt.Sprintf("%q(id=%s,type=%s)", p.Name, p.ParticipantID, p.ParticipantType)
	}

	fmt.Println("=== V5 WINNERS (matches) ===")
	for _, m := range s.Matches {
		fmt.Printf("  %s r%d p1=%s p2=%s completed=%v winner=%s next=%q loser_next=%q\n",
			m.MatchID, m.RoundNum, pn(m.Participant1), pn(m.Participant2), m.Completed, pn(m.Winner), m.NextMatchID, m.LoserNextMatchID)
	}
	fmt.Println("=== V5 LOSERS BRACKET ===")
	for _, m := range s.LosersBracketMatches {
		fmt.Printf("  %s r%d p1=%s p2=%s completed=%v winner=%s next=%q loser_next=%q\n",
			m.MatchID, m.RoundNum, pn(m.Participant1), pn(m.Participant2), m.Completed, pn(m.Winner), m.NextMatchID, m.LoserNextMatchID)
	}

	app := &App{tournament: s, tournamentDir: t.TempDir()}
	simulateBracket(app)

	fmt.Println("=== V5 AFTER SIMULATION ===")
	fmt.Printf("completed: %v winner: %s\n", s.Completed, pn(s.Winner))
	fmt.Println("--- same-match duplicate check ---")
	for _, m := range allMatchesIn(s) {
		if m.Participant1 != nil && m.Participant2 != nil &&
			m.Participant1.ParticipantID == m.Participant2.ParticipantID {
			fmt.Printf("  !! %s: BOTH SIDES are %s\n", m.MatchID, pn(m.Participant1))
		}
	}
	fmt.Println("--- final structure ---")
	for _, m := range allMatchesIn(s) {
		fmt.Printf("  %s r%d p1=%s p2=%s completed=%v winner=%s\n",
			m.MatchID, m.RoundNum, pn(m.Participant1), pn(m.Participant2), m.Completed, pn(m.Winner))
	}
}
