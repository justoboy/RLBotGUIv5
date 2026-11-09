package main

import "errors"

var errNegativeCount = errors.New("count must be >= 0")
var errNotEnoughNames = errors.New("cannot generate enough unique team names")

// generateTeamBracket builds a bracket between teams (stand-in participants).
func generateTeamBracket(teams []*TournamentTeam, tournamentFormat string) ([]*TournamentMatch, []*TournamentMatch, *TournamentMatch) {
	if len(teams) < 2 {
		return nil, nil, nil
	}

	var standIns []*TournamentParticipant
	for _, t := range teams {
		standIns = append(standIns, &TournamentParticipant{
			Name:            t.Name,
			ParticipantID:   t.TeamID,
			ParticipantType: "team",
			Seed:            0,
		})
	}

	var matches, losersMatches []*TournamentMatch
	var grandFinal *TournamentMatch

	switch tournamentFormat {
	case "single_elimination":
		matches, _ = generateSingleEliminationBracket(standIns)
	case "double_elimination":
		matches, losersMatches, grandFinal, _ = generateDoubleEliminationBracket(standIns)
	case "round_robin":
		matches = generateRoundRobinBracket(standIns)
	case "swiss":
		matches = generateSwissRound1(standIns)
	}

	teamByID := make(map[string]*TournamentTeam, len(teams))
	for _, t := range teams {
		teamByID[t.TeamID] = t
	}
	for _, m := range matches {
		attachTeams(m, teamByID)
	}
	for _, m := range losersMatches {
		attachTeams(m, teamByID)
	}
	if grandFinal != nil {
		attachTeams(grandFinal, teamByID)
	}
	return matches, losersMatches, grandFinal
}

// attachTeams sets match.team1/team2/winner_team from stand-in participant ids.
func attachTeams(match *TournamentMatch, teamByID map[string]*TournamentTeam) {
	if match.Participant1 != nil {
		if t, ok := teamByID[match.Participant1.ParticipantID]; ok {
			match.Team1 = t
		}
	}
	if match.Participant2 != nil {
		if t, ok := teamByID[match.Participant2.ParticipantID]; ok {
			match.Team2 = t
		}
	}
	if match.Winner != nil {
		if t, ok := teamByID[match.Winner.ParticipantID]; ok {
			match.WinnerTeam = t
		}
	}
}

// buildMatchPlayers builds blue/orange player lists for a (possibly team) match.
//
// Human participants are intentionally NOT added to the match config. RLBotCore
// rejects a config with more than one Human player ("Only one player can be of
// type Human"), and humans don't need to be in the config anyway: they join the
// live match themselves as spectators via the Rocket Plugin, and Rocket League
// assigns them a car. So we only spawn the bots, and RL/the plugin handle the
// humans. This is the v5 equivalent of v4's human-LAN staging flow.
func buildMatchPlayers(match *TournamentMatch) (blue, orange []PlayerJs) {
	add := func(p *TournamentParticipant, team int) {
		if p == nil || p.Player == nil {
			return
		}
		if p.ParticipantType == "human" {
			return // humans self-join as spectators; don't pass them to the core
		}
		if team == 0 {
			blue = append(blue, *p.Player)
		} else {
			orange = append(orange, *p.Player)
		}
	}
	if match.Team1 != nil && match.Team2 != nil {
		for _, p := range match.Team1.Participants {
			add(p, 0)
		}
		for _, p := range match.Team2.Participants {
			add(p, 1)
		}
	} else {
		add(match.Participant1, 0)
		add(match.Participant2, 1)
	}
	return blue, orange
}

// pairParticipants groups participants into connected components by pairings.
func pairParticipants(participants []*TournamentParticipant, pairings [][2]string, teamSize int) ([][]*TournamentParticipant, []string, []*TournamentParticipant) {
	var errorsList []string
	byID := make(map[string]*TournamentParticipant, len(participants))
	for _, p := range participants {
		byID[p.ParticipantID] = p
	}
	adj := make(map[string]map[string]bool)
	for _, pair := range pairings {
		pid1, pid2 := pair[0], pair[1]
		if _, ok := byID[pid1]; !ok {
			errorsList = append(errorsList, "Participant '"+pid1+"' not found in participant pool")
			continue
		}
		if _, ok := byID[pid2]; !ok {
			errorsList = append(errorsList, "Participant '"+pid2+"' not found in participant pool")
			continue
		}
		if adj[pid1] == nil {
			adj[pid1] = map[string]bool{}
		}
		if adj[pid2] == nil {
			adj[pid2] = map[string]bool{}
		}
		adj[pid1][pid2] = true
		adj[pid2][pid1] = true
	}

	visited := map[string]bool{}
	var components [][]string
	for _, p := range participants {
		pid := p.ParticipantID
		if !visited[pid] {
			if _, inAdj := adj[pid]; inAdj {
				comp := dfsComponent(pid, adj, visited)
				if len(comp) > teamSize {
					errorsList = append(errorsList, "Group of "+itoa(len(comp))+" participants exceeds team size ("+itoa(teamSize)+")")
				} else {
					components = append(components, comp)
				}
			}
		}
	}

	var assignments [][]*TournamentParticipant
	used := map[string]bool{}
	for _, comp := range components {
		var members []*TournamentParticipant
		for _, pid := range comp {
			members = append(members, byID[pid])
			used[pid] = true
		}
		assignments = append(assignments, members)
	}
	var remaining []*TournamentParticipant
	for _, p := range participants {
		if !used[p.ParticipantID] {
			remaining = append(remaining, p)
		}
	}
	return assignments, errorsList, remaining
}

func dfsComponent(start string, adj map[string]map[string]bool, visited map[string]bool) []string {
	stack := []string{start}
	var component []string
	for len(stack) > 0 {
		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[curr] {
			continue
		}
		visited[curr] = true
		component = append(component, curr)
		for neighbor := range adj[curr] {
			if !visited[neighbor] {
				stack = append(stack, neighbor)
			}
		}
	}
	return component
}

// formTeamsWithManualPairings forms teams from participants with pairings.
func formTeamsWithManualPairings(participants []*TournamentParticipant, pairings [][2]string, teamSize int, allowDuplicates bool) ([]*TournamentTeam, []string) {
	if allowDuplicates {
		return formTeamsRandom(participants, teamSize, true), nil
	}
	if len(participants)%teamSize != 0 {
		missing := teamSize - (len(participants) % teamSize)
		return nil, []string{"Participant count (" + itoa(len(participants)) + ") is not divisible by team size (" + itoa(teamSize) + "). Add " + itoa(missing) + " more participant(s)."}
	}
	assignments, errs, remaining := pairParticipants(participants, pairings, teamSize)
	if len(errs) > 0 {
		return nil, errs
	}
	for i := 0; i < len(remaining); i += teamSize {
		end := i + teamSize
		if end > len(remaining) {
			end = len(remaining)
		}
		chunk := remaining[i:end]
		if len(chunk) == teamSize {
			assignments = append(assignments, chunk)
		}
	}
	var teams []*TournamentTeam
	for i, members := range assignments {
		if len(members) != teamSize {
			errs = append(errs, "Team "+itoa(i+1)+" has "+itoa(len(members))+" members, expected "+itoa(teamSize))
			continue
		}
		teams = append(teams, &TournamentTeam{
			TeamID:       newTeamID(),
			Name:         teamName(i),
			Participants: members,
		})
	}
	if len(teams) > 0 {
		assignRandomTeamNames(teams)
	}
	return teams, errs
}
