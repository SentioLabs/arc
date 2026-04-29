package sharesconfig_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/sharesconfig"
)

func TestAddAndFind(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := sharesconfig.Share{ID: "abc", Kind: "local", URL: "http://x", KeyB64Url: "k", EditToken: "t", CreatedAt: time.Now()}
	if err := sharesconfig.Add(s); err != nil {
		t.Fatal(err)
	}
	found, err := sharesconfig.Find("abc")
	if err != nil || found == nil || found.ID != "abc" {
		t.Errorf("unexpected: %+v err=%v", found, err)
	}
}

func TestFileMode0600(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = sharesconfig.Add(sharesconfig.Share{ID: "x", Kind: "local", CreatedAt: time.Now()})
	info, err := os.Stat(filepath.Join(home, ".arc", "shares.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = sharesconfig.Add(sharesconfig.Share{ID: "a", CreatedAt: time.Now()})
	_ = sharesconfig.Add(sharesconfig.Share{ID: "b", CreatedAt: time.Now()})
	_ = sharesconfig.Remove("a")
	f, _ := sharesconfig.Load()
	if len(f.Shares) != 1 || f.Shares[0].ID != "b" {
		t.Errorf("after remove, got %+v", f.Shares)
	}
}
