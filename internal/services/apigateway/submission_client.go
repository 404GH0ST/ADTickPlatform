package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
)

var errSubmissionServiceDisabled = errors.New("submission service is not configured")

type submissionClient interface {
	SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error)
	AttackFeed(ctx context.Context, query AttackFeedQuery) (AttackFeedPage, error)
}

type noopSubmissionClient struct{}

func (noopSubmissionClient) SubmitFlags(context.Context, int, []string) ([]submissionVerdict, error) {
	return nil, errSubmissionServiceDisabled
}

func (noopSubmissionClient) AttackFeed(context.Context, AttackFeedQuery) (AttackFeedPage, error) {
	return AttackFeedPage{}, errSubmissionServiceDisabled
}

type httpSubmissionClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPSubmissionClient(baseURL, token string) submissionClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		return noopSubmissionClient{}
	}
	return &httpSubmissionClient{
		baseURL: normalizedBaseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpSubmissionClient) SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error) {
	return requestSubmissionJSON[[]submissionVerdict](ctx, c, http.MethodPost, "/internal/v1/submissions/submit", GameSubmitFlagsRequest{
		TeamID: teamID,
		Flags:  flags,
	})
}

func (c *httpSubmissionClient) AttackFeed(ctx context.Context, query AttackFeedQuery) (AttackFeedPage, error) {
	path := "/internal/v1/submissions/attacks"
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Offset > 0 {
		values.Set("offset", strconv.Itoa(query.Offset))
	}
	if query.Attacker != "" {
		values.Set("attacker", query.Attacker)
	}
	if query.Victim != "" {
		values.Set("victim", query.Victim)
	}
	if query.Service != "" {
		values.Set("service", query.Service)
	}
	if query.TickFrom > 0 {
		values.Set("tick_from", strconv.Itoa(query.TickFrom))
	}
	if query.TickTo > 0 {
		values.Set("tick_to", strconv.Itoa(query.TickTo))
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return requestSubmissionJSON[AttackFeedPage](ctx, c, http.MethodGet, path)
}

func requestSubmissionJSON[T any](ctx context.Context, c *httpSubmissionClient, method, path string, body ...any) (T, error) {
	var zero T
	var requestBody io.Reader

	if len(body) > 0 && body[0] != nil {
		encodedBody, err := json.Marshal(body[0])
		if err != nil {
			return zero, err
		}
		requestBody = strings.NewReader(string(encodedBody))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return zero, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal submission service origin.
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return decodeSubmissionSuccess[T](resp.Body)
	}

	message, err := decodeSubmissionError(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("submission-service request failed with status %d", resp.StatusCode)
	}
	if message != "" {
		return zero, fmt.Errorf("submission-service request failed: %s", message)
	}
	return zero, fmt.Errorf("submission-service request failed with status %d", resp.StatusCode)
}

func decodeSubmissionSuccess[T any](body io.Reader) (T, error) {
	var zero T
	if err := json.NewDecoder(body).Decode(&zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func decodeSubmissionError(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(data, &problem); err == nil {
		if trimmed := strings.TrimSpace(problem.Detail); trimmed != "" {
			return trimmed, nil
		}
		if trimmed := strings.TrimSpace(problem.Title); trimmed != "" {
			return trimmed, nil
		}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Message), nil
}
