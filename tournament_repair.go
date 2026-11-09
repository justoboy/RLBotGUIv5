package main

import "sort"

// Bracket repair for tournaments corrupted by the pre-Round-18 pointer-identity
// bug (see sameParticipant in tournament_advance.go). That bug duplicated a
// participant into BOTH slots of a downstream match (e.g. "BP vs BP") after a
// save/load round-trip, because the loser-detection and "already placed" checks
// compared pointers instead of ids.
//
// The bracket STRUCTURE (next_match_id / loser_next_match_id links) was never
// corrupted — only the participant slots (and, in team mode, the Team1/Team2
// stand-ins derived from them, which is why a match could spawn "Blue Wolves vs
// Blue Wolves" in-game even though its two participants were distinct). So we
// re-derive every match's participants from its feeders' recorded outcomes,
// processing matches in topological (round-ascending) order:
//
//   - A source match (no feeders) keeps its seed-list participants.
//   - Any other match's participants are re-derived from its feeders: a feeder
//     whose next_match_id points here contributes its Winner; a feeder whose
//     loser_next_match_id points here contributes its Loser. The match's
//     Team1/Team2 are then re-attached from the corrected participant ids.
//   - If the re-derived set EQUALS the current set (the corruption was only a
//     cosmetic duplicate, e.g. a bye rendered as "SE vs SE"), we fix the slots
//     and KEEP the recorded result.
//   - If the re-derived set DIFFERS, the recorded result was made against the
//     wrong entrants, so we un-complete the match for re-play. Downstream
//     matches re-derive from the now-empty slot, cascading the correction.
//
// This is only defined for double elimination (the only format with the WB/LB
// feeder graph that the bug corrupted).

// bracketLoser returns the participant that lost a completed match (the side
// that is not the winner), or nil if the match is incomplete or has no loser.
func bracketLoser(m *TournamentMatch) *TournamentParticipant {
	if !m.Completed || m.Winner == nil {
		return nil
	}
	if m.Participant1 != nil && !sameParticipant(m.Participant1, m.Winner) {
		return m.Participant1
	}
	if m.Participant2 != nil && !sameParticipant(m.Participant2, m.Winner) {
		return m.Participant2
	}
	return nil
}

// feedersOf returns every match that feeds into matchID, in a stable order
// (by feeder id) so slot assignment is deterministic.
func feedersOf(all []*TournamentMatch, matchID string) []*TournamentMatch {
	var out []*TournamentMatch
	for _, f := range all {
		if f.NextMatchID == matchID || f.LoserNextMatchID == matchID {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].MatchID < out[j].MatchID })
	return out
}

// idSet collects the non-nil participant ids of a match as a comparable set.
func idSet(a, b *TournamentParticipant) []string {
	var s []string
	if a != nil {
		s = append(s, a.ParticipantID)
	}
	if b != nil {
		s = append(s, b.ParticipantID)
	}
	sort.Strings(s)
	return s
}

func setsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TournamentRepairBracket re-derives corrupted participant slots from the
// (intact) feeder links. See the file comment for the full algorithm.
func (a *App) TournamentRepairBracket() (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	s := a.tournament
	if s.Format != "double_elimination" {
		return nil, errorf("bracket repair only supports double elimination")
	}

	all := s.allMatches()

	// WB match ids, so we can order WB matches before LB matches within the same
	// round (a WB match's loser feeds an LB match in the same round, so the WB
	// match must be finalized first).
	wbIDs := map[string]bool{}
	for _, m := range s.Matches {
		wbIDs[m.MatchID] = true
	}

	// Topological order: ascending round; within a round, WB before LB; then by
	// id. WB matches only depend on strictly-earlier rounds, and LB matches
	// depend on same-round WB (WB-first) plus earlier LB — so this ordering
	// guarantees every feeder is finalized before its child is re-derived.
	ordered := make([]*TournamentMatch, len(all))
	copy(ordered, all)
	sort.SliceStable(ordered, func(i, j int) bool {
		mi, mj := ordered[i], ordered[j]
		if mi.RoundNum != mj.RoundNum {
			return mi.RoundNum < mj.RoundNum
		}
		li, lj := wbIDs[mi.MatchID], wbIDs[mj.MatchID]
		if li != lj {
			return li // WB (true) before LB (false)
		}
		return mi.MatchID < mj.MatchID
	})

	for _, m := range ordered {
		feeders := feedersOf(all, m.MatchID)
		if len(feeders) == 0 {
			continue // source match: participants come from the seed list
		}

		// Re-derive incoming entrants in feeder order, and track whether every
		// feeder has already produced its outcome (a completed feeder). A match
		// with a single entrant is a BYE only once all its feeders are done;
		// otherwise it's still waiting on a pending feeder and must stay open.
		var entrants []*TournamentParticipant
		allFeedersCompleted := true
		for _, f := range feeders {
			if !f.Completed {
				allFeedersCompleted = false
			}
			if f.NextMatchID == m.MatchID && f.Winner != nil {
				entrants = append(entrants, f.Winner) // winner advances
			}
			if f.LoserNextMatchID == m.MatchID {
				if l := bracketLoser(f); l != nil {
					entrants = append(entrants, l) // loser drops down
				}
			}
		}

		switch {
		case len(entrants) == 1 && allFeedersCompleted:
			// Bye: auto-complete and propagate the winner downstream.
			m.Participant1, m.Participant2 = entrants[0], nil
			m.Completed = true
			m.Winner = entrants[0]
			attachTeamsToMatch(s, m)
		case len(entrants) == 2:
			np1, np2 := entrants[0], entrants[1]
			if setsEqual(idSet(m.Participant1, m.Participant2), idSet(np1, np2)) {
				// Cosmetic duplicate only — keep the recorded result, fix slots.
				m.Participant1, m.Participant2 = np1, np2
				attachTeamsToMatch(s, m)
			} else {
				// Wrong entrants — the recorded result is untrustworthy.
				m.Participant1, m.Participant2 = np1, np2
				m.Completed = false
				m.Winner = nil
				m.WinnerTeam = nil
				attachTeamsToMatch(s, m)
			}
		default:
			// 0 entrants, or 1 entrant still waiting on a pending feeder: leave
			// the match open so it can be played once its remaining feeder lands.
			if len(entrants) == 1 {
				m.Participant1, m.Participant2 = entrants[0], nil
			} else {
				m.Participant1, m.Participant2 = nil, nil
			}
			m.Completed = false
			m.Winner = nil
			m.WinnerTeam = nil
			attachTeamsToMatch(s, m)
		}
	}

	recomputeTeamRecords(s)

	// If the grand final is now complete, declare the tournament winner.
	if s.Winner == nil {
		for _, m := range s.Matches {
			if isTournamentFinalMatch(m) && m.Completed && m.Winner != nil {
				s.Winner = m.Winner
				s.WinnerTeam = m.WinnerTeam
				s.Completed = true
				break
			}
		}
	}

	a.persistLocked()
	return s, nil
}

// attachTeamsToMatch sets a match's Team1/Team2 from the participants' ids.
func attachTeamsToMatch(s *TournamentState, m *TournamentMatch) {
	if len(s.Teams) == 0 {
		return
	}
	byID := map[string]*TournamentTeam{}
	for _, t := range s.Teams {
		byID[t.TeamID] = t
	}
	if m.Participant1 != nil {
		m.Team1 = byID[m.Participant1.ParticipantID]
	}
	if m.Participant2 != nil {
		m.Team2 = byID[m.Participant2.ParticipantID]
	}
	if m.Winner != nil {
		m.WinnerTeam = byID[m.Winner.ParticipantID]
	}
}

// recomputeTeamRecords recomputes each team's Wins/Losses from the currently
// completed matches (used after a repair un-completes matches).
func recomputeTeamRecords(s *TournamentState) {
	if len(s.Teams) == 0 {
		return
	}
	wins := map[string]int{}
	losses := map[string]int{}
	for _, m := range s.allMatches() {
		if !m.Completed || m.Winner == nil {
			continue
		}
		loser := bracketLoser(m)
		wins[m.Winner.ParticipantID]++
		if loser != nil {
			losses[loser.ParticipantID]++
		}
	}
	for _, t := range s.Teams {
		t.Wins = wins[t.TeamID]
		t.Losses = losses[t.TeamID]
	}
}
