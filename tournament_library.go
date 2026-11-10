package main

// Tournament library (saved tournaments + templates), ported from the v4 fork's
// tournament_runner.py (tournament_save_to_list, tournament_get_saved_list,
// tournament_load_from_id, tournament_delete_from_list, tournament_save_template,
// tournament_get_templates, tournament_delete_template, tournament_export_to_json,
// tournament_import_from_json).
//
// v4 stored everything in QSettings under two keys (a list of saved tournaments
// and a list of templates). v5 stores each saved tournament as its own JSON file
// under <tournamentDir>/saved/<id>.json and each template under
// <tournamentDir>/templates/<id>.json. The "current" tournament still lives in
// <tournamentDir>/current.json (see tournament.go persistLocked).
//
// v4 auto-synced progress back into the saved-list entry. We reproduce that by
// having persistLocked also rewrite saved/<id>.json whenever it already exists,
// so once a tournament is in the library, match results persist across restarts.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TournamentSummary is the lightweight metadata returned by TournamentListSaved.
// It deliberately omits the heavy full-state payload (matches, stats) so the
// landing page list stays small; the full state is fetched one at a time via
// TournamentLoadFromID.
type TournamentSummary struct {
	TournamentID     string `json:"tournament_id"`
	Name             string `json:"name"`
	Format           string `json:"format"`
	TeamSize         int    `json:"team_size"`
	ParticipantCount int    `json:"participant_count"`
	Completed        bool   `json:"completed"`
	WinnerName       string `json:"winner_name"`
	CreatedAt        string `json:"created_at"`
}

// TournamentTemplate is a saved creation-time configuration that can be reused
// to quickly spin up a similar tournament.
type TournamentTemplate struct {
	TemplateID string           `json:"template_id"`
	Name       string           `json:"name"`
	Config     TournamentConfig `json:"config"`
	CreatedAt  string           `json:"created_at"`
}

func (a *App) savedDir() string {
	return filepath.Join(a.tournamentDir, "saved")
}

func (a *App) templatesDir() string {
	return filepath.Join(a.tournamentDir, "templates")
}

// TournamentSaveToLibrary copies the current tournament into the saved library
// so it appears on the landing page. Idempotent: re-saving updates the entry.
func (a *App) TournamentSaveToLibrary() error {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return errorf("no tournament loaded")
	}
	return a.saveToLibraryLocked()
}

// saveToLibraryLocked writes the current tournament to saved/<id>.json (slim
// wire format). Caller holds the lock.
func (a *App) saveToLibraryLocked() error {
	if a.tournament == nil {
		return errorf("no tournament loaded")
	}
	if err := os.MkdirAll(a.savedDir(), 0o755); err != nil {
		return err
	}
	data, err := encodeTournamentState(a.tournament)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.savedDir(), a.tournament.TournamentID+".json"), data, 0o644)
}

// TournamentListSaved returns metadata for every saved tournament, sorted by
// creation time (newest first).
func (a *App) TournamentListSaved() ([]TournamentSummary, error) {
	entries, err := os.ReadDir(a.savedDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []TournamentSummary{}, nil
		}
		return nil, err
	}
	summaries := make([]TournamentSummary, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.savedDir(), e.Name()))
		if err != nil {
			continue
		}
		s, err := decodeTournamentState(data)
		if err != nil {
			continue
		}
		winner := ""
		if s.WinnerTeam != nil {
			winner = s.WinnerTeam.Name
		} else if s.Winner != nil {
			winner = s.Winner.Name
		}
		summaries = append(summaries, TournamentSummary{
			TournamentID:     s.TournamentID,
			Name:             s.Name,
			Format:           s.Format,
			TeamSize:         s.TeamSize,
			ParticipantCount: len(s.Participants),
			Completed:        s.Completed,
			WinnerName:       winner,
			CreatedAt:        s.CreatedAt,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt > summaries[j].CreatedAt
	})
	return summaries, nil
}

// TournamentLoadFromID loads a saved tournament from the library into the
// current slot (and makes it the active tournament).
func (a *App) TournamentLoadFromID(tournamentID string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	data, err := os.ReadFile(filepath.Join(a.savedDir(), tournamentID+".json"))
	if err != nil {
		return nil, err
	}
	state, err := decodeTournamentState(data)
	if err != nil {
		return nil, err
	}
	a.tournament = state
	return state, nil
}

// TournamentDeleteFromList removes a tournament from the saved library. If it is
// the currently-loaded tournament, that is cleared too.
func (a *App) TournamentDeleteFromList(tournamentID string) error {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if err := os.Remove(filepath.Join(a.savedDir(), tournamentID+".json")); err != nil {
		return err
	}
	if a.tournament != nil && a.tournament.TournamentID == tournamentID {
		a.tournament = nil
		_ = os.Remove(filepath.Join(a.tournamentDir, "current.json"))
	}
	return nil
}

// TournamentSaveCurrentAsTemplate captures the CURRENT tournament's settings
// (format, team size, participants/seeding, map, mutators, etc.) as a reusable
// template, so a user can save a bracket they configured in the bracket view and
// re-run it later. Mirrors v4's "save as template" from the bracket screen.
func (a *App) TournamentSaveCurrentAsTemplate(name string) (*TournamentTemplate, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return nil, errorf("no tournament loaded")
	}
	if name == "" {
		name = a.tournament.Name
	}
	if name == "" {
		return nil, errorf("template name is required")
	}
	s := a.tournament
	inputs := make([]TournamentParticipantInput, 0, len(s.Participants))
	for _, p := range s.Participants {
		inputs = append(inputs, TournamentParticipantInput{
			Name:            p.Name,
			ParticipantID:   p.ParticipantID,
			ParticipantType: p.ParticipantType,
			Player:          p.Player,
			Seed:            p.Seed,
		})
	}
	cfg := TournamentConfig{
		Name:            name,
		Format:          s.Format,
		Participants:    inputs,
		TeamSize:        s.TeamSize,
		AllowDuplicates: s.AllowDuplicates,
		SwissRounds:     s.SwissRounds,
		Map:             s.Map,
		GameMode:        s.GameMode,
		RandomizeMap:    s.RandomizeMap,
		Scripts:         s.Scripts,
		MercyRule:       s.MercyRule,
		Mutators:        s.Mutators,
		ExtraOptions:    s.ExtraOptions,
		Launcher:        s.Launcher,
		LauncherArg:     s.LauncherArg,
	}
	return a.saveTemplateLocked(name, cfg)
}

// TournamentSaveTemplate saves (or replaces, by name) a reusable template.
func (a *App) TournamentSaveTemplate(name string, cfg TournamentConfig) (*TournamentTemplate, error) {
	if name == "" {
		return nil, errorf("template name is required")
	}
	return a.saveTemplateLocked(name, cfg)
}

// saveTemplateLocked upserts a template by name. Caller must NOT assume the
// tournament lock is held (templates are independent of the current tournament).
func (a *App) saveTemplateLocked(name string, cfg TournamentConfig) (*TournamentTemplate, error) {
	templates, err := a.readTemplates()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range templates {
		if equalFold(templates[i].Name, name) {
			templates[i].Config = cfg
			templates[i].CreatedAt = now
			if err := a.writeTemplates(templates); err != nil {
				return nil, err
			}
			return &templates[i], nil
		}
	}
	tmpl := TournamentTemplate{
		TemplateID: newID(),
		Name:       name,
		Config:     cfg,
		CreatedAt:  now,
	}
	templates = append(templates, tmpl)
	if err := a.writeTemplates(templates); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// TournamentListTemplates returns all saved templates, newest first.
func (a *App) TournamentListTemplates() ([]TournamentTemplate, error) {
	templates, err := a.readTemplates()
	if err != nil {
		return nil, err
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].CreatedAt > templates[j].CreatedAt
	})
	return templates, nil
}

// TournamentDeleteTemplate removes a template by id.
func (a *App) TournamentDeleteTemplate(templateID string) error {
	templates, err := a.readTemplates()
	if err != nil {
		return err
	}
	filtered := templates[:0]
	for _, t := range templates {
		if t.TemplateID != templateID {
			filtered = append(filtered, t)
		}
	}
	return a.writeTemplates(filtered)
}

// TournamentExportJSON returns the current tournament as a pretty JSON string
// for file export (slim wire format, same as on-disk saves).
func (a *App) TournamentExportJSON() (string, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	if a.tournament == nil {
		return "", errorf("no tournament loaded")
	}
	sv := a.tournament.toSaved()
	data, err := json.MarshalIndent(sv, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TournamentImportJSON loads a tournament from a JSON string (file import). The
// imported tournament always gets a FRESH id so it can never overwrite an
// existing saved tournament, and an optional newName (when non-empty) renames it
// so it doesn't collide with an existing entry in the landing-page list.
func (a *App) TournamentImportJSON(tournamentJSON string, newName string) (*TournamentState, error) {
	a.tournamentMu.Lock()
	defer a.tournamentMu.Unlock()
	state, err := decodeTournamentState([]byte(tournamentJSON))
	if err != nil {
		return nil, errorf("failed to import tournament: %w", err)
	}
	// Always assign a fresh id: importing must never overwrite an existing
	// saved tournament (the library is keyed by id).
	state.TournamentID = newID()
	if name := strings.TrimSpace(newName); name != "" {
		state.Name = name
	}
	a.tournament = state
	a.persistLocked()
	// Register in the library so it shows on the landing page under its (new) id.
	_ = a.saveToLibraryLocked()
	return state, nil
}

// TournamentImportTemplateJSON loads a template from a JSON string and adds it to
// the templates list (mirrors v4's tournament_import_template). Accepts either a
// full TournamentTemplate object (as produced by TournamentExportJSON of a saved
// tournament, or a prior template export) or a bare TournamentConfig, which is
// wrapped with a generated id. Replaces an existing template of the same name.
func (a *App) TournamentImportTemplateJSON(templateJSON string) (*TournamentTemplate, error) {
	// Prefer a full template object (has a populated config.name).
	var tmpl TournamentTemplate
	if err := json.Unmarshal([]byte(templateJSON), &tmpl); err == nil && tmpl.Config.Name != "" {
		if tmpl.TemplateID == "" {
			tmpl.TemplateID = newID()
		}
		if tmpl.CreatedAt == "" {
			tmpl.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		templates, err := a.readTemplates()
		if err != nil {
			return nil, err
		}
		replaced := false
		for i := range templates {
			if equalFold(templates[i].Name, tmpl.Name) {
				templates[i] = tmpl
				replaced = true
				break
			}
		}
		if !replaced {
			templates = append(templates, tmpl)
		}
		if err := a.writeTemplates(templates); err != nil {
			return nil, err
		}
		return &tmpl, nil
	}
	// Fall back to a bare config.
	var cfg TournamentConfig
	if err := json.Unmarshal([]byte(templateJSON), &cfg); err != nil {
		return nil, errorf("failed to parse template: %w", err)
	}
	if cfg.Name == "" {
		return nil, errorf("template JSON had no name")
	}
	return a.TournamentSaveTemplate(cfg.Name, cfg)
}

// TournamentExportTemplateJSON returns a saved template as a pretty JSON string
// for file export.
func (a *App) TournamentExportTemplateJSON(templateID string) (string, error) {
	templates, err := a.readTemplates()
	if err != nil {
		return "", err
	}
	for _, t := range templates {
		if t.TemplateID == templateID {
			data, err := json.MarshalIndent(t, "", "  ")
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}
	return "", errorf("template %s not found", templateID)
}

// --- template file helpers ---

func (a *App) readTemplates() ([]TournamentTemplate, error) {
	entries, err := os.ReadDir(a.templatesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []TournamentTemplate{}, nil
		}
		return nil, err
	}
	templates := make([]TournamentTemplate, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.templatesDir(), e.Name()))
		if err != nil {
			continue
		}
		var t TournamentTemplate
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (a *App) writeTemplates(templates []TournamentTemplate) error {
	if err := os.MkdirAll(a.templatesDir(), 0o755); err != nil {
		return err
	}
	// Rewrite the whole set: clear existing files, then write each.
	if entries, err := os.ReadDir(a.templatesDir()); err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				_ = os.Remove(filepath.Join(a.templatesDir(), e.Name()))
			}
		}
	}
	for _, t := range templates {
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(a.templatesDir(), t.TemplateID+".json"), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// equalFold is a tiny case-insensitive string compare (avoids importing
// strings just for this).
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
