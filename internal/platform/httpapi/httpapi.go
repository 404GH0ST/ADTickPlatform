package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxJSONBodyBytes int64 = 1 << 20

type ServiceInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Addr    string `json:"addr"`
}

type ErrorEnvelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ProblemDetails struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func NewBaseMux(info ServiceInfo) *http.ServeMux {
	mux := http.NewServeMux()
	metrics := lookupServiceMetrics(info)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": info.Name,
		})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ready",
			"service": info.Name,
		})
	})

	mux.HandleFunc("GET /metadata", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, info)
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics.writePrometheus(w)
	})

	return mux
}

func WriteJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteProblem(w http.ResponseWriter, statusCode int, problem ProblemDetails) {
	if problem.Title == "" {
		problem.Title = http.StatusText(statusCode)
	}
	if problem.Status == 0 {
		problem.Status = statusCode
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(problem)
}

func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("unexpected trailing json")
	}

	return nil
}

func BearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}

func RunServer(ctx context.Context, info ServiceInfo, handler http.Handler) error {
	server := &http.Server{
		Addr:              info.Addr,
		Handler:           instrumentHandler(info, handler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
