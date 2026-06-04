package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddSource_MergeAndIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	seed := "cache: {}\nproviders:\n  - name: twitter\n    enabled: true\n    config:\n      accounts: [msdev]\n"
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	// Append to existing twitter block.
	if added, err := AddSource(path, "twitter", "karpathy", "", ""); err != nil || !added {
		t.Fatalf("append twitter: added=%v err=%v", added, err)
	}
	// Idempotent (exact + case-insensitive).
	if added, _ := AddSource(path, "twitter", "karpathy", "", ""); added {
		t.Fatal("expected duplicate for karpathy")
	}
	if added, _ := AddSource(path, "twitter", "MSDEV", "", ""); added {
		t.Fatal("expected case-insensitive duplicate for msdev")
	}

	// Create new reddit + rssblog blocks.
	if added, _ := AddSource(path, "reddit", "golang", "", ""); !added {
		t.Fatal("expected new reddit block")
	}
	if added, _ := AddSource(path, "rssblog", "", "https://ex.com/feed.xml", "ex"); !added {
		t.Fatal("expected new rssblog feed")
	}
	// rssblog dedup by URL.
	if added, _ := AddSource(path, "rssblog", "", "https://ex.com/feed.xml", "ex"); added {
		t.Fatal("expected duplicate rssblog feed")
	}

	// Result must still be valid and have 3 providers.
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load after edits: %v", err)
	}
	if len(cfg.Providers) != 3 {
		t.Fatalf("want 3 providers, got %d", len(cfg.Providers))
	}
}

func TestAddSource_CreatesProvidersSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("cache: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No providers key at all -> AddSource should create it.
	if added, err := AddSource(path, "hackernews", "", "", ""); err != nil || !added {
		t.Fatalf("add hackernews: added=%v err=%v", added, err)
	}
	// hackernews is a singleton -> re-add is a no-op.
	if added, _ := AddSource(path, "hackernews", "", "", ""); added {
		t.Fatal("expected hackernews duplicate")
	}
	if cfg, err := Load(path); err != nil || len(cfg.Providers) != 1 {
		t.Fatalf("load: err=%v providers=%d", err, len(cfg.Providers))
	}
}

func TestAddSource_MissingFileError(t *testing.T) {
	_, err := AddSource(filepath.Join(t.TempDir(), "nope.yaml"), "hackernews", "", "", "")
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}
