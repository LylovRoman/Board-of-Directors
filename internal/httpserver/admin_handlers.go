package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"agentbackend/internal/game"
)

func (s *Server) handleBotGameSimulation(w http.ResponseWriter, r *http.Request) {
	if s.adminAPIToken == "" || r.Header.Get("X-Admin-Token") != s.adminAPIToken {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "admin token required"})
		return
	}

	var request game.BotSimulationRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	response, err := game.SimulateBotGames(request)
	if err != nil {
		writeJSON(w, statusFromError(err), errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}
