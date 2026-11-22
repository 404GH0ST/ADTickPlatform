package apigateway

import (
	"context"
	"net/http"
	"time"

	"adplatform/internal/platform/httpapi"
)

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	token, _ := httpapi.BearerToken(r)
	claims, err := verifyTeamJWT(s.teamTokenSecret, token, now)
	if err == nil {
		if err := s.store.RevokePlayerSession(r.Context(), claims.SessionID, time.Unix(claims.ExpiresAt, 0), now); err != nil {
			writeStoreFailure(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *postgresStore) IsPlayerSessionRevoked(ctx context.Context, sessionID string) (bool, error) {
	return s.exists(ctx, `SELECT EXISTS (SELECT 1 FROM revoked_player_sessions WHERE session_id = $1)`, sessionID)
}

func (s *postgresStore) RevokePlayerSession(ctx context.Context, sessionID string, expires, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		WITH expired AS (DELETE FROM revoked_player_sessions WHERE expires_at <= $3)
		INSERT INTO revoked_player_sessions (session_id, expires_at) VALUES ($1, $2)
		ON CONFLICT (session_id) DO NOTHING
	`, sessionID, expires, now)
	return err
}

func (s *memoryStore) IsPlayerSessionRevoked(_ context.Context, sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, revoked := s.revokedSessions[sessionID]
	return revoked, nil
}

func (s *memoryStore) RevokePlayerSession(_ context.Context, sessionID string, expires, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revokedSessions == nil {
		s.revokedSessions = make(map[string]time.Time)
	}
	for id, expiration := range s.revokedSessions {
		if !expiration.After(now) {
			delete(s.revokedSessions, id)
		}
	}
	s.revokedSessions[sessionID] = expires
	return nil
}
