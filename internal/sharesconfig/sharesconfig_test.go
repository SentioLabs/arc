package sharesconfig

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

type fakeClient struct {
	store map[string]*types.Share
}

func newFakeClient() *fakeClient {
	return &fakeClient{store: map[string]*types.Share{}}
}

func (f *fakeClient) ListShares() ([]*types.Share, error) {
	out := make([]*types.Share, 0, len(f.store))
	for _, s := range f.store {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeClient) GetShare(id string) (*types.Share, error) {
	s, ok := f.store[id]
	if !ok {
		return nil, client.ErrShareNotFound
	}
	return s, nil
}

func (f *fakeClient) UpsertShare(s *types.Share) (*types.Share, error) {
	f.store[s.ID] = s
	return s, nil
}

func (f *fakeClient) DeleteShare(id string) error {
	delete(f.store, id)
	return nil
}

func withFake(t *testing.T) *fakeClient {
	t.Helper()
	fake := newFakeClient()
	SetClientFactory(func() (Client, error) { return fake, nil })
	t.Cleanup(func() { SetClientFactory(nil) })
	return fake
}

func TestAddAndFind(t *testing.T) {
	withFake(t)
	s := Share{ID: "x", Kind: "local", URL: "u", KeyB64Url: "k", EditToken: "t", CreatedAt: time.Now()}
	if err := Add(s); err != nil {
		t.Fatalf("add: %v", err)
	}
	got, err := Find("x")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != "x" || got.URL != "u" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestFindNotFound(t *testing.T) {
	withFake(t)
	_, err := Find("missing")
	if !errors.Is(err, ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestLoadEmpty(t *testing.T) {
	withFake(t)
	f, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(f.Shares) != 0 {
		t.Errorf("expected empty, got %d", len(f.Shares))
	}
}

func TestRemove(t *testing.T) {
	fake := withFake(t)
	fake.store["x"] = &types.Share{ID: "x"}
	if err := Remove("x"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := fake.store["x"]; ok {
		t.Errorf("expected entry removed")
	}
}

func TestLegacyPath(t *testing.T) {
	p, err := LegacyPath()
	if err != nil {
		t.Fatalf("legacy path: %v", err)
	}
	if p == "" || !strings.HasSuffix(p, "/.arc/shares.json") {
		t.Errorf("unexpected legacy path: %s", p)
	}
}

func TestNoFactorySet(t *testing.T) {
	SetClientFactory(nil)
	_, err := Load()
	if err == nil {
		t.Error("expected error when factory not set, got nil")
	}
}
