package apigateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicGameStatusFailureHidesUpstreamDetail(t *testing.T) {
	secret := `Get "http://game-core:8081/internal/v1/game/status": dial tcp 172.20.0.5:8081`
	for name, err := range map[string]error{
		"transport":     errors.New(secret),
		"upstream body": gameCoreHTTPError{StatusCode: http.StatusInternalServerError, Message: secret},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writePublicGameStatusFailure(recorder, err)
			if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "game-core:8081") || strings.Contains(recorder.Body.String(), "172.20") {
				t.Fatalf("leaked upstream detail: %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	recorder := httptest.NewRecorder()
	writePublicGameStatusFailure(recorder, errGameCoreDisabled)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled game-core: %d", recorder.Code)
	}
}
