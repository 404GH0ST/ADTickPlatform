package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestRotationWaitsForRetentionPermit(t *testing.T) {
	for _, permitWrite := range []bool{true, false} {
		name := "acknowledged"
		if !permitWrite {
			name = "disconnected"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sentinel := filepath.Join(dir, "put-ran")
			t.Setenv("ROTATION_PUT_MARKER", sentinel)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			docker := filepath.Join(dir, "docker")
			checker := filepath.Join(dir, "checker")
			if err := os.WriteFile(docker, []byte(`#!/bin/sh
[ "$1" = rm ] && exit 0
for arg in "$@"; do
  last="$arg"
  case "$arg" in AD_PHASES=*|AD_TICK_ID=*) export "$arg";; esac
done
exec /bin/sh -c "$last"
`), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(checker, []byte(`#!/bin/sh
case "$AD_PHASE:$AD_TICK_ID:$AD_FLAG:$AD_METADATA" in
  get:18:previous:old-meta) exit 42;;
  put:19:next:) touch "$ROTATION_PUT_MARKER"; printf 'put-meta';;
  get:19:next:put-meta) printf 'readback-meta';;
  *) exit 1;;
esac
`), 0o700); err != nil {
				t.Fatal(err)
			}
			runner := newCheckerRunnerServer("token", &dockerCheckerExecutor{
				binary: docker, network: "service", timeout: 2 * time.Second,
			})
			mux := http.NewServeMux()
			runner.RegisterRoutes(mux)
			server := httptest.NewServer(mux)
			defer server.Close()

			body, send := io.Pipe()
			defer send.Close()
			go func() {
				_ = json.NewEncoder(send).Encode(apigateway.CheckerRotationRequest{
					CheckerBatchExecutionRequest: apigateway.CheckerBatchExecutionRequest{
						ChallengeID: 7, TeamID: 101, Target: "10.80.7.11:10007",
						TickID: 19, CheckerImage: "checker", Flag: "next",
					},
					PreviousTickID: 18, PreviousFlag: "previous", PreviousMetadata: "old-meta",
				})
			}()
			req, err := http.NewRequest(http.MethodPost, server.URL+"/internal/v1/checkers/rotate", body)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer token")
			client := server.Client()
			client.Timeout = 6 * time.Second
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			events := json.NewDecoder(resp.Body)
			nextEvent := func(kind string) apigateway.CheckerPhaseResult {
				t.Helper()
				var event apigateway.CheckerRotationEvent
				if err := events.Decode(&event); err != nil || event.Kind != kind {
					t.Fatalf("wanted %q event, got %+v, %v (HTTP %d)", kind, event, err, resp.StatusCode)
				}
				return event.Result
			}
			if old := nextEvent("retention"); old.Status != "failed" || old.ExitCode != 42 {
				t.Fatalf("incorrect previous-flag verdict: %+v", old)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("put ran before permission: %v", err)
			}
			if !permitWrite {
				_ = send.Close()
				if _, err := io.Copy(io.Discard, resp.Body); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
					t.Fatalf("put ran after disconnect before permission: %v", err)
				}
				return
			}
			if _, err := io.WriteString(send, "true\n"); err != nil {
				t.Fatal(err)
			}
			_ = send.Close()
			if put := nextEvent("phase"); put.Phase != "put" || put.Output != "put-meta" {
				t.Fatalf("incorrect put verdict: %+v", put)
			}
			if get := nextEvent("phase"); get.Phase != "get" || get.Output != "readback-meta" {
				t.Fatalf("incorrect new-flag verdict: %+v", get)
			}
			nextEvent("done")
			if _, err := os.Stat(sentinel); err != nil {
				t.Fatalf("put failed to run after permission: %v", err)
			}
		})
	}
}
