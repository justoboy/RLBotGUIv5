package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// makeRLBotPlayer builds an rlbot PlayerJs whose BotInfo carries a base64 icon
// and a populated details block, i.e. the display-only data the slim codec is
// supposed to strip on save.
func makeRLBotPlayer(t *testing.T, name, agentID string) *PlayerJs {
	t.Helper()
	info := BotInfo{
		Config: BotConfig{
			Settings: BotSettings{Name: name, AgentId: agentID, RunCommand: "run.bat"},
			Details:  BotDetails{Description: "a long description", Developer: "dev", Language: "python", Tags: []string{"1v1"}},
		},
		Icon: "data:image/png;base64," + strings.Repeat("QUJDRA==", 64),
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal botinfo: %v", err)
	}
	return &PlayerJs{Sort: "rlbot", Player: raw}
}

// buildSlimTestState builds a single-elimination 1v1 state whose participants
// carry heavy display-only bot data, so we can measure the size reduction and
// verify it is stripped.
func buildSlimTestState(t *testing.T, n int) *TournamentState {
	t.Helper()
	parts := make([]*TournamentParticipant, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, &TournamentParticipant{
			Name:            "Bot" + itoa(i+1),
			ParticipantID:   "P" + itoa(i+1),
			ParticipantType: "bot",
			Player:          makeRLBotPlayer(t, "Bot"+itoa(i+1), "dev/bot"+itoa(i+1)),
			Seed:            i,
		})
	}
	matches, _ := generateSingleEliminationBracket(parts)
	return &TournamentState{
		Name:         "slim-test",
		TournamentID: "SLIM",
		Format:       "single_elimination",
		Participants: parts,
		Matches:      matches,
		CurrentRound: 1,
	}
}

// TestSlimSaveStripsDisplayData verifies the slim codec removes the base64 icon
// and the details block from every participant, and that the encoded payload is
// dramatically smaller than the legacy full marshal of the same state.
func TestSlimSaveStripsDisplayData(t *testing.T) {
	s := buildSlimTestState(t, 8)

	full, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("full marshal: %v", err)
	}
	slim, err := encodeTournamentState(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if len(slim) >= len(full) {
		t.Errorf("slim save (%d bytes) should be smaller than full (%d bytes)", len(slim), len(full))
	}
	if !bytes.Contains(slim, []byte("run.bat")) {
		t.Errorf("slim save must keep the run command needed to launch the bot")
	}
	if bytes.Contains(slim, []byte("data:image/png;base64")) {
		t.Errorf("slim save must not contain base64 icons")
	}
	if bytes.Contains(slim, []byte("a long description")) {
		t.Errorf("slim save must not contain the display-only description")
	}
}

// TestSlimRoundTripPreservesGraph encodes then decodes a state and confirms the
// reconstructed graph shares pointers (so bracket advancement still works) and
// still completes cleanly with a single winner.
func TestSlimRoundTripPreservesGraph(t *testing.T) {
	s := buildSlimTestState(t, 6)

	data, err := encodeTournamentState(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	reloaded, err := decodeTournamentState(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The decoded participants must have had their display data stripped.
	for _, p := range reloaded.Participants {
		if p.Player == nil || p.Player.Sort != "rlbot" {
			t.Fatalf("participant %s lost its player config", p.ParticipantID)
		}
		var info BotInfo
		if err := json.Unmarshal(p.Player.Player, &info); err != nil {
			t.Fatalf("decoded player not valid BotInfo: %v", err)
		}
		if info.Icon != "" {
			t.Errorf("decoded participant still carries an icon")
		}
		if info.Config.Details.Description != "" {
			t.Errorf("decoded participant still carries display details")
		}
		if info.Config.Settings.RunCommand != "run.bat" {
			t.Errorf("decoded participant lost the run command needed to launch")
		}
	}

	// Play the reloaded bracket to completion; pointer sharing must hold.
	app := &App{tournament: reloaded, tournamentDir: t.TempDir()}
	simulateBracket(app)
	assertNoDuplication(t, reloaded)
	if !reloaded.Completed || reloaded.Winner == nil {
		t.Fatalf("reloaded bracket did not complete cleanly (completed=%v winner=%v)", reloaded.Completed, reloaded.Winner)
	}
}

// TestSlimRoundTripTeamMode verifies the wire format reconstructs team stand-ins
// so every match referencing a team shares the same team object, and that a
// duplicate-mode team (same member cloned into several slots) round-trips.
func TestSlimRoundTripTeamMode(t *testing.T) {
	parts := []*TournamentParticipant{
		{Name: "A", ParticipantID: "PA", ParticipantType: "bot", Player: makeRLBotPlayer(t, "A", "dev/a")},
		{Name: "B", ParticipantID: "PB", ParticipantType: "bot", Player: makeRLBotPlayer(t, "B", "dev/b")},
		{Name: "C", ParticipantID: "PC", ParticipantType: "bot", Player: makeRLBotPlayer(t, "C", "dev/c")},
		{Name: "D", ParticipantID: "PD", ParticipantType: "bot", Player: makeRLBotPlayer(t, "D", "dev/d")},
	}
	teams := formTeamsRandom(parts, 2, false)
	matches, losers, gf := generateTeamBracket(teams, "single_elimination")
	if gf != nil {
		matches = append(matches, gf)
	}
	s := &TournamentState{
		Name: "team-test", TournamentID: "TEAM", Format: "single_elimination",
		Participants: parts, Teams: teams, Matches: matches, LosersBracketMatches: losers, TeamSize: 2,
	}

	data, err := encodeTournamentState(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	reloaded, err := decodeTournamentState(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(reloaded.Teams) != len(teams) {
		t.Fatalf("team count changed across round-trip")
	}
	// Each team must still have its members.
	for _, tm := range reloaded.Teams {
		if len(tm.Participants) != 2 {
			t.Errorf("team %s lost members on round-trip (got %d)", tm.TeamID, len(tm.Participants))
		}
	}
	app := &App{tournament: reloaded, tournamentDir: t.TempDir()}
	simulateBracket(app)
	assertNoDuplication(t, reloaded)
	if !reloaded.Completed || reloaded.WinnerTeam == nil {
		t.Fatalf("team bracket did not complete cleanly (completed=%v winnerTeam=%v)", reloaded.Completed, reloaded.WinnerTeam)
	}
}

// TestDecodeLegacyFormat ensures an OLD full-format save (match participants are
// embedded objects, not id strings) still loads via the fallback path.
func TestDecodeLegacyFormat(t *testing.T) {
	s := buildSlimTestState(t, 4)
	// Legacy format == plain json.Marshal of the live state.
	legacy, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("legacy marshal: %v", err)
	}
	got, err := decodeTournamentState(legacy)
	if err != nil {
		t.Fatalf("decode legacy: %v", err)
	}
	if got.TournamentID != "SLIM" || len(got.Participants) != 4 {
		t.Errorf("legacy decode produced wrong state (id=%s parts=%d)", got.TournamentID, len(got.Participants))
	}
}

// TestImportAssignsFreshIDAndRename verifies importing always mints a new id (so
// it never overwrites an existing saved tournament) and applies a new name.
func TestImportAssignsFreshIDAndRename(t *testing.T) {
	s := buildSlimTestState(t, 4)
	data, err := encodeTournamentState(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	origID := s.TournamentID

	dir := t.TempDir()
	app := &App{tournamentDir: dir}
	imported, err := app.TournamentImportJSON(string(data), "Renamed Cup")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.TournamentID == origID {
		t.Errorf("import must assign a fresh id (still %s)", origID)
	}
	if imported.TournamentID == "" {
		t.Errorf("imported tournament has no id")
	}
	if imported.Name != "Renamed Cup" {
		t.Errorf("import did not apply the new name (got %q)", imported.Name)
	}
	// A blank new name keeps the original name.
	imported2, err := app.TournamentImportJSON(string(data), "   ")
	if err != nil {
		t.Fatalf("import2: %v", err)
	}
	if imported2.Name != s.Name {
		t.Errorf("blank rename should keep original name (got %q want %q)", imported2.Name, s.Name)
	}
	if imported2.TournamentID == imported.TournamentID {
		t.Errorf("two imports must not share an id")
	}
}
