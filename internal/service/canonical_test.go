package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// The API may answer with a different (canonical) ref than the one asked for;
// the item must still be cached under the requested ref, or it never hits.
func TestItemCachedUnderRequestedRef(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(fixture(t, "verse-bg-2-13.json")) // ref bg/2/13
	}))
	defer srv.Close()
	svc := &Service{
		Client: api.NewClient(srv.URL, "test", nil),
		Items:  store.ItemCache{Dir: filepath.Join(t.TempDir(), "items")},
		Base:   srv.URL,
	}
	ctx := context.Background()
	if _, _, err := svc.Item(ctx, "bg/2/13-14"); err != nil {
		t.Fatal(err)
	}
	if !svc.Cached("bg/2/13-14") {
		t.Fatal("not cached under the requested ref")
	}
	_, cached, err := svc.Item(ctx, "bg/2/13-14")
	if err != nil || !cached || hits != 1 {
		t.Fatalf("second: cached=%v err=%v hits=%d", cached, err, hits)
	}
}
