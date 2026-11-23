package apigateway

import "testing"

func TestPaginateAttackFeedKeepsFirstBloodsOutsideThePage(t *testing.T) {
	events := []attackEvent{
		{ID: "newest", Service: "web"},
		{ID: "second", Service: "pwn", FirstBlood: true},
		{ID: "ordinary", Service: "web"},
		{ID: "first", Service: "web", FirstBlood: true},
	}

	page := paginateAttackFeed(events, AttackFeedQuery{Limit: 1, Service: "web"})

	if len(page.Items) != 1 || page.Items[0].ID != "newest" {
		t.Fatalf("unexpected page: %+v", page.Items)
	}
	if len(page.FirstBloods) != 2 || page.FirstBloods[0].ID != "first" || page.FirstBloods[1].ID != "second" {
		t.Fatalf("expected every first blood in capture order: %+v", page.FirstBloods)
	}
}

func TestFirstBloodsInCaptureOrderIsNeverNil(t *testing.T) {
	if got := FirstBloodsInCaptureOrder(nil); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty list, got %#v", got)
	}
}
