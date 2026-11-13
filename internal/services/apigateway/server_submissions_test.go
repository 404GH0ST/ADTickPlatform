package apigateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type submissionTestCore struct {
	noopGameCoreClient
	query   SubmissionQuery
	deleted int64
	err     error
}

func (c *submissionTestCore) Submissions(_ context.Context, query SubmissionQuery) (SubmissionPage, error) {
	c.query = query
	return SubmissionPage{Items: []AdminSubmission{}, Limit: query.Limit}, c.err
}
func (c *submissionTestCore) DeleteSubmission(_ context.Context, id int64) (SubmissionDeletion, error) {
	c.deleted = id
	return SubmissionDeletion{ID: id, Status: "accepted", Service: "banking", FirstBloodCleared: true}, c.err
}

func TestAdminSubmissionsRequiresAuthAndForwardsFiltersAndDeletion(t *testing.T) {
	core := &submissionTestCore{}
	mux := newTestMuxWithGameCore(core)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		path := "/api/v2/admin/submissions"
		if method == http.MethodDelete {
			path += "/1"
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != 403 {
			t.Fatalf("unauthorized %s returned %d", method, response.Code)
		}
	}
	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer dev-admin-token")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	if res := call(http.MethodGet, "/api/v2/admin/submissions?attacker=Alpha&status=duplicate&flag=literal_flag&tick_from=4&tick_to=6&limit=50&offset=10"); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if core.query.Attacker != "Alpha" || core.query.Status != "duplicate" || core.query.Flag != "literal_flag" || core.query.TickFrom != 4 || core.query.TickTo != 6 || core.query.Limit != 50 || core.query.Offset != 10 {
		t.Fatalf("incorrect filters: %+v", core.query)
	}
	if res := call(http.MethodDelete, "/api/v2/admin/submissions/31"); res.Code != 200 || core.deleted != 31 {
		t.Fatalf("delete failed: %d %s", res.Code, res.Body.String())
	}
	core.err = gameCoreHTTPError{StatusCode: 404, Message: "submission not found"}
	if res := call(http.MethodDelete, "/api/v2/admin/submissions/31"); res.Code != 404 {
		t.Fatalf("missing submission returned %d", res.Code)
	}
	if res := call(http.MethodDelete, "/api/v2/admin/submissions/not-an-id"); res.Code != 400 {
		t.Fatalf("invalid id returned %d", res.Code)
	}
}
