package browse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"stash-vr/internal/library"
)

// stubBuilder lets a test substitute the heavy GraphQL work so we can
// assert version/hash/TTL behavior without touching Stash.
type stubBuilder struct {
	catalogCalls int32
	matrixCalls  int32
	catalog      filterIndexCatalog
	matrixResp   filterMatrixSeeds
}

func (s *stubBuilder) BuildCatalog(_ context.Context, _ graphql.Client) (filterIndexCatalog, error) {
	atomic.AddInt32(&s.catalogCalls, 1)
	return s.catalog, nil
}

func (s *stubBuilder) BuildMatrix(_ context.Context, _ graphql.Client) (filterMatrixSeeds, error) {
	atomic.AddInt32(&s.matrixCalls, 1)
	return s.matrixResp, nil
}

func newTestCatalog() filterIndexCatalog {
	return filterIndexCatalog{
		sidebar: SidebarData{
			Performers: []Entity{{ID: "p1", Name: "Alpha"}},
			Studios:    []Entity{{ID: "s1", Name: "S"}},
			Tags:       []Entity{{ID: "t1", Name: "T"}},
		},
		selectableStudioIDs: map[string]struct{}{"s1": {}},
		selectableTagIDs:    map[string]struct{}{"t1": {}},
	}
}

func newTestMatrix() filterMatrixSeeds {
	return filterMatrixSeeds{
		Scenes: []facetSceneSeed{
			{ID: "sc1", PerformerIDs: []string{"p1"}, StudioID: "s1", TagIDs: []string{"t1"}, Rating100: 80, OCount: 2},
		},
	}
}

func TestFilterCache_CatalogBuildsOnceWithinTTL(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour)

	_, etag1, err := cache.Catalog(context.Background(), nil)
	if err != nil {
		t.Fatalf("Catalog(1) err = %v", err)
	}
	_, etag2, err := cache.Catalog(context.Background(), nil)
	if err != nil {
		t.Fatalf("Catalog(2) err = %v", err)
	}
	if etag1 != etag2 {
		t.Fatalf("etag changed within TTL: %s vs %s", etag1, etag2)
	}
	if got := atomic.LoadInt32(&stub.catalogCalls); got != 1 {
		t.Fatalf("catalog builder calls = %d, want 1", got)
	}
}

func TestFilterCache_CatalogRebuildsAfterTTL(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 10*time.Millisecond)

	_, etag1, _ := cache.Catalog(context.Background(), nil)
	time.Sleep(20 * time.Millisecond)
	_, etag2, _ := cache.Catalog(context.Background(), nil)
	// Content identical → version stays the same → ETag stable.
	if etag1 != etag2 {
		t.Fatalf("etag changed despite identical content: %s vs %s", etag1, etag2)
	}
	if got := atomic.LoadInt32(&stub.catalogCalls); got != 2 {
		t.Fatalf("catalog builder calls = %d, want 2 (rebuild after TTL)", got)
	}
}

func TestFilterCache_VersionBumpsOnContentChange(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 10*time.Millisecond)

	_, etag1, _ := cache.Catalog(context.Background(), nil)
	// Mutate the stub's returned catalog so the next build sees new content.
	stub.catalog.sidebar.Performers = append(stub.catalog.sidebar.Performers, Entity{ID: "p2", Name: "Bravo"})
	time.Sleep(20 * time.Millisecond)
	_, etag2, _ := cache.Catalog(context.Background(), nil)
	if etag1 == etag2 {
		t.Fatalf("etag did not change despite content change: %s", etag1)
	}
}

func TestFilterCache_MatrixUsesCachedCatalogParents(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour)

	if _, _, err := cache.Matrix(context.Background(), nil); err != nil {
		t.Fatalf("Matrix err = %v", err)
	}
	// Matrix must implicitly produce or reuse a catalog (for parent expansion).
	if atomic.LoadInt32(&stub.catalogCalls) < 1 {
		t.Fatalf("matrix did not trigger catalog build; calls=%d", atomic.LoadInt32(&stub.catalogCalls))
	}
	if atomic.LoadInt32(&stub.matrixCalls) != 1 {
		t.Fatalf("matrix calls = %d, want 1", atomic.LoadInt32(&stub.matrixCalls))
	}
}

func TestFilterCatalogHandler304(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	h := &httpHandler{
		libraryService: &library.Service{},
		filterCache:    newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour),
	}

	// First request: expect 200 with ETag header.
	req1 := httptest.NewRequest(http.MethodGet, "/browse/filter-catalog", nil)
	rec1 := httptest.NewRecorder()
	h.filterCatalogHandler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	// Second request with If-None-Match: expect 304 and empty body.
	req2 := httptest.NewRequest(http.MethodGet, "/browse/filter-catalog", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.filterCatalogHandler(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("revalidated request status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 response body should be empty, got %d bytes", rec2.Body.Len())
	}
}
