package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_DefaultsAndProviderDecode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := writeTemp(t, `
providers:
  - name: hackernews
    enabled: true
    config:
      feeds: [top, show]
      max_items: 50
  - name: reddit
    enabled: false
    config:
      subreddits: [golang]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Defaults applied.
	wantDB := filepath.Join(os.Getenv("HOME"), AppDirName, DefaultDBName)
	if cfg.Cache.DBPath != wantDB {
		t.Errorf("db_path default: got %q, want %q", cfg.Cache.DBPath, wantDB)
	}

	// EnabledProviders filters by enabled.
	en := cfg.EnabledProviders()
	if len(en) != 1 || en[0].Name != "hackernews" {
		t.Fatalf("EnabledProviders: %+v", en)
	}

	// Per-provider Decode pulls typed settings out of the yaml.Node.
	var s struct {
		Feeds    []string `yaml:"feeds"`
		MaxItems int      `yaml:"max_items"`
	}
	if err := en[0].Decode(&s); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(s.Feeds) != 2 || s.Feeds[0] != "top" || s.MaxItems != 50 {
		t.Fatalf("decoded settings: %+v", s)
	}
}

func TestLoad_DefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, AppDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, DefaultConfigName), []byte(`
providers:
  - name: hackernews
    enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load default path: %v", err)
	}
	wantDB := filepath.Join(home, AppDirName, DefaultDBName)
	if cfg.Cache.DBPath != wantDB {
		t.Fatalf("default db path: got %q, want %q", cfg.Cache.DBPath, wantDB)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := map[string]string{
		"no providers": `cache: {db_path: x}`,
		"empty name": `
providers:
  - enabled: true
`,
		"duplicate name": `
providers:
  - name: hackernews
    enabled: true
  - name: hackernews
    enabled: false
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, body)); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestDecode_EmptyBlockIsNoop(t *testing.T) {
	path := writeTemp(t, `
providers:
  - name: hackernews
    enabled: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Feeds []string `yaml:"feeds"`
	}
	if err := cfg.Providers[0].Decode(&s); err != nil {
		t.Fatalf("Decode empty: %v", err)
	}
	if s.Feeds != nil {
		t.Fatalf("expected nil feeds, got %+v", s.Feeds)
	}
}

func TestInitDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, created, err := InitDefaultConfig("", false)
	if err != nil {
		t.Fatalf("InitDefaultConfig: %v", err)
	}
	wantPath := filepath.Join(home, AppDirName, DefaultConfigName)
	if path != wantPath {
		t.Fatalf("path: got %q, want %q", path, wantPath)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("generated config should load: %v", err)
	}

	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, created, err = InitDefaultConfig("", false)
	if err != nil {
		t.Fatalf("second InitDefaultConfig: %v", err)
	}
	if created {
		t.Fatal("expected existing config to be preserved")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "custom" {
		t.Fatalf("existing config overwritten without force: %q", raw)
	}

	_, created, err = InitDefaultConfig("", true)
	if err != nil {
		t.Fatalf("force InitDefaultConfig: %v", err)
	}
	if !created {
		t.Fatal("expected force to rewrite config")
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("forced generated config should load: %v", err)
	}
}
