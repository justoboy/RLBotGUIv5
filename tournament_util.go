package main

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/RLBot/go-interface/flat"
)

// flatMutator aliases the flat mutator settings type so tournament files can
// reference it without importing the flat package everywhere.
type flatMutator = flat.MutatorSettingsT

// validGameModes is the set of accepted game-mode strings for a tournament.
// These strings must match the v5 GUI's game_mode vocabulary (see
// frontend/src/components/MatchSettings/rlmutators.ts and the switch in
// tournament_match.go), i.e. "Soccar" and "Snowday", NOT "Soccer"/"Hockey".
var validGameModes = map[string]bool{
	"Soccar":     true,
	"Rumble":     true,
	"Heatseeker": true,
	"Hoops":      true,
	"Dropshot":   true,
	"Snowday":    true,
	"Gridiron":   true,
	"Knockout":   true,
}

// errorf is a tiny fmt.Errorf wrapper for readability at call sites.
func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// randShuffleParticipants shuffles a participant slice in place.
func randShuffleParticipants(participants []*TournamentParticipant) {
	rand.Shuffle(len(participants), func(i, j int) {
		participants[i], participants[j] = participants[j], participants[i]
	})
}

// insertAt returns a new slice with v inserted at index idx (clamped).
func insertAt(participants []*TournamentParticipant, idx int, v *TournamentParticipant) []*TournamentParticipant {
	if idx < 0 {
		idx = 0
	}
	if idx > len(participants) {
		idx = len(participants)
	}
	out := make([]*TournamentParticipant, 0, len(participants)+1)
	out = append(out, participants[:idx]...)
	out = append(out, v)
	out = append(out, participants[idx:]...)
	return out
}

// itoa is a tiny wrapper around strconv.Itoa for readability at call sites.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// mod returns the non-negative modulo of a mod b (Go's % is negative when the
// dividend is negative). Used by the round-robin circle-method rotation.
func mod(a, b int) int {
	return ((a % b) + b) % b
}

// sortBySeed sorts participants by seed ascending, in place (stable).
func sortBySeed(participants []*TournamentParticipant) {
	// Insertion sort keeps stability without importing "sort" for a tiny slice.
	for i := 1; i < len(participants); i++ {
		key := participants[i]
		j := i - 1
		for j >= 0 && participants[j].Seed > key.Seed {
			participants[j+1] = participants[j]
			j--
		}
		participants[j+1] = key
	}
}
