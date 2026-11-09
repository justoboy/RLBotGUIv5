package main

// Match advancement + bracket progression, ported from the v4 fork's
// tournament_runner.py (advance_winner, _populate_losers_bracket_match,
// _check_and_handle_lb_bye, _handle_bye_situation, _handle_swiss_progression,
// _handle_round_robin_progression, is_tournament_final_match).
//
// These operate on a *TournamentState directly (v4 used a module-global
// CURRENT_TOURNAMENT; here the state is passed explicitly).

// allMatches returns winners-bracket and losers-bracket matches combined.
func (s *TournamentState) allMatches() []*TournamentMatch {
	return append(append([]*TournamentMatch{}, s.Matches...), s.LosersBracketMatches...)
}

// sameParticipant reports whether two participant references denote the SAME
// entrant. This is a VALUE comparison (by ParticipantID), not pointer identity.
//
// This matters because the state is persisted to JSON and reloaded on restart.
// Within a single session a participant object is shared by pointer across the
// matches that reference it, but json.Marshal/Unmarshal deep-copies it into each
// match on a save/load round-trip, so the pointers diverge while the id stays
// the same. v4 relied on Python dataclass value-equality here; without a value
// comparison the reload silently breaks loser-detection and the "already placed"
// bye checks, which re-advances an already-placed winner and duplicates a
// participant across the bracket.
func sameParticipant(a, b *TournamentParticipant) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ParticipantID == b.ParticipantID
}

// findMatchByIDInState searches both brackets for a match by id.
func (s *TournamentState) findMatchByIDInState(id string) *TournamentMatch {
	if id == "" {
		return nil
	}
	for _, m := range s.Matches {
		if m.MatchID == id {
			return m
		}
	}
	for _, m := range s.LosersBracketMatches {
		if m.MatchID == id {
			return m
		}
	}
	return nil
}

// isTournamentFinalMatch reports whether a match is the tournament final (its
// winner does not advance anywhere).
func isTournamentFinalMatch(match *TournamentMatch) bool {
	return match.NextMatchID == "" && match.LoserNextMatchID == ""
}

// advanceWinner places the winner into its next match (and attaches the winning
// team in team mode), cascading any resulting bye.
func (s *TournamentState) advanceWinner(match *TournamentMatch, winner *TournamentParticipant) {
	nextID := match.NextMatchID
	if nextID == "" {
		return
	}
	next := s.findMatchByIDInState(nextID)
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

	if len(s.Teams) > 0 {
		var winningTeam *TournamentTeam
		for _, t := range s.Teams {
			if t.TeamID == winner.ParticipantID {
				winningTeam = t
				break
			}
		}
		if winningTeam != nil {
			if sameParticipant(next.Participant1, winner) && next.Team1 == nil {
				next.Team1 = winningTeam
			} else if sameParticipant(next.Participant2, winner) && next.Team2 == nil {
				next.Team2 = winningTeam
			}
		}
	}

	s.handleByeSituation(next)
}

// populateLosersBracketMatch moves the loser of a completed WB match into its
// linked LB match, then checks for an LB bye.
func (s *TournamentState) populateLosersBracketMatch(completed *TournamentMatch, winner *TournamentParticipant) {
	if s.Format != "double_elimination" || completed.LoserNextMatchID == "" {
		return
	}
	lb := s.findMatchByIDInState(completed.LoserNextMatchID)
	if lb == nil {
		return
	}

	var loser *TournamentParticipant
	if completed.Participant1 != nil && !sameParticipant(completed.Participant1, winner) {
		loser = completed.Participant1
	} else if completed.Participant2 != nil && !sameParticipant(completed.Participant2, winner) {
		loser = completed.Participant2
	}
	if loser == nil {
		return
	}

	if lb.Participant1 == nil {
		lb.Participant1 = loser
	} else if lb.Participant2 == nil {
		lb.Participant2 = loser
	} else {
		return
	}

	if len(s.Teams) > 0 {
		var losingTeam *TournamentTeam
		for _, t := range s.Teams {
			if t.TeamID == loser.ParticipantID {
				losingTeam = t
				break
			}
		}
		if losingTeam != nil {
			if sameParticipant(lb.Participant1, loser) && lb.Team1 == nil {
				lb.Team1 = losingTeam
			} else if sameParticipant(lb.Participant2, loser) && lb.Team2 == nil {
				lb.Team2 = losingTeam
			}
		}
	}

	s.checkAndHandleLBBye(lb)
}

// checkAndHandleLBBye auto-advances a single-participant LB match once all of
// its feeders are exhausted.
func (s *TournamentState) checkAndHandleLBBye(lb *TournamentMatch) {
	hasP1 := lb.Participant1 != nil
	hasP2 := lb.Participant2 != nil
	if hasP1 && hasP2 {
		return
	}
	if !hasP1 && !hasP2 {
		return
	}

	wbFeederCount, completedWBFeeder := 0, 0
	for _, m := range s.Matches {
		if m.LoserNextMatchID == lb.MatchID {
			if m.Participant1 != nil && m.Participant2 != nil {
				wbFeederCount++
				if m.Completed {
					completedWBFeeder++
				}
			}
		}
	}
	lbFeederCount, completedLBFeeder := 0, 0
	for _, m := range s.LosersBracketMatches {
		if m.NextMatchID == lb.MatchID {
			lbFeederCount++
			if m.Completed {
				completedLBFeeder++
			}
		}
	}

	wbExhausted := wbFeederCount == 0 || completedWBFeeder == wbFeederCount
	lbExhausted := lbFeederCount == 0 || completedLBFeeder == lbFeederCount

	if wbExhausted && lbExhausted {
		single := lb.Participant1
		if single == nil {
			single = lb.Participant2
		}
		lb.Completed = true
		lb.Winner = single
		if lb.NextMatchID != "" {
			s.advanceWinner(lb, single)
		}
	}
}

// handleByeSituation auto-advances a single-participant match when no other
// feeder will produce a participant.
func (s *TournamentState) handleByeSituation(next *TournamentMatch) {
	if next.Participant1 != nil && next.Participant2 == nil {
		s.handleByeForSide(next, next.Participant1)
	} else if next.Participant1 == nil && next.Participant2 != nil {
		s.handleByeForSide(next, next.Participant2)
	}
}

func (s *TournamentState) handleByeForSide(next *TournamentMatch, single *TournamentParticipant) {
	feeder := s.findCompletedUnprocessedFeeder(next)
	if feeder != nil && feeder.Winner != nil {
		s.advanceWinner(feeder, feeder.Winner)
		return
	}

	feeders := s.getAllFeeders(next)
	hasPendingReal := false
	for _, f := range feeders {
		if f.LoserNextMatchID == next.MatchID {
			h1 := f.Participant1 != nil
			h2 := f.Participant2 != nil
			if h1 && h2 {
				if !f.Completed {
					hasPendingReal = true
					break
				}
			} else if !h1 && !h2 {
				hasPendingReal = true
				break
			}
		}
		if f.NextMatchID == next.MatchID && !f.Completed {
			hasPendingReal = true
			break
		}
	}

	if len(feeders) == 0 || !hasPendingReal {
		next.Completed = true
		next.Winner = single
		if next.NextMatchID != "" {
			s.advanceWinner(next, single)
		}
	}
}

func (s *TournamentState) getAllFeeders(match *TournamentMatch) []*TournamentMatch {
	var feeders []*TournamentMatch
	for _, m := range s.Matches {
		if m.NextMatchID == match.MatchID || m.LoserNextMatchID == match.MatchID {
			feeders = append(feeders, m)
		}
	}
	for _, m := range s.LosersBracketMatches {
		if m.NextMatchID == match.MatchID || m.LoserNextMatchID == match.MatchID {
			feeders = append(feeders, m)
		}
	}
	return feeders
}

func (s *TournamentState) findCompletedUnprocessedFeeder(match *TournamentMatch) *TournamentMatch {
	for _, f := range s.getAllFeeders(match) {
		if f.Completed && f.Winner != nil {
			if !sameParticipant(match.Participant1, f.Winner) && !sameParticipant(match.Participant2, f.Winner) {
				return f
			}
		}
	}
	return nil
}

// --- Swiss / round-robin progression ---

// swissEntities returns the stand-in participants (teams in team mode).
func (s *TournamentState) swissEntities() []*TournamentParticipant {
	if s.TeamSize > 1 && len(s.Teams) > 0 {
		var standIns []*TournamentParticipant
		for _, t := range s.Teams {
			standIns = append(standIns, &TournamentParticipant{
				Name:            t.Name,
				ParticipantID:   t.TeamID,
				ParticipantType: "team",
				Seed:            0,
			})
		}
		return standIns
	}
	return s.Participants
}

// swissNameToEID maps player name -> owning entity id.
func (s *TournamentState) swissNameToEID() map[string]string {
	m := map[string]string{}
	if s.TeamSize > 1 && len(s.Teams) > 0 {
		for _, t := range s.Teams {
			for _, p := range t.Participants {
				m[p.Name] = t.TeamID
			}
		}
	} else {
		for _, p := range s.Participants {
			m[p.Name] = p.ParticipantID
		}
	}
	return m
}

// swissAttachTeams attaches team1/team2 to swiss matches in team mode.
func (s *TournamentState) swissAttachTeams(matches []*TournamentMatch) {
	if s.TeamSize <= 1 || len(s.Teams) == 0 {
		return
	}
	byID := map[string]*TournamentTeam{}
	for _, t := range s.Teams {
		byID[t.TeamID] = t
	}
	for _, m := range matches {
		if m.Participant1 != nil {
			if t, ok := byID[m.Participant1.ParticipantID]; ok {
				m.Team1 = t
			}
		}
		if m.Participant2 != nil {
			if t, ok := byID[m.Participant2.ParticipantID]; ok {
				m.Team2 = t
			}
		}
	}
}

func (s *TournamentState) swissDeclareWinner(winner *TournamentParticipant) {
	s.Winner = winner
	if s.TeamSize > 1 && len(s.Teams) > 0 {
		for _, t := range s.Teams {
			if t.TeamID == winner.ParticipantID {
				s.WinnerTeam = t
				break
			}
		}
	}
	s.Completed = true
}

// handleSwissProgression advances a Swiss tournament after a match completes.
func (s *TournamentState) handleSwissProgression(completed *TournamentMatch) {
	if s.Format != "swiss" {
		return
	}
	if completed.RoundNum > s.SwissRounds {
		if completed.Winner != nil {
			s.swissDeclareWinner(completed.Winner)
		}
		return
	}

	var roundMatches []*TournamentMatch
	allDone := true
	for _, m := range s.Matches {
		if m.RoundNum == completed.RoundNum {
			roundMatches = append(roundMatches, m)
			if !m.Completed {
				allDone = false
			}
		}
	}
	if !allDone {
		return
	}

	if completed.RoundNum < s.SwissRounds {
		next := completed.RoundNum + 1
		for _, m := range s.Matches {
			if m.RoundNum == next {
				return // already generated
			}
		}
		entities := s.swissEntities()
		var done []*TournamentMatch
		for _, m := range s.Matches {
			if m.Completed {
				done = append(done, m)
			}
		}
		newMatches := generateSwissNextRound(entities, done, next, s.swissNameToEID())
		s.swissAttachTeams(newMatches)
		s.Matches = append(s.Matches, newMatches...)
		s.CurrentRound = next
	} else {
		entities := s.swissEntities()
		var done []*TournamentMatch
		for _, m := range s.Matches {
			if m.Completed {
				done = append(done, m)
			}
		}
		result := determineSwissWinner(entities, done, s.swissNameToEID())
		if result.PlayoffNeeded && !s.SwissPlayoffScheduled {
			playoff := &TournamentMatch{
				MatchID:      "SW_PLAYOFF",
				RoundNum:     s.SwissRounds + 1,
				Participant1: result.PlayoffParticipants[0],
				Participant2: result.PlayoffParticipants[1],
				Completed:    false,
			}
			s.swissAttachTeams([]*TournamentMatch{playoff})
			s.Matches = append(s.Matches, playoff)
			s.SwissPlayoffScheduled = true
			s.CurrentRound = s.SwissRounds + 1
		} else if result.Winner != nil {
			s.swissDeclareWinner(result.Winner)
		}
	}
}

// handleRoundRobinProgression declares the round-robin winner once all matches
// are complete.
func (s *TournamentState) handleRoundRobinProgression() {
	if s.Format != "round_robin" || s.Completed {
		return
	}
	for _, m := range s.Matches {
		if !m.Completed {
			return
		}
	}
	entities := s.swissEntities()
	var done []*TournamentMatch
	for _, m := range s.Matches {
		if m.Completed {
			done = append(done, m)
		}
	}
	ordered, _ := rankEntities(entities, done, s.swissNameToEID())
	if len(ordered) > 0 {
		s.swissDeclareWinner(ordered[0])
	}
}
