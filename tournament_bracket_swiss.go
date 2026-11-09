package main

// Swiss scoring and ranking, ported from bracket_generator.py.

// swissRecord is the per-entity aggregate used for ranking.
type swissRecord struct {
	Wins           int
	Losses         int
	Draws          int
	GoalsFor       int
	GoalsAgainst   int
	GoalDifference int
	Score          int
	Played         int
}

// computeEntityRecords aggregates per-entity records from completed matches.
// nameToEID maps a player name to the entity id that owns it (for team-mode
// score attribution).
func computeEntityRecords(
	entities []*TournamentParticipant,
	completedMatches []*TournamentMatch,
	nameToEID map[string]string,
) map[string]*swissRecord {
	records := make(map[string]*swissRecord, len(entities))
	for _, p := range entities {
		records[p.ParticipantID] = &swissRecord{}
	}

	for _, m := range completedMatches {
		if !m.Completed || len(m.Score) < 2 {
			continue
		}
		if m.Participant1 == nil || m.Participant2 == nil {
			continue
		}
		p1ID := m.Participant1.ParticipantID
		p2ID := m.Participant2.ParticipantID
		r1, ok1 := records[p1ID]
		r2, ok2 := records[p2ID]
		if !ok1 || !ok2 {
			continue
		}
		s1, s2 := m.Score[0], m.Score[1]

		r1.Played++
		r2.Played++
		r1.GoalsFor += s1
		r1.GoalsAgainst += s2
		r2.GoalsFor += s2
		r2.GoalsAgainst += s1

		switch {
		case s1 > s2:
			r1.Wins++
			r2.Losses++
		case s2 > s1:
			r2.Wins++
			r1.Losses++
		default:
			r1.Draws++
			r2.Draws++
		}

		for _, ps := range m.PlayerStats {
			// Attribute by team index, not name (in-game names carry " (1)" suffixes
			// when bots share a name, so name matching is unreliable).
			eid := sideEntityID(m, ps.Team)
			if eid == "" {
				continue
			}
			if r, ok := records[eid]; ok {
				r.Score += ps.Score
			}
		}
	}

	for _, r := range records {
		r.GoalDifference = r.GoalsFor - r.GoalsAgainst
	}
	return records
}

// headToHeadWinner returns the winner of the head-to-head between p1 and p2,
// or nil if they haven't played or it was a draw.
func headToHeadWinner(p1, p2 *TournamentParticipant, completedMatches []*TournamentMatch) *TournamentParticipant {
	for _, m := range completedMatches {
		if !m.Completed || len(m.Score) < 2 {
			continue
		}
		if m.Participant1 == nil || m.Participant2 == nil {
			continue
		}
		if (m.Participant1.ParticipantID == p1.ParticipantID && m.Participant2.ParticipantID == p2.ParticipantID) ||
			(m.Participant1.ParticipantID == p2.ParticipantID && m.Participant2.ParticipantID == p1.ParticipantID) {
			s1, s2 := m.Score[0], m.Score[1]
			if s1 > s2 {
				return m.Participant1
			} else if s2 > s1 {
				return m.Participant2
			}
			return nil
		}
	}
	return nil
}

// sortTiedGroup orders a group tied on overall wins by head-to-head wins,
// then goal difference, goals for, score.
func sortTiedGroup(group []*TournamentParticipant, records map[string]*swissRecord, completedMatches []*TournamentMatch) []*TournamentParticipant {
	if len(group) <= 1 {
		out := make([]*TournamentParticipant, len(group))
		copy(out, group)
		return out
	}

	groupIDs := make(map[string]bool, len(group))
	for _, p := range group {
		groupIDs[p.ParticipantID] = true
	}
	h2hWins := make(map[string]int, len(group))
	for _, p := range group {
		h2hWins[p.ParticipantID] = 0
	}
	for _, m := range completedMatches {
		if !m.Completed || m.Winner == nil {
			continue
		}
		if m.Participant1 == nil || m.Participant2 == nil {
			continue
		}
		if groupIDs[m.Participant1.ParticipantID] && groupIDs[m.Participant2.ParticipantID] {
			h2hWins[m.Winner.ParticipantID]++
		}
	}

	out := make([]*TournamentParticipant, len(group))
	copy(out, group)
	// Stable insertion sort by composite key (descending h2h, GD, GF, score).
	for i := 1; i < len(out); i++ {
		key := out[i]
		kr := records[key.ParticipantID]
		j := i - 1
		for j >= 0 {
			jr := records[out[j].ParticipantID]
			// Move out[j] right if key ranks before (is better than) out[j].
			if !lessTied(key, kr, h2hWins, out[j], jr) {
				break
			}
			out[j+1] = out[j]
			j--
		}
		out[j+1] = key
	}
	return out
}

// lessTied reports whether a should rank before b (a is "better").
func lessTied(a *TournamentParticipant, ar *swissRecord, ah map[string]int, b *TournamentParticipant, br *swissRecord) bool {
	if ar == nil || br == nil {
		return false
	}
	// Higher h2h wins first.
	if ah[a.ParticipantID] != ah[b.ParticipantID] {
		return ah[a.ParticipantID] > ah[b.ParticipantID]
	}
	if ar.GoalDifference != br.GoalDifference {
		return ar.GoalDifference > br.GoalDifference
	}
	if ar.GoalsFor != br.GoalsFor {
		return ar.GoalsFor > br.GoalsFor
	}
	return ar.Score > br.Score
}

// rankEntities ranks entities by wins (desc), ties broken by multi-way
// head-to-head, then goal difference, goals for, score.
func rankEntities(
	entities []*TournamentParticipant,
	completedMatches []*TournamentMatch,
	nameToEID map[string]string,
) ([]*TournamentParticipant, map[string]*swissRecord) {
	records := computeEntityRecords(entities, completedMatches, nameToEID)

	byWins := make(map[int][]*TournamentParticipant)
	var winOrder []int
	for _, e := range entities {
		w := records[e.ParticipantID].Wins
		if _, ok := byWins[w]; !ok {
			winOrder = append(winOrder, w)
		}
		byWins[w] = append(byWins[w], e)
	}

	// Sort win buckets descending.
	for i := 1; i < len(winOrder); i++ {
		for j := i; j > 0 && winOrder[j-1] < winOrder[j]; j-- {
			winOrder[j-1], winOrder[j] = winOrder[j], winOrder[j-1]
		}
	}

	var ordered []*TournamentParticipant
	for _, w := range winOrder {
		ordered = append(ordered, sortTiedGroup(byWins[w], records, completedMatches)...)
	}
	return ordered, records
}

// generateSwissNextRound generates the next round of Swiss matches based on
// current records, avoiding rematches when possible.
func generateSwissNextRound(
	participants []*TournamentParticipant,
	completedMatches []*TournamentMatch,
	roundNum int,
	nameToEID map[string]string,
) []*TournamentMatch {
	if len(participants) < 2 {
		return nil
	}

	sorted, _ := rankEntities(participants, completedMatches, nameToEID)

	previousOpponents := make(map[string]map[string]bool)
	for _, m := range completedMatches {
		if m.Participant1 == nil || m.Participant2 == nil {
			continue
		}
		p1 := m.Participant1.ParticipantID
		p2 := m.Participant2.ParticipantID
		if previousOpponents[p1] == nil {
			previousOpponents[p1] = map[string]bool{}
		}
		if previousOpponents[p2] == nil {
			previousOpponents[p2] = map[string]bool{}
		}
		previousOpponents[p1][p2] = true
		previousOpponents[p2][p1] = true
	}

	remaining := make([]*TournamentParticipant, len(sorted))
	copy(remaining, sorted)

	var matches []*TournamentMatch
	matchNum := 1
	for len(remaining) >= 2 {
		p1 := remaining[0]
		remaining = remaining[1:]
		p1Prev := previousOpponents[p1.ParticipantID]

		var opponent *TournamentParticipant
		for _, candidate := range remaining {
			if !p1Prev[candidate.ParticipantID] {
				opponent = candidate
				break
			}
		}
		if opponent == nil {
			opponent = remaining[0]
		}

		// Remove opponent from remaining.
		for i, c := range remaining {
			if c == opponent {
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}

		match := &TournamentMatch{
			MatchID:      "S" + itoa(roundNum) + "_" + itoa(matchNum),
			RoundNum:     roundNum,
			Participant1: p1,
			Participant2: opponent,
			Completed:    false,
		}
		matches = append(matches, match)
		matchNum++
	}
	return matches
}

// swissStanding is one row of the standings table.
type swissStanding struct {
	Participant  *TournamentParticipant `json:"participant"`
	Rank         int                      `json:"rank"`
	Wins         int                      `json:"wins"`
	Losses       int                      `json:"losses"`
	Draws        int                      `json:"draws"`
	GoalsFor     int                      `json:"goals_for"`
	GoalsAgainst int                      `json:"goals_against"`
	GoalDiff     int                      `json:"goal_difference"`
	Score        int                      `json:"score"`
	Played       int                      `json:"played"`
}

// calculateSwissStandings returns standings sorted by rank.
func calculateSwissStandings(
	participants []*TournamentParticipant,
	completedMatches []*TournamentMatch,
	nameToEID map[string]string,
) []swissStanding {
	sorted, records := rankEntities(participants, completedMatches, nameToEID)
	var standings []swissStanding
	for rank, p := range sorted {
		r := records[p.ParticipantID]
		if r == nil {
			r = &swissRecord{}
		}
		standings = append(standings, swissStanding{
			Participant:  p,
			Rank:         rank + 1,
			Wins:         r.Wins,
			Losses:       r.Losses,
			Draws:        r.Draws,
			GoalsFor:     r.GoalsFor,
			GoalsAgainst: r.GoalsAgainst,
			GoalDiff:     r.GoalDifference,
			Score:        r.Score,
			Played:       r.Played,
		})
	}
	return standings
}

// swissWinnerResult is the outcome of determineSwissWinner.
type swissWinnerResult struct {
	Winner              *TournamentParticipant `json:"winner"`
	PlayoffNeeded       bool                   `json:"playoff_needed"`
	PlayoffParticipants []*TournamentParticipant `json:"playoff_participants"`
	Standings           []swissStanding          `json:"standings"`
}

// determineSwissWinner determines the Swiss winner, or schedules a playoff if
// the top 2 are indistinguishable.
func determineSwissWinner(
	participants []*TournamentParticipant,
	completedMatches []*TournamentMatch,
	nameToEID map[string]string,
) swissWinnerResult {
	standings := calculateSwissStandings(participants, completedMatches, nameToEID)

	if len(standings) < 2 {
		var w *TournamentParticipant
		if len(standings) == 1 {
			w = standings[0].Participant
		}
		return swissWinnerResult{Winner: w, PlayoffNeeded: false, Standings: standings}
	}

	top := standings[0]
	second := standings[1]

	if top.Wins != second.Wins {
		return swissWinnerResult{Winner: top.Participant, PlayoffNeeded: false, Standings: standings}
	}

	p1 := top.Participant
	p2 := second.Participant
	if h2h := headToHeadWinner(p1, p2, completedMatches); h2h != nil {
		return swissWinnerResult{Winner: h2h, PlayoffNeeded: false, Standings: standings}
	}

	if top.GoalDiff != second.GoalDiff || top.GoalsFor != second.GoalsFor || top.Score != second.Score {
		return swissWinnerResult{Winner: top.Participant, PlayoffNeeded: false, Standings: standings}
	}

	return swissWinnerResult{Winner: nil, PlayoffNeeded: true, PlayoffParticipants: []*TournamentParticipant{p1, p2}, Standings: standings}
}
