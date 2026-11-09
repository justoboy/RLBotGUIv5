package main

import "math"

// generateDoubleEliminationBracket builds a double-elimination bracket.
// Returns (winnersMatches, losersMatches, grandFinal, numRounds).
func generateDoubleEliminationBracket(participants []*TournamentParticipant) ([]*TournamentMatch, []*TournamentMatch, *TournamentMatch, int) {
	numParticipants := len(participants)
	if numParticipants < 2 {
		return nil, nil, nil, 0
	}

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

	var winnersMatches []*TournamentMatch
	matchIDCounter := 1

	var round1 []*TournamentMatch
	for i := 0; i < bracketSize; i += 2 {
		matchID := "W" + itoa(matchIDCounter)
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
	wbRound := 2
	for len(remaining) > 1 {
		var nextRound []*TournamentMatch
		for i := 0; i < len(remaining); i += 2 {
			matchID := "W" + itoa(matchIDCounter)
			match := &TournamentMatch{
				MatchID:   matchID,
				RoundNum:  wbRound,
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
		winnersMatches = append(winnersMatches, remaining...)
		remaining = nextRound
		wbRound++
	}
	if len(remaining) > 0 {
		winnersMatches = append(winnersMatches, remaining...)
	}

	processByeWinners(winnersMatches)

	// Group WB matches by round to compute losers per round.
	wbByRoundCalc := make(map[int][]*TournamentMatch)
	for _, m := range winnersMatches {
		wbByRoundCalc[m.RoundNum] = append(wbByRoundCalc[m.RoundNum], m)
	}
	wbLosersByRound := make(map[int]int)
	for wbRoundNum, ms := range wbByRoundCalc {
		count := 0
		if wbRoundNum == 1 {
			for _, m := range ms {
				if m.Participant1 != nil && m.Participant2 != nil {
					count++
				}
			}
		} else {
			count = len(ms)
		}
		wbLosersByRound[wbRoundNum] = count
	}

	var losersMatches []*TournamentMatch
	lbByRound := make(map[int][]*TournamentMatch)
	lbMatchID := 1
	numLBRounds := 2*numRounds - 2

	if numLBRounds <= 0 {
		return winnersMatches, nil, nil, numRounds
	}

	for lbRoundNum := 1; lbRoundNum <= numLBRounds; lbRoundNum++ {
		var numLBMatches int
		if lbRoundNum == 1 {
			losers := wbLosersByRound[1]
			if losers > 0 {
				numLBMatches = int(math.Ceil(float64(losers) / 2))
			}
		} else if lbRoundNum%2 == 1 {
			prev := len(lbByRound[lbRoundNum-1])
			if prev > 0 {
				numLBMatches = int(math.Ceil(float64(prev) / 2))
			}
		} else {
			prev := len(lbByRound[lbRoundNum-1])
			wbRoundForLosers := (lbRoundNum / 2) + 1
			losersFromWB := wbLosersByRound[wbRoundForLosers]
			total := prev + losersFromWB
			if total > 0 {
				numLBMatches = int(math.Ceil(float64(total) / 2))
			}
		}
		if numLBMatches < 1 && (lbRoundNum == 1 || len(lbByRound[lbRoundNum-1]) > 0 || wbLosersByRound[(lbRoundNum/2)+1] > 0) {
			numLBMatches = 1
		}

		var roundMatches []*TournamentMatch
		for i := 0; i < numLBMatches; i++ {
			matchID := "L" + itoa(lbMatchID)
			match := &TournamentMatch{
				MatchID:   matchID,
				RoundNum:  lbRoundNum,
				Completed: false,
			}
			roundMatches = append(roundMatches, match)
			lbMatchID++
		}
		lbByRound[lbRoundNum] = roundMatches
		losersMatches = append(losersMatches, roundMatches...)
	}

	grandFinalID := "L" + itoa(lbMatchID)
	grandFinalRound := numLBRounds + 1
	grandFinal := &TournamentMatch{
		MatchID:   grandFinalID,
		RoundNum:  grandFinalRound,
		Completed: false,
	}

	lbFinalsRound := numLBRounds
	var lbFinalsID string
	var lbFinals *TournamentMatch
	if ms, ok := lbByRound[lbFinalsRound]; ok && len(ms) > 0 {
		lbFinals = ms[0]
		lbFinalsID = lbFinals.MatchID
	}

	// Link LB R1 -> LB R2.
	if r1, ok := lbByRound[1]; ok {
		if r2, ok2 := lbByRound[2]; ok2 {
			for i, lb := range r1 {
				if i < len(r2) {
					lb.NextMatchID = r2[i].MatchID
				}
			}
		}
	}

	for lbRoundNum := 2; lbRoundNum < numLBRounds; lbRoundNum++ {
		lbRoundMatches := lbByRound[lbRoundNum]
		nextRound := lbByRound[lbRoundNum+1]
		if len(lbRoundMatches) == 1 {
			if len(nextRound) > 0 {
				lbRoundMatches[0].NextMatchID = nextRound[0].MatchID
			}
		} else {
			if lbRoundNum%2 == 1 {
				for i, lb := range lbRoundMatches {
					if i < len(nextRound) {
						lb.NextMatchID = nextRound[i].MatchID
					}
				}
			} else {
				for i := 0; i < len(lbRoundMatches); i += 2 {
					nextIdx := i / 2
					if i+1 < len(lbRoundMatches) {
						if nextIdx < len(nextRound) {
							lbRoundMatches[i].NextMatchID = nextRound[nextIdx].MatchID
							lbRoundMatches[i+1].NextMatchID = nextRound[nextIdx].MatchID
						}
					} else if i < len(lbRoundMatches) {
						if nextIdx < len(nextRound) {
							lbRoundMatches[i].NextMatchID = nextRound[nextIdx].MatchID
						}
					}
				}
			}
		}
	}

	if lbFinals != nil {
		lbFinals.NextMatchID = grandFinalID
	}

	// Group WB by round for loser linking.
	wbByRound := make(map[int][]*TournamentMatch)
	for _, m := range winnersMatches {
		wbByRound[m.RoundNum] = append(wbByRound[m.RoundNum], m)
	}

	// Link WB R1 losers to LB R1.
	if wbR1, ok := wbByRound[1]; ok {
		if lbR1, ok2 := lbByRound[1]; ok2 {
			var actual []*TournamentMatch
			for _, m := range wbR1 {
				if m.Participant1 != nil && m.Participant2 != nil {
					actual = append(actual, m)
				}
			}
			for i, lb := range lbR1 {
				i1 := i * 2
				i2 := i*2 + 1
				if i1 < len(actual) {
					actual[i1].LoserNextMatchID = lb.MatchID
				}
				if i2 < len(actual) {
					actual[i2].LoserNextMatchID = lb.MatchID
				}
			}
		}
	}

	// Link WB R N (2..numRounds-1) losers to LB R (2N-2).
	for wbRound := 2; wbRound < numRounds; wbRound++ {
		wbMatches, ok := wbByRound[wbRound]
		if !ok {
			continue
		}
		actual := make([]*TournamentMatch, len(wbMatches))
		copy(actual, wbMatches)
		lbRoundNum := 2*wbRound - 2
		lbMatches, ok := lbByRound[lbRoundNum]
		if !ok {
			continue
		}
		prevLB := lbByRound[lbRoundNum-1]

		lbIdx := 0
		wbIdx := 0
		for _, prev := range prevLB {
			_ = prev
			if wbIdx < len(actual) && lbIdx < len(lbMatches) {
				actual[wbIdx].LoserNextMatchID = lbMatches[lbIdx].MatchID
				wbIdx++
				lbIdx++
			}
		}
		for wbIdx < len(actual) {
			if lbIdx < len(lbMatches) {
				actual[wbIdx].LoserNextMatchID = lbMatches[lbIdx].MatchID
				wbIdx++
				if wbIdx < len(actual) {
					actual[wbIdx].LoserNextMatchID = lbMatches[lbIdx].MatchID
					wbIdx++
				}
				lbIdx++
			} else {
				wbIdx++
			}
		}
	}

	// Link WB Finals to Grand Final.
	for _, m := range winnersMatches {
		if m.NextMatchID == "" {
			m.NextMatchID = grandFinalID
			if numLBRounds > 0 && len(lbByRound[numLBRounds]) > 0 {
				m.LoserNextMatchID = lbFinalsID
			}
			break
		}
	}

	processByeWinners(losersMatches)

	return winnersMatches, losersMatches, grandFinal, numRounds
}
