package main

import (
	"math/rand"
	"strings"
)

// Team formation and team-based bracket logic, ported from team_manager.py.

// validTeamSizes are the allowed participants-per-team values.
var validTeamSizes = []int{1, 2, 3, 4, 5}

// Team name pools (15 x 15 = 225 combinations).
var teamNameDescriptors = []string{
	"Blue", "Red", "Golden", "Silver", "Dark", "Raging", "Soaring", "Fierce",
	"Swift", "Iron", "Crimson", "Shadow", "Thunder", "Mystic", "Blazing",
}
var teamNameNouns = []string{
	"Eagles", "Ravens", "Wolves", "Tigers", "Dragons", "Knights", "Titans",
	"Phantoms", "Vipers", "Comets", "Storms", "Rockets", "Falcons", "Bears", "Serpents",
}

// teamName returns a default name like "Team A", "Team B", ...
func teamName(index int) string {
	if index < 26 {
		return "Team " + string(rune('A'+index))
	}
	return "Team " + itoa(index+1)
}

// validateTeamFormation checks participants can be split into full teams.
func validateTeamFormation(numParticipants, teamSize int, allowDuplicates bool) (bool, string) {
	if !containsInt(validTeamSizes, teamSize) {
		return false, "Team size must be one of 1, 2, 3, 4, or 5"
	}
	if allowDuplicates {
		if numParticipants < 2 {
			return false, "Need at least 2 participants (one per team) for duplicate mode"
		}
		return true, ""
	}
	if numParticipants < teamSize*2 {
		return false, "Need at least " + itoa(teamSize*2) + " participants (" + itoa(teamSize) + " per team x 2 teams)"
	}
	if numParticipants%teamSize != 0 {
		missing := teamSize - (numParticipants % teamSize)
		return false, "Participant count (" + itoa(numParticipants) + ") is not divisible by team size (" + itoa(teamSize) + "). Add " + itoa(missing) + " more participant(s)."
	}
	return true, ""
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// formTeamsRandom shuffles participants and chunks into teams.
func formTeamsRandom(participants []*TournamentParticipant, teamSize int, allowDuplicates bool) []*TournamentTeam {
	shuffled := make([]*TournamentParticipant, len(participants))
	copy(shuffled, participants)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	var teams []*TournamentTeam
	if allowDuplicates {
		teams = chunkIntoDuplicateTeams(shuffled, teamSize)
	} else {
		teams = chunkIntoTeams(shuffled, teamSize)
	}
	assignRandomTeamNames(teams)
	return teams
}

// formTeamsSeeded forms teams via a snake draft over seed order.
func formTeamsSeeded(participants []*TournamentParticipant, teamSize int, allowDuplicates bool) []*TournamentTeam {
	ordered := make([]*TournamentParticipant, len(participants))
	copy(ordered, participants)
	sortBySeed(ordered)

	if allowDuplicates {
		numTeams := len(ordered)
		teams := make([]*TournamentTeam, numTeams)
		for i := range teams {
			teams[i] = &TournamentTeam{TeamID: newTeamID(), Name: teamName(i)}
		}
		for i, p := range ordered {
			var dupes []*TournamentParticipant
			for range teamSize {
				dupes = append(dupes, cloneParticipant(p))
			}
			teams[i].Participants = dupes
		}
		assignRandomTeamNames(teams)
		return teams
	}

	numTeams := len(ordered) / teamSize
	teams := make([]*TournamentTeam, numTeams)
	for i := range teams {
		teams[i] = &TournamentTeam{TeamID: newTeamID(), Name: teamName(i)}
	}

	direction := 1
	index := 0
	for _, p := range ordered {
		var teamIdx int
		if direction == 1 {
			teamIdx = index % numTeams
		} else {
			teamIdx = numTeams - 1 - (index % numTeams)
		}
		teams[teamIdx].Participants = append(teams[teamIdx].Participants, p)
		index++
		if index%numTeams == 0 {
			direction *= -1
		}
	}
	assignRandomTeamNames(teams)
	return teams
}

func chunkIntoTeams(participants []*TournamentParticipant, teamSize int) []*TournamentTeam {
	var teams []*TournamentTeam
	for i := 0; i < len(participants); i += teamSize {
		end := i + teamSize
		if end > len(participants) {
			end = len(participants)
		}
		chunk := participants[i:end]
		teams = append(teams, &TournamentTeam{
			TeamID:       newTeamID(),
			Name:         teamName(i / teamSize),
			Participants: chunk,
		})
	}
	return teams
}

func chunkIntoDuplicateTeams(participants []*TournamentParticipant, teamSize int) []*TournamentTeam {
	var teams []*TournamentTeam
	for i, p := range participants {
		var dupes []*TournamentParticipant
		for range teamSize {
			dupes = append(dupes, cloneParticipant(p))
		}
		teams = append(teams, &TournamentTeam{
			TeamID:       newTeamID(),
			Name:         teamName(i),
			Participants: dupes,
		})
	}
	return teams
}

// generateTeamNames returns count unique random names, excluding given names.
func generateTeamNames(count int, excludeNames []string) ([]string, error) {
	if count < 0 {
		return nil, errNegativeCount
	}
	if count == 0 {
		return nil, nil
	}
	excluded := map[string]bool{}
	for _, n := range excludeNames {
		if strings.TrimSpace(n) != "" {
			excluded[strings.ToLower(strings.TrimSpace(n))] = true
		}
	}
	var pool []string
	for _, d := range teamNameDescriptors {
		for _, n := range teamNameNouns {
			name := d + " " + n
			if !excluded[strings.ToLower(name)] {
				pool = append(pool, name)
			}
		}
	}
	if len(pool) < count {
		return nil, errNotEnoughNames
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	return pool[:count], nil
}

// assignRandomTeamNames assigns a unique random name to every team in place.
func assignRandomTeamNames(teams []*TournamentTeam) {
	if len(teams) == 0 {
		return
	}
	var existing []string
	for _, t := range teams {
		if strings.TrimSpace(t.Name) != "" {
			existing = append(existing, t.Name)
		}
	}
	names, err := generateTeamNames(len(teams), existing)
	if err != nil {
		return
	}
	for i, team := range teams {
		if i < len(names) {
			team.Name = names[i]
		}
	}
}
