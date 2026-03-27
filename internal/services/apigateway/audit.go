package apigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
)

func (s *Server) recordAdminAudit(ctx context.Context, action, targetType, target, message string, metadata map[string]any) {
	s.recordAudit(ctx, adminAuditLogEntry{
		ActorType:  "admin",
		Actor:      "organizer",
		Action:     action,
		TargetType: targetType,
		Target:     target,
		Status:     "success",
		Message:    message,
		Metadata:   marshalAuditMetadata(metadata),
	})
}

func (s *Server) recordTeamAudit(ctx context.Context, teamID int, action, targetType, target, message string, metadata map[string]any) {
	s.recordAudit(ctx, adminAuditLogEntry{
		ActorType:  "team",
		Actor:      fmt.Sprintf("team:%d", teamID),
		Action:     action,
		TargetType: targetType,
		Target:     target,
		Status:     "success",
		Message:    message,
		Metadata:   marshalAuditMetadata(metadata),
	})
}

func (s *Server) recordAudit(ctx context.Context, entry adminAuditLogEntry) {
	if err := s.store.AppendAdminAuditLog(ctx, entry); err != nil {
		log.Printf("audit log append failed for action %q on %q: %v", entry.Action, entry.Target, err)
	}
}

func marshalAuditMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ordered := make(map[string]any, len(keys))
	for _, key := range keys {
		ordered[key] = metadata[key]
	}

	encoded, err := json.Marshal(ordered)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func auditServiceTarget(teamID, challengeID int) string {
	return fmt.Sprintf("team:%d challenge:%d", teamID, challengeID)
}

func auditChallengeTarget(challengeID int, name string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return fmt.Sprintf("challenge:%d %s", challengeID, trimmed)
	}
	return fmt.Sprintf("challenge:%d", challengeID)
}
