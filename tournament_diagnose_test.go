package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestDiagnoseCurrentSave is a diagnostic (not an assertion). It loads the real
// current.json, prints every match's slots/winner/score/links, then runs the
// repair and prints again, so we can see exactly what the repair does wrong.
//
// Run with: go test -run TestDiagnoseCurrentSave -v
func TestDiagnoseCurrentSave(t *testing.T) {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "RLBot5", "tournaments")
	data, err := os.ReadFile(filepath.Join(dir, "current.json"))
	if err != nil {
		t.Skipf("no current.json to diagnose: %v", err)
	}
	var s TournamentState
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	fmt.Printf("=== FORMAT=%s TEAMSIZE=%d TEAMS=%d ===\n", s.Format, s.TeamSize, len(s.Teams))
	fmt.Printf("--- BEFORE REPAIR ---\n")
	dumpMatches(&s)

	app := &App{tournament: &s, tournamentDir: t.TempDir()}
	if _, err := app.TournamentRepairBracket(); err != nil {
		t.Fatalf("repair: %v", err)
	}
	fmt.Printf("--- AFTER REPAIR ---\n")
	dumpMatches(&s)
}

func pname(p *TournamentParticipant) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%s[%s]", p.Name, p.ParticipantID)
}

func tname(t *TournamentTeam) string {
	if t == nil {
		return "-"
	}
	return fmt.Sprintf("%s[%s]", t.Name, t.TeamID)
}

func dumpMatches(s *TournamentState) {
	for _, m := range s.allMatches() {
		feederDesc := ""
		for _, f := range s.allMatches() {
			if f.NextMatchID == m.MatchID {
				feederDesc += fmt.Sprintf(" [W:%s->%s]", pname(f.Winner), m.MatchID)
			}
			if f.LoserNextMatchID == m.MatchID {
				feederDesc += fmt.Sprintf(" [L:%s->%s]", pname(bracketLoser(f)), m.MatchID)
			}
		}
		fmt.Printf("%s r%d done=%v score=%v\n    p1=%s p2=%s win=%s\n    t1=%s t2=%s wteam=%s\n    next=%s lnext=%s%s\n",
			m.MatchID, m.RoundNum, m.Completed, m.Score,
			pname(m.Participant1), pname(m.Participant2), pname(m.Winner),
			tname(m.Team1), tname(m.Team2), tname(m.WinnerTeam),
			m.NextMatchID, m.LoserNextMatchID, feederDesc)
	}
}
