package main

import "math/rand"

// Map pools per game mode, ported from the v4 fork's tournament_runner.py.
// Used when a tournament has map randomization enabled. Each pool maps a map's
// PACKAGE IDENTIFIER (the value the core loads as game_map_upk, e.g.
// "eurostadium_snownight_p") to its selection weight, mirroring Rocket League's
// official map rotation drop chances (normalized to the subset of rotation maps
// RLBot can load). Maps outside a mode's rotation are extra rare (weight 1.0);
// special themed variants are super rare (weight 0.25).
//
// IMPORTANT: keys are the v5 package identifiers (the VALUES in
// frontend/src/arena-names.ts), NOT the v4 underscore display names. The v5 core
// only loads the package identifier; unlike v4 it does not auto-resolve pretty
// names. pickRandomMap returns the key directly, so the key must already be a
// loadable identifier. The tournament UI's NiceSelect maps the stored package
// value back to its friendly display label, so randomizing still shows e.g.
// "Mannfield (Snowy)".
//
// v4's super-rare "Farmstead_Upsidedown" was intentionally dropped: it has no
// entry in v5's arena-names.ts, so the v5 core cannot load it. (Marked for review.)

var soccarRotationWeights = map[string]float64{
	"Stadium_P": 7.67, "EuroStadium_P": 7.67, "Park_P": 4.93,
	"cs_p": 4.56, "UtopiaStadium_P": 4.38, "TrainStation_P": 4.01,
	"farm_grs_p": 3.65, "CHN_Stadium_P": 3.65, "NeoTokyo_Standard_P": 3.65,
	"wasteland_grs_p": 3.65, "ff_dusk_p": 2.92, "Street_P": 2.74,
	"Underwater_P": 2.74, "outlaw_oasis_p": 2.55, "cs_day_p": 2.37,
	"CHN_Stadium_Day_P": 2.37, "Park_Night_P": 1.46,
	"UtopiaStadium_Lux_P": 1.27, "eurostadium_dusk_p": 1.09, "stadium_day_p": 1.02,
	"TrainStation_Dawn_P": 0.91, "TrainStation_Night_P": 0.91,
	"Park_Rainy_P": 0.73, "EuroStadium_Night_P": 0.73, "Stadium_Foggy_P": 0.58,
	"UtopiaStadium_Dusk_P": 0.54, "EuroStadium_Rainy_P": 0.36,
}

var soccarExtraRareWeights = map[string]float64{
	"arc_standard_p": 1.0, "beach_P": 1.0, "beach_night_p": 1.0,
	"wasteland_s_p": 1.0, "Farm_Night_P": 1.0, "Outlaw_P": 1.0,
}

var soccarSuperRareWeights = map[string]float64{
	"ARC_Darc_P": 0.25, "Stadium_Race_Day_P": 0.25, "music_p": 0.25,
	"cs_hw_p": 0.25, "Park_Bman_P": 0.25, "Farm_HW_P": 0.25,
	"NeoTokyo_Toon_P": 0.25, "neotokyo_hax_p": 0.25,
	"fni_stadium_p": 0.25, "swoosh_p": 0.25,
	"Haunted_TrainStation_P": 0.25,
}

func mergeMaps(ms ...map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for _, m := range ms {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

var soccarMapPool = mergeMaps(soccarRotationWeights, soccarExtraRareWeights, soccarSuperRareWeights)

var rumbleMapPool = func() map[string]float64 {
	base := map[string]float64{
		"Stadium_P": 6.1, "EuroStadium_P": 6.1, "Park_P": 3.92,
		"cs_p": 3.63, "UtopiaStadium_P": 3.49, "TrainStation_P": 3.2,
		"ARC_P": 2.9, "Wasteland_P": 2.9, "Wasteland_Night_P": 2.9,
		"Labs_DoubleGoal_V2_P": 2.9, "farm_grs_p": 2.9, "CHN_Stadium_P": 2.9,
		"NeoTokyo_Standard_P": 2.9, "ThrowbackStadium_P": 2.9, "NeoTokyo_P": 2.9,
		"Labs_Underpass_P": 2.9, "wasteland_grs_p": 2.9, "Underwater_P": 4.5,
		"ff_dusk_p": 2.32, "Street_P": 2.18, "outlaw_oasis_p": 2.03,
		"cs_day_p": 1.89, "CHN_Stadium_Day_P": 1.89,
		"Park_Night_P": 1.16, "UtopiaStadium_Lux_P": 1.01,
		"eurostadium_dusk_p": 0.87, "stadium_day_p": 0.81, "TrainStation_Dawn_P": 0.72,
		"TrainStation_Night_P": 0.72, "Park_Rainy_P": 0.58, "EuroStadium_Night_P": 0.58,
		"Stadium_Foggy_P": 0.46, "UtopiaStadium_Dusk_P": 0.43, "EuroStadium_Rainy_P": 0.29,
	}
	return mergeMaps(base, soccarExtraRareWeights, soccarSuperRareWeights)
}()

var heatseekerMapPool = func() map[string]float64 {
	return mergeMaps(soccarMapPool, map[string]float64{
		"Labs_Basin_P": 0.25, "Labs_Corridor_P": 0.25, "Labs_Galleon_P": 0.25,
		"Labs_Galleon_Mast_P": 0.25, "Labs_Holyfield_P": 0.25, "Labs_PillarGlass_P": 0.25,
		"Labs_PillarHeat_P": 0.25, "Labs_PillarWings_P": 0.25, "Labs_Octagon_02_P": 0.25,
		"Labs_CirclePillars_P": 0.25, "Labs_Cosmic_V4_P": 0.25, "Labs_Utopia_P": 0.25,
	})
}()

var hockeyMapPool = map[string]float64{
	"Stadium_Winter_P": 33.33, "eurostadium_snownight_p": 33.33, "UtopiaStadium_Snow_P": 33.33,
	"ThrowbackHockey_p": 1.0, "Park_Snowy_P": 1.0,
}

var mapPools = map[string]map[string]float64{
	"Soccar":     soccarMapPool,
	"Rumble":     rumbleMapPool,
	"Heatseeker": heatseekerMapPool,
	"Hoops":      {"HoopsStadium_P": 50.0, "HoopsStreet_P": 50.0},
	"Dropshot":   {"ShatterShot_P": 100.0},
	"Snowday":    hockeyMapPool,
	"Gridiron":   {"BB_P": 100.0},
}

// pickRandomMap does weighted-random map selection for a game mode, mirroring
// v4's random.choices(maps, weights=weights, k=1). Returns a package identifier
// the core can load directly as game_map_upk.
func pickRandomMap(gameMode string) string {
	pool, ok := mapPools[gameMode]
	if !ok || len(pool) == 0 {
		pool = soccarMapPool
	}
	var maps []string
	var weights []float64
	for m, w := range pool {
		maps = append(maps, m)
		weights = append(weights, w)
	}
	return weightedChoice(maps, weights)
}

// weightedChoice picks one key weighted by its value (equivalent to Python's
// random.choices with weights).
func weightedChoice(keys []string, weights []float64) string {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := rand.Float64() * total
	acc := 0.0
	for i, w := range weights {
		acc += w
		if r <= acc {
			return keys[i]
		}
	}
	return keys[len(keys)-1]
}
