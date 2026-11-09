package main

import "math"

// Bracket generation, ported from the v4 fork's bracket_generator.py.
//
// Pointer slices are used so a participant/team object is shared across the
// matches that reference it (matching v4's Python object-sharing semantics).

// seedBracket applies standard bracket seeding so top seeds don't meet until
// later rounds. nil slots represent byes.
func seedBracket(slots []*TournamentParticipant, bracketSize int) []*TournamentParticipant {
	if len(slots) == 0 {
		return slots
	}

	seeded := make([]*TournamentParticipant, bracketSize)

	var participants []*TournamentParticipant
	for _, s := range slots {
		if s != nil {
			participants = append(participants, s)
		}
	}
	if len(participants) == 0 {
		return slots
	}

	// Sort by seed (stable, ascending).
	sortBySeed(participants)

	positions := getSeedPositions(bracketSize)
	for i, pos := range positions {
		if i < len(participants) {
			seeded[pos] = participants[i]
		}
	}
	return seeded
}

// getSeedPositions returns bracket slot positions ordered by seed (position for
// seed 1 first, etc.).
func getSeedPositions(bracketSize int) []int {
	switch bracketSize {
	case 2:
		return []int{0, 1}
	case 4:
		return []int{0, 3, 1, 2}
	case 8:
		return []int{0, 7, 3, 4, 5, 2, 1, 6}
	case 16:
		return []int{0, 15, 7, 8, 11, 4, 3, 12, 13, 2, 6, 9, 10, 5, 1, 14}
	}

	positions := []int{0, 1}
	size := 2
	for size < bracketSize {
		var newPositions []int
		for _, pos := range positions {
			newPositions = append(newPositions, pos*2, pos*2+1)
		}
		positions = newPositions
		size *= 2
		if size <= bracketSize {
			positions = reorderSeeding(positions, size)
		}
	}
	if len(positions) > bracketSize {
		positions = positions[:bracketSize]
	}
	return positions
}

// reorderSeeding creates the standard bracket where 1 plays size, 2 plays
// size-1, etc.
func reorderSeeding(positions []int, size int) []int {
	if size == 2 {
		return []int{0, 1}
	}
	result := make([]int, 0, size)
	half := size / 2
	for i := 0; i < half; i++ {
		result = append(result, i, size-1-i)
	}
	return result
}

// generateSingleEliminationBracket builds a single-elimination bracket.
func generateSingleEliminationBracket(participants []*TournamentParticipant) ([]*TournamentMatch, int) {
	numParticipants := len(participants)

	bracketSize := 1
	for bracketSize < numParticipants {
		bracketSize *= 2
	}
	numRounds := int(math.Log2(float64(bracketSize)))

	slots := make([]*TournamentParticipant, len(participants))
	copy(slots, participants)
	for len(slots) < bracketSize {
		slots = append(slots, nil)
	}

	seededSlots := seedBracket(slots, bracketSize)

	var matches []*TournamentMatch
	matchIDCounter := 1

	var round1 []*TournamentMatch
	for i := 0; i < bracketSize; i += 2 {
		matchID := "M" + itoa(matchIDCounter)
		var p2 *TournamentParticipant
		if i+1 < len(seededSlots) {
			p2 = seededSlots[i+1]
		}
		match := &TournamentMatch{
			MatchID:      matchID,
			RoundNum:     1,
			Participant1: seededSlots[i],
			Participant2: p2,
			Completed:    false,
		}
		if match.Participant2 == nil && match.Participant1 != nil {
			match.Completed = true
			match.Winner = match.Participant1
		} else if match.Participant1 == nil && match.Participant2 != nil {
			match.Completed = true
			match.Winner = match.Participant2
		}
		round1 = append(round1, match)
		matchIDCounter++
	}

	remaining := round1
	roundNum := 2
	for len(remaining) > 1 {
		var nextRound []*TournamentMatch
		for i := 0; i < len(remaining); i += 2 {
			matchID := "M" + itoa(matchIDCounter)
			match := &TournamentMatch{
				MatchID:   matchID,
				RoundNum:  roundNum,
				Completed: false,
			}
			if i+1 < len(remaining) {
				remaining[i].NextMatchID = matchID
				remaining[i+1].NextMatchID = matchID
			} else {
				remaining[i].NextMatchID = matchID
			}
			nextRound = append(nextRound, match)
			matchIDCounter++
		}
		matches = append(matches, remaining...)
		remaining = nextRound
		roundNum++
	}
	if len(remaining) > 0 {
		matches = append(matches, remaining...)
	}

	processByeWinners(matches)
	return matches, numRounds
}

// processByeWinners places bye winners into their next-round matches, cascading
// through later rounds.
func processByeWinners(matches []*TournamentMatch) {
	changed := true
	maxIterations := len(matches) * 2
	iteration := 0
	for changed && iteration < maxIterations {
		iteration++
		changed = false
		for _, match := range matches {
			if match.Completed && match.Winner != nil && match.NextMatchID != "" {
				next := findMatchByID(matches, match.NextMatchID)
				if next == nil {
					continue
				}
				alreadyPlaced := sameParticipant(next.Participant1, match.Winner) || sameParticipant(next.Participant2, match.Winner)
				if !alreadyPlaced {
					advanceByeWinner(match, match.Winner, matches)
					changed = true
				}
			}
		}
	}
}

// advanceByeWinner places a bye winner into its next match, cascading further
// byes.
func advanceByeWinner(match *TournamentMatch, winner *TournamentParticipant, allMatches []*TournamentMatch) {
	next := findMatchByID(allMatches, match.NextMatchID)
	if next == nil {
		return
	}

	if next.Participant1 == nil {
		next.Participant1 = winner
	} else if next.Participant2 == nil {
		next.Participant2 = winner
	} else {
		return
	}

	if next.Participant1 != nil && next.Participant2 == nil {
		feeder := findCompletedUnprocessedFeeder(next, allMatches)
		if feeder != nil && feeder.Winner != nil {
			advanceByeWinner(feeder, feeder.Winner, allMatches)
		} else if len(getAllFeeders(next, allMatches)) == 0 {
			next.Completed = true
			next.Winner = next.Participant1
			if next.NextMatchID != "" {
				advanceByeWinner(next, next.Participant1, allMatches)
			}
		}
	} else if next.Participant1 == nil && next.Participant2 != nil {
		feeder := findCompletedUnprocessedFeeder(next, allMatches)
		if feeder != nil && feeder.Winner != nil {
			advanceByeWinner(feeder, feeder.Winner, allMatches)
		} else if len(getAllFeeders(next, allMatches)) == 0 {
			next.Completed = true
			next.Winner = next.Participant2
			if next.NextMatchID != "" {
				advanceByeWinner(next, next.Participant2, allMatches)
			}
		}
	}
}

// getAllFeeders returns all matches that feed into the given match (via winner
// or loser links).
func getAllFeeders(match *TournamentMatch, allMatches []*TournamentMatch) []*TournamentMatch {
	var feeders []*TournamentMatch
	for _, m := range allMatches {
		if m.NextMatchID == match.MatchID || m.LoserNextMatchID == match.MatchID {
			feeders = append(feeders, m)
		}
	}
	return feeders
}

// findCompletedUnprocessedFeeder finds a feeder whose winner hasn't yet been
// placed into match.
func findCompletedUnprocessedFeeder(match *TournamentMatch, allMatches []*TournamentMatch) *TournamentMatch {
	for _, feeder := range getAllFeeders(match, allMatches) {
		if feeder.Completed && feeder.Winner != nil {
			if !sameParticipant(match.Participant1, feeder.Winner) && !sameParticipant(match.Participant2, feeder.Winner) {
				return feeder
			}
		}
	}
	return nil
}

// findMatchByID returns the match with the given id, or nil.
func findMatchByID(matches []*TournamentMatch, id string) *TournamentMatch {
	if id == "" {
		return nil
	}
	for _, m := range matches {
		if m.MatchID == id {
			return m
		}
	}
	return nil
}
