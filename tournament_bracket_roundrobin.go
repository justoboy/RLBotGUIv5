package main

import "math"

// generateRoundRobinBracket builds a round-robin schedule using the circle
// method. For even N: N-1 rounds; for odd N: N rounds with one bye per round.
func generateRoundRobinBracket(participants []*TournamentParticipant) []*TournamentMatch {
	var matches []*TournamentMatch
	matchID := 1
	numParticipants := len(participants)
	if numParticipants < 2 {
		return matches
	}

	indices := make([]int, numParticipants)
	for i := range indices {
		indices[i] = i
	}

	var numRounds, perRound int
	if numParticipants%2 == 0 {
		numRounds = numParticipants - 1
		perRound = numParticipants / 2
	} else {
		numRounds = numParticipants
		perRound = numParticipants / 2
	}

	for roundNum := 1; roundNum <= numRounds; roundNum++ {
		rotated := make([]int, 0, numParticipants)
		rotated = append(rotated, indices[0])
		for i := 1; i < numParticipants; i++ {
			if numParticipants > 2 {
				// Circle method: position 0 is fixed, positions 1..N-1 rotate by
				// roundNum. Go's % is negative for negative dividends, so wrap with
				// a non-negative modulo (this was the source of the index [-1] panic).
				rotatedIdx := 1 + mod(i-roundNum, numParticipants-1)
				rotated = append(rotated, indices[rotatedIdx])
			} else {
				rotated = append(rotated, indices[1])
			}
		}

		for i := 0; i < perRound; i++ {
			p1Idx := rotated[i]
			p2Idx := rotated[numParticipants-1-i]
			if p1Idx == p2Idx {
				continue
			}
			match := &TournamentMatch{
				MatchID:      "M" + itoa(matchID),
				RoundNum:     roundNum,
				Participant1: participants[p1Idx],
				Participant2: participants[p2Idx],
				Completed:    false,
			}
			matches = append(matches, match)
			matchID++
		}
	}
	return matches
}

// calculateSwissRounds returns ceil(log2(numParticipants)), minimum 1.
func calculateSwissRounds(numParticipants int) int {
	if numParticipants < 2 {
		return 1
	}
	return max(1, int(math.Ceil(math.Log2(float64(numParticipants)))))
}

// generateSwissRound1 pairs participants in seed order: 1v2, 3v4, etc.
func generateSwissRound1(participants []*TournamentParticipant) []*TournamentMatch {
	var matches []*TournamentMatch
	for i := 0; i < len(participants)-1; i += 2 {
		match := &TournamentMatch{
			MatchID:      "S1_" + itoa(i/2+1),
			RoundNum:     1,
			Participant1: participants[i],
			Participant2: participants[i+1],
			Completed:    false,
		}
		matches = append(matches, match)
	}
	return matches
}
