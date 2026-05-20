package httpserver

import (
	"context"
	"log"
	"time"

	"agentbackend/internal/game"
)

func (s *Server) runMaintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastCleanup := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.advanceStartedGames(ctx, now.UTC())
			if lastCleanup.IsZero() || now.Sub(lastCleanup) >= 30*time.Second {
				if s.cleanupExpiredLobbies(ctx, now.UTC()) {
					s.broadcastGames(ctx)
				}
				lastCleanup = now
			}
		}
	}
}

func (s *Server) advanceStartedGames(ctx context.Context, now time.Time) {
	games, err := s.store.ListGames(ctx)
	if err != nil {
		log.Printf("maintenance list games: %v", err)
		return
	}
	for _, gameModel := range games {
		changed, err := s.engine.AdvanceGame(ctx, gameModel.ID, now)
		if err != nil {
			log.Printf("maintenance advance game %d: %v", gameModel.ID, err)
			continue
		}
		if changed {
			s.broadcastGameState(ctx, gameModel.ID)
			s.broadcastGames(ctx)
		}
	}
}

func (s *Server) cleanupExpiredLobbies(ctx context.Context, now time.Time) bool {
	games, err := s.store.ListGames(ctx)
	if err != nil {
		log.Printf("maintenance list games for cleanup: %v", err)
		return false
	}
	deleted := false
	for _, gameModel := range games {
		if gameModel.CreatedAt.IsZero() || now.Sub(gameModel.CreatedAt) <= time.Hour {
			continue
		}
		events, err := s.store.ListEventsByGameID(ctx, gameModel.ID)
		if err != nil {
			log.Printf("maintenance list events for game %d: %v", gameModel.ID, err)
			continue
		}
		state, err := game.BuildState(gameModel.ID, gameModel.Title, events)
		if err != nil {
			log.Printf("maintenance build state for game %d: %v", gameModel.ID, err)
			continue
		}
		if state.Status != game.GameStatusLobby {
			continue
		}
		if err := s.store.DeleteGame(ctx, gameModel.ID); err != nil {
			log.Printf("maintenance delete lobby %d: %v", gameModel.ID, err)
			continue
		}
		deleted = true
	}
	return deleted
}
