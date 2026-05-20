package httpserver

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	authpkg "agentbackend/internal/auth"
	"agentbackend/internal/game"
)

type liveHub struct {
	mu           sync.Mutex
	clients      map[int64]map[*liveClient]bool
	lobbyClients map[*lobbyLiveClient]bool
}

type liveClient struct {
	gameID int64
	userID int64
	conn   *websocket.Conn
	mu     sync.Mutex
}

type lobbyLiveClient struct {
	userID int64
	conn   *websocket.Conn
	mu     sync.Mutex
}

type liveMessage struct {
	Type  string                `json:"type"`
	State *game.PublicGameState `json:"state,omitempty"`
	Games []gameListItem        `json:"games,omitempty"`
}

func newLiveHub() *liveHub {
	return &liveHub{
		clients:      map[int64]map[*liveClient]bool{},
		lobbyClients: map[*lobbyLiveClient]bool{},
	}
}

func (h *liveHub) add(client *liveClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.gameID] == nil {
		h.clients[client.gameID] = map[*liveClient]bool{}
	}
	h.clients[client.gameID][client] = true
}

func (h *liveHub) remove(client *liveClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.gameID] == nil {
		return
	}
	delete(h.clients[client.gameID], client)
	if len(h.clients[client.gameID]) == 0 {
		delete(h.clients, client.gameID)
	}
}

func (h *liveHub) snapshot(gameID int64) []*liveClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[gameID]
	out := make([]*liveClient, 0, len(clients))
	for client := range clients {
		out = append(out, client)
	}
	return out
}

func (h *liveHub) addLobby(client *lobbyLiveClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lobbyClients[client] = true
}

func (h *liveHub) removeLobby(client *lobbyLiveClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.lobbyClients, client)
}

func (h *liveHub) lobbySnapshot() []*lobbyLiveClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*lobbyLiveClient, 0, len(h.lobbyClients))
	for client := range h.lobbyClients {
		out = append(out, client)
	}
	return out
}

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleGamesWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	claims, err := authpkg.ParseToken(s.jwtSecret, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
		return
	}
	if _, err := s.store.GetUserByID(r.Context(), claims.UserID); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
		return
	}

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &lobbyLiveClient{userID: claims.UserID, conn: conn}
	s.live.addLobby(client)
	defer func() {
		s.live.removeLobby(client)
		_ = conn.Close()
	}()

	s.writeGamesToClient(r.Context(), client)
	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (s *Server) handleGameWebSocket(w http.ResponseWriter, r *http.Request) {
	gameID, err := readIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid game id"})
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	claims, err := authpkg.ParseToken(s.jwtSecret, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
		return
	}
	if _, err := s.store.GetUserByID(r.Context(), claims.UserID); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or expired token"})
		return
	}

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &liveClient{gameID: gameID, userID: claims.UserID, conn: conn}
	s.live.add(client)
	defer func() {
		s.live.remove(client)
		_ = conn.Close()
	}()

	s.writeGameStateToClient(r.Context(), client)
	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (s *Server) broadcastGameState(ctx context.Context, gameID int64) {
	for _, client := range s.live.snapshot(gameID) {
		if err := s.writeGameStateToClient(ctx, client); err != nil {
			s.live.remove(client)
			_ = client.conn.Close()
		}
	}
}

func (s *Server) broadcastGames(ctx context.Context) {
	for _, client := range s.live.lobbySnapshot() {
		if err := s.writeGamesToClient(ctx, client); err != nil {
			s.live.removeLobby(client)
			_ = client.conn.Close()
		}
	}
}

func (s *Server) writeGameStateToClient(ctx context.Context, client *liveClient) error {
	state, err := s.publicStateForViewer(ctx, client.gameID, client.userID)
	if err != nil {
		return err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	_ = client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return client.conn.WriteJSON(liveMessage{Type: "state", State: state})
}

func (s *Server) writeGamesToClient(ctx context.Context, client *lobbyLiveClient) error {
	games, err := s.listGameItems(ctx, client.userID)
	if err != nil {
		return err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	_ = client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return client.conn.WriteJSON(liveMessage{Type: "games", Games: games})
}

func (s *Server) publicStateForViewer(ctx context.Context, gameID int64, viewerID int64) (*game.PublicGameState, error) {
	if _, err := s.engine.AdvanceGame(ctx, gameID, time.Now().UTC()); err != nil {
		return nil, err
	}

	gameModel, err := s.store.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEventsByGameID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	state, err := game.BuildState(gameID, gameModel.Title, events)
	if err != nil {
		return nil, err
	}
	publicState, err := game.ProjectStateForViewer(state, viewerID)
	if err != nil {
		return nil, err
	}
	s.decoratePublicState(ctx, publicState)
	return publicState, nil
}
