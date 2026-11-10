package main

// Tournament runner (bound Wails methods), ported from the v4 fork's
// tournament_runner.py. v4 exposed @eel.expose functions returning JSON
// strings; v5 binds typed Go methods directly to the Svelte frontend, so the
// JSON-string marshalling is gone and arguments/returns are real types.
//
// The current tournament is held on App.tournament (guarded by App.tournamentMu)
// and persisted to disk under App.tournamentDir, mirroring v4's
// QSettings("rlbotgui", "tournament_save") behavior.

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TournamentConfig is the creation-time configuration for a new tournament.
type TournamentConfig struct {
	Name            string                       `json:"name"`
	Format          string                       `json:"format"` // single_elimination, double_elimination, round_robin, swiss
	Participants    []TournamentParticipantInput `json:"participants"`
	TeamSize        int                          `json:"teamSize"`
	AllowDuplicates bool                         `json:"allowDuplicates"`
	SwissRounds     int                          `json:"swissRounds"` // 0 = auto ceil(log2(entities))
	Map             string                       `json:"map"`
	GameMode        string                       `json:"gameMode"`
	RandomizeMap    bool                         `json:"randomizeMap"`
	Scripts         []BotInfo                    `json:"scripts"`
	MercyRule       int                          `json:"mercyRule"`
	Mutators        flatMutator                  `json:"mutators"`
	ExtraOptions    ExtraOptions                 `json:"extraOptions"`
	Launcher        string                       `json:"launcher"`
	LauncherArg     string                       `json:"launcherArg"`
}

// TournamentParticipantInput is the frontend-facing participant payload. The
// Player field carries the underlying v5 player config (BotInfo/Psyonix/Human).
type TournamentParticipantInput struct {
	Name            string    `json:"name"`
	ParticipantID   string    `json:"participantId"`
	ParticipantType string    `json:"participantType"` // "bot", "human"
	Player          *PlayerJs `json:"player"`
	Seed            int       `json:"seed"`
}

// toParticipant converts a frontend participant payload into the internal type,
// generating an id when absent.
func (in TournamentParticipantInput) toParticipant() *TournamentParticipant {
	id := in.ParticipantID
	if id == "" {
		id = newID()
	}
	return &TournamentParticipant{
		Name:            in.Name,
		ParticipantID:   id,
		ParticipantType: in.ParticipantType,
		Player:          in.Player,
		Seed:            in.Seed,
	}
}

// TournamentNew creates a new tournament, forms teams (team mode), and
// generates the bracket. Mirrors v4's tournament_new.
func (a *App) TournamentNew(cfg TournamentConfig) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()

	if !containsInt(validTeamSizes, cfg.TeamSize) {
		return nil, errorf("team size must be one of 1, 2, 3, 4, or 5 (got %d)", cfg.TeamSize)
	}

	participants := make([]*TournamentParticipant, 0, len(cfg.Participants))
	for _, p := range cfg.Participants {
		participants = append(participants, p.toParticipant())
	}

	if cfg.TeamSize > 1 {
		if ok, msg := validateTeamFormation(len(participants), cfg.TeamSize, cfg.AllowDuplicates); !ok {
			return nil, errorf("%s", msg)
		}
	}

	if cfg.Format == "swiss" {
		numEntities := len(participants)
		if cfg.TeamSize > 1 {
			numEntities = len(participants) / cfg.TeamSize
		}
		if numEntities < 2 {
			return nil, errorf("swiss format requires at least 2 participants (or 2 teams)")
		}
		if numEntities%2 != 0 {
			return nil, errorf("swiss format requires an even number of participants/teams (got %d)", numEntities)
		}
	}

	gameMode := cfg.GameMode
	if !validGameModes[gameMode] {
		gameMode = "Soccar"
	}

	swissRounds := 0
	if cfg.Format == "swiss" {
		numEntities := len(participants)
		if cfg.TeamSize > 1 {
			numEntities = len(participants) / cfg.TeamSize
		}
		if cfg.SwissRounds > 0 {
			swissRounds = cfg.SwissRounds
		} else {
			swissRounds = calculateSwissRounds(numEntities)
		}
	}

	state := &TournamentState{
		Name:            cfg.Name,
		TournamentID:    newID(),
		Format:          cfg.Format,
		Participants:    participants,
		CurrentRound:    1,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		TeamSize:        cfg.TeamSize,
		AllowDuplicates: cfg.AllowDuplicates,
		SwissRounds:     swissRounds,
		Map:             cfg.Map,
		GameMode:        gameMode,
		RandomizeMap:    cfg.RandomizeMap,
		Scripts:         cfg.Scripts,
		MercyRule:       cfg.MercyRule,
		Mutators:        cfg.Mutators,
		ExtraOptions:    cfg.ExtraOptions,
		Launcher:        cfg.Launcher,
		LauncherArg:     cfg.LauncherArg,
	}

	if cfg.TeamSize > 1 {
		teams := formTeamsRandom(participants, cfg.TeamSize, cfg.AllowDuplicates)
		state.Teams = teams
		matches, losers, gf := generateTeamBracket(teams, cfg.Format)
		state.Matches = matches
		state.LosersBracketMatches = losers
		if gf != nil {
			state.Matches = append(state.Matches, gf)
		}
	} else {
		switch cfg.Format {
		case "single_elimination":
			matches, _ := generateSingleEliminationBracket(participants)
			state.Matches = matches
		case "double_elimination":
			wm, lm, gf, _ := generateDoubleEliminationBracket(participants)
			state.Matches = wm
			state.LosersBracketMatches = lm
			if gf != nil {
				state.Matches = append(state.Matches, gf)
			}
		case "round_robin":
			state.Matches = generateRoundRobinBracket(participants)
		case "swiss":
			state.Matches = generateSwissRound1(participants)
		}
	}

	a.tournament = state
	a.persistLocked()
	// New tournaments are automatically added to the saved library so they show
	// up on the landing page (mirrors v4's tournament_save_state adding to the
	// saved-tournaments list).
	_ = a.saveToLibraryLocked()
	return state, nil
}

// TournamentGetState returns the current tournament state (nil if none).
func (a *App) TournamentGetState() *TournamentState {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	return a.tournament
}

// TournamentDelete clears the current tournament and its save file. It also
// removes the saved-library entry (if any) so the tournament disappears from
// the landing page too — otherwise deleting from the bracket view would leave a
// stale saved/<id>.json that reappears on the next landing refresh.
func (a *App) TournamentDelete() error {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	id := ""
	if a.tournament != nil {
		id = a.tournament.TournamentID
	}
	a.tournament = nil
	if err := os.Remove(filepath.Join(a.tournamentDir, "current.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if id != "" {
		_ = os.Remove(filepath.Join(a.savedDir(), id+".json"))
	}
	return nil
}

// TournamentAddParticipant adds a participant to the current tournament.
func (a *App) TournamentAddParticipant(in TournamentParticipantInput) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	a.tournament.Participants = append(a.tournament.Participants, in.toParticipant())
	a.persistLocked()
	return a.tournament, nil
}

// TournamentRemoveParticipant removes a participant by id.
func (a *App) TournamentRemoveParticipant(participantID string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	filtered := a.tournament.Participants[:0]
	for _, p := range a.tournament.Participants {
		if p.ParticipantID != participantID {
			filtered = append(filtered, p)
		}
	}
	a.tournament.Participants = filtered
	a.persistLocked()
	return a.tournament, nil
}

// TournamentRandomizeSeeding shuffles participants and regenerates the bracket.
func (a *App) TournamentRandomizeSeeding() (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	shuffled := make([]*TournamentParticipant, len(a.tournament.Participants))
	copy(shuffled, a.tournament.Participants)
	randShuffleParticipants(shuffled)
	for i, p := range shuffled {
		p.Seed = i
	}
	a.tournament.Participants = shuffled
	a.regenerateBracketLocked()
	a.persistLocked()
	return a.tournament, nil
}

// TournamentSetParticipantSeed moves a participant to a new seed position.
func (a *App) TournamentSetParticipantSeed(participantID string, newSeed int) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	target := a.findParticipantLocked(participantID)
	if target == nil {
		return nil, errorf("participant %s not found", participantID)
	}
	a.removeParticipantLocked(target)
	if newSeed < 0 || newSeed > len(a.tournament.Participants) {
		return nil, errorf("seed out of range")
	}
	a.tournament.Participants = insertAt(a.tournament.Participants, newSeed, target)
	a.reassignSeedsLocked()
	a.persistLocked()
	return a.tournament, nil
}

// TournamentSwapSeeds swaps two participants' positions in the seeding order.
func (a *App) TournamentSwapSeeds(id1, id2 string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	i1, i2 := -1, -1
	for i, p := range a.tournament.Participants {
		if p.ParticipantID == id1 {
			i1 = i
		}
		if p.ParticipantID == id2 {
			i2 = i
		}
	}
	if i1 < 0 || i2 < 0 {
		return nil, errorf("participant not found")
	}
	a.tournament.Participants[i1], a.tournament.Participants[i2] = a.tournament.Participants[i2], a.tournament.Participants[i1]
	a.reassignSeedsLocked()
	a.persistLocked()
	return a.tournament, nil
}

// TournamentGetCurrentSeeding returns participants in current seed order.
func (a *App) TournamentGetCurrentSeeding() ([]*TournamentParticipant, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	return a.tournament.Participants, nil
}

// TournamentRenameTeam renames a team by index and updates the stand-in
// participant name in every match that references it. Mirrors v4's
// tournament_rename_team.
func (a *App) TournamentRenameTeam(teamIndex int, newName string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if teamIndex < 0 || teamIndex >= len(a.tournament.Teams) {
		return nil, errorf("team index %d out of range", teamIndex)
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errorf("team name cannot be empty")
	}
	for i, t := range a.tournament.Teams {
		if i != teamIndex && strings.EqualFold(strings.TrimSpace(t.Name), newName) {
			return nil, errorf("team name '%s' is already used by another team", newName)
		}
	}
	team := a.tournament.Teams[teamIndex]
	team.Name = newName
	for _, m := range a.tournament.allMatches() {
		if m.Team1 != nil && m.Team1.TeamID == team.TeamID && m.Participant1 != nil {
			m.Participant1.Name = newName
		}
		if m.Team2 != nil && m.Team2.TeamID == team.TeamID && m.Participant2 != nil {
			m.Participant2.Name = newName
		}
	}
	a.persistLocked()
	return a.tournament, nil
}

// TournamentFormTeamsWithPairings forms teams from manual pairings and
// regenerates the bracket. Mirrors v4's tournament_form_teams_with_pairings.
func (a *App) TournamentFormTeamsWithPairings(pairings [][2]string, teamNames []string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if a.tournament.TeamSize <= 1 {
		return nil, errorf("manual team pairing only applies when team size > 1")
	}
	teams, errs := formTeamsWithManualPairings(a.tournament.Participants, pairings, a.tournament.TeamSize, a.tournament.AllowDuplicates)
	if len(errs) > 0 {
		return nil, errorf("%s", strings.Join(errs, "; "))
	}
	for i, t := range teams {
		if i < len(teamNames) && strings.TrimSpace(teamNames[i]) != "" {
			t.Name = strings.TrimSpace(teamNames[i])
		}
	}
	a.tournament.Teams = teams
	matches, losers, gf := generateTeamBracket(teams, a.tournament.Format)
	a.tournament.Matches = matches
	a.tournament.LosersBracketMatches = losers
	if gf != nil {
		a.tournament.Matches = append(a.tournament.Matches, gf)
	}
	a.tournament.CurrentRound = 1
	a.tournament.Completed = false
	a.tournament.Winner = nil
	a.tournament.WinnerTeam = nil
	a.persistLocked()
	return a.tournament, nil
}

// TournamentRandomizeTeamNames re-randomizes every team's name from the pool
// (unique within the tournament) WITHOUT changing team membership or seeding,
// then refreshes the stand-in participant names in every match so the bracket
// reflects the new names. Mirrors v4's tournament_randomize_team_names.
func (a *App) TournamentRandomizeTeamNames() (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if len(a.tournament.Teams) == 0 {
		return nil, errorf("no teams to randomize")
	}
	assignRandomTeamNames(a.tournament.Teams)
	// Sync stand-in participant names so the bracket display updates.
	for _, m := range a.tournament.allMatches() {
		if m.Team1 != nil && m.Participant1 != nil {
			m.Participant1.Name = m.Team1.Name
		}
		if m.Team2 != nil && m.Participant2 != nil {
			m.Participant2.Name = m.Team2.Name
		}
	}
	a.persistLocked()
	return a.tournament, nil
}

// TournamentReformTeams re-shuffles participants into fresh random teams and
// regenerates the bracket (this DOES change membership, unlike randomize-names).
// Mirrors v4's tournament_form_teams (random).
func (a *App) TournamentReformTeams() (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if a.tournament.TeamSize <= 1 {
		return nil, errorf("team re-forming only applies when team size > 1")
	}
	teams := formTeamsRandom(a.tournament.Participants, a.tournament.TeamSize, a.tournament.AllowDuplicates)
	a.tournament.Teams = teams
	matches, losers, gf := generateTeamBracket(teams, a.tournament.Format)
	a.tournament.Matches = matches
	a.tournament.LosersBracketMatches = losers
	if gf != nil {
		a.tournament.Matches = append(a.tournament.Matches, gf)
	}
	a.tournament.CurrentRound = 1
	a.tournament.Completed = false
	a.tournament.Winner = nil
	a.tournament.WinnerTeam = nil
	a.persistLocked()
	return a.tournament, nil
}

// TournamentReorderTeamMember moves a member between slots within a team, which
// determines the in-game seat. Mirrors v4's tournament_reorder_team_member.
func (a *App) TournamentReorderTeamMember(teamIndex, fromSlot, toSlot int) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if teamIndex < 0 || teamIndex >= len(a.tournament.Teams) {
		return nil, errorf("team index %d out of range", teamIndex)
	}
	team := a.tournament.Teams[teamIndex]
	n := len(team.Participants)
	if fromSlot < 0 || fromSlot >= n || toSlot < 0 || toSlot >= n {
		return nil, errorf("slot index out of range (team has %d members)", n)
	}
	member := team.Participants[fromSlot]
	team.Participants = append(team.Participants[:fromSlot], team.Participants[fromSlot+1:]...)
	team.Participants = insertAt(team.Participants, toSlot, member)
	a.persistLocked()
	return a.tournament, nil
}

// --- helpers operating on the locked state ---

func (a *App) findParticipantLocked(id string) *TournamentParticipant {
	for _, p := range a.tournament.Participants {
		if p.ParticipantID == id {
			return p
		}
	}
	return nil
}

func (a *App) removeParticipantLocked(target *TournamentParticipant) {
	filtered := a.tournament.Participants[:0]
	for _, p := range a.tournament.Participants {
		if p != target {
			filtered = append(filtered, p)
		}
	}
	a.tournament.Participants = filtered
}

func (a *App) reassignSeedsLocked() {
	for i, p := range a.tournament.Participants {
		p.Seed = i
	}
}

// regenerateBracketLocked rebuilds matches from the current participants/teams.
func (a *App) regenerateBracketLocked() {
	s := a.tournament
	if s.TeamSize > 1 {
		teams := formTeamsRandom(s.Participants, s.TeamSize, s.AllowDuplicates)
		s.Teams = teams
		matches, losers, gf := generateTeamBracket(teams, s.Format)
		s.Matches = matches
		s.LosersBracketMatches = losers
		if gf != nil {
			s.Matches = append(s.Matches, gf)
		}
		return
	}
	switch s.Format {
	case "single_elimination":
		matches, _ := generateSingleEliminationBracket(s.Participants)
		s.Matches = matches
	case "double_elimination":
		wm, lm, gf, _ := generateDoubleEliminationBracket(s.Participants)
		s.Matches = wm
		s.LosersBracketMatches = lm
		if gf != nil {
			s.Matches = append(s.Matches, gf)
		}
	case "round_robin":
		s.Matches = generateRoundRobinBracket(s.Participants)
	case "swiss":
		s.Matches = generateSwissRound1(s.Participants)
	}
}

// --- persistence ---

// persistLocked writes the current tournament to disk (slim wire format). Caller
// holds the lock. If the tournament is already in the saved library, its library
// entry is updated too, so match progress survives a restart (mirrors v4's
// auto-sync into the saved-tournaments list).
func (a *App) persistLocked() {
	if a.tournament == nil {
		return
	}
	_ = os.MkdirAll(a.tournamentDir, 0o755)
	data, err := encodeTournamentState(a.tournament)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(a.tournamentDir, "current.json"), data, 0o644)

	// Keep the library entry in sync if this tournament is in the library.
	savedPath := filepath.Join(a.savedDir(), a.tournament.TournamentID+".json")
	if _, err := os.Stat(savedPath); err == nil {
		_ = os.WriteFile(savedPath, data, 0o644)
	}
}

// TournamentLoad restores the most recent saved tournament from disk.
func (a *App) TournamentLoad() (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	data, err := os.ReadFile(filepath.Join(a.tournamentDir, "current.json"))
	if err != nil {
		return nil, err
	}
	state, err := decodeTournamentState(data)
	if err != nil {
		return nil, err
	}
	a.tournament = state
	return state, nil
}
