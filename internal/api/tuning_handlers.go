package api

import (
	"encoding/json"
	"net/http"

	"github.com/z-ghostshell/ghostvault/internal/config"
)

func (s *Server) getTuning(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, _, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	eff, _, _, dbRaw, sources := s.Tuning.APISnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"tuning":      config.MergedAPIMap(eff),
		"sources":     sources,
		"tuning_file": s.Cfg.TuningFile,
		"overrides":   json.RawMessage(dbRaw),
	})
}

func (s *Server) postTuningReload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, _, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	if err := s.Tuning.Reload(ctx); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Reload failed", err.Error(), "tuning_reload")
		return
	}
	eff, _, _, dbRaw, sources := s.Tuning.APISnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"tuning":      config.MergedAPIMap(eff),
		"sources":     sources,
		"tuning_file": s.Cfg.TuningFile,
		"overrides":   json.RawMessage(dbRaw),
		"reloaded":    true,
	})
}

func (s *Server) patchTuning(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, _, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error(), "json")
		return
	}
	if len(patch) == 0 {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "empty patch", "validate")
		return
	}
	existing, err := s.Store.GetServerTuning(ctx)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Store failed", err.Error(), "store")
		return
	}
	merged, err := config.MergeTuningOverrideJSON(existing, patch)
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error(), "validate")
		return
	}
	if _, _, err := config.MergeTuningFull(config.DefaultRuntimeTuning(), s.Cfg.TuningFile, merged); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Invalid tuning", err.Error(), "validate")
		return
	}
	if err := s.Store.SetServerTuningOverrides(ctx, merged); err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Store failed", err.Error(), "store")
		return
	}
	if err := s.Tuning.Reload(ctx); err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Reload failed", err.Error(), "tuning_reload")
		return
	}
	eff, _, _, dbRaw, sources := s.Tuning.APISnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"tuning":      config.MergedAPIMap(eff),
		"sources":     sources,
		"tuning_file": s.Cfg.TuningFile,
		"overrides":   json.RawMessage(dbRaw),
		"updated":     true,
	})
}
