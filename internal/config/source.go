package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddSource merges a resolved source into the config file at path (empty =
// default path), editing the YAML node tree so existing comments survive. It is
// idempotent: re-adding the same source returns added=false.
//
//   - twitter:    appends key to providers[twitter].config.accounts
//   - reddit:     appends key to providers[reddit].config.subreddits
//   - rssblog:    appends {url: feedURL, source} to providers[rssblog].config.feeds
//   - hackernews: ensures a providers[hackernews] block exists
//
// If the target provider block is absent, a new one is created.
func AddSource(path, provider, key, feedURL, source string) (added bool, err error) {
	if path == "" {
		if path, err = DefaultConfigPath(); err != nil {
			return false, err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err // caller inspects os.IsNotExist
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return false, fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("config %s: unexpected structure", path)
	}
	root := doc.Content[0]

	providers := mappingGet(root, "providers")
	if providers == nil || providers.Kind != yaml.SequenceNode {
		providers = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mappingAppend(root, "providers", providers)
	}

	block := findProviderBlock(providers, provider)
	if block == nil {
		// No such provider yet: create the whole block.
		nb, berr := newProviderBlock(provider, key, feedURL, source)
		if berr != nil {
			return false, berr
		}
		providers.Content = append(providers.Content, nb)
		return true, writeDoc(path, &doc)
	}

	// Provider exists: append the new item to the right field (idempotent).
	switch provider {
	case "hackernews":
		return false, nil // singleton; already present
	case "twitter":
		added = appendScalar(block, "accounts", key, true)
	case "reddit":
		added = appendScalar(block, "subreddits", key, true)
	case "rssblog":
		added = appendFeed(block, feedURL, source)
	default:
		return false, fmt.Errorf("unknown provider %q", provider)
	}
	if !added {
		return false, nil
	}
	return true, writeDoc(path, &doc)
}

// appendScalar appends value to the named sequence under the block's config,
// creating the sequence if needed. fold makes the dedup check case-insensitive.
// Returns false (no-op) if the value already exists.
func appendScalar(block *yaml.Node, field, value string, fold bool) bool {
	seq := ensureConfigSeq(block, field)
	for _, n := range seq.Content {
		if n.Kind == yaml.ScalarNode && eq(n.Value, value, fold) {
			return false
		}
	}
	seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	return true
}

// appendFeed appends {url, source} to the rssblog feeds sequence, deduping by URL.
func appendFeed(block *yaml.Node, feedURL, source string) bool {
	seq := ensureConfigSeq(block, "feeds")
	for _, item := range seq.Content {
		if item.Kind == yaml.MappingNode {
			if u := mappingGet(item, "url"); u != nil && u.Value == feedURL {
				return false
			}
		}
	}
	item, err := nodeFromYAML(fmt.Sprintf("url: %s\nsource: %s\n", feedURL, source))
	if err != nil {
		return false
	}
	seq.Content = append(seq.Content, item)
	return true
}

// ensureConfigSeq returns the named sequence node under block.config, creating
// block.config and/or the sequence if absent.
func ensureConfigSeq(block *yaml.Node, field string) *yaml.Node {
	cfg := mappingGet(block, "config")
	if cfg == nil || cfg.Kind != yaml.MappingNode {
		cfg = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mappingAppend(block, "config", cfg)
	}
	seq := mappingGet(cfg, field)
	if seq == nil || seq.Kind != yaml.SequenceNode {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mappingAppend(cfg, field, seq)
	}
	return seq
}

// findProviderBlock returns the providers[] entry whose name == provider.
func findProviderBlock(providers *yaml.Node, provider string) *yaml.Node {
	for _, block := range providers.Content {
		if block.Kind != yaml.MappingNode {
			continue
		}
		if n := mappingGet(block, "name"); n != nil && n.Value == provider {
			return block
		}
	}
	return nil
}

// newProviderBlock builds a fresh providers[] entry for the given source.
func newProviderBlock(provider, key, feedURL, source string) (*yaml.Node, error) {
	var snippet string
	switch provider {
	case "twitter":
		snippet = fmt.Sprintf("name: twitter\nenabled: true\nconfig:\n  accounts:\n    - %s\n", key)
	case "reddit":
		snippet = fmt.Sprintf("name: reddit\nenabled: true\nconfig:\n  subreddits:\n    - %s\n", key)
	case "hackernews":
		snippet = "name: hackernews\nenabled: true\nconfig:\n  feeds: [top]\n"
	case "rssblog":
		snippet = fmt.Sprintf("name: rssblog\nenabled: true\nconfig:\n  feeds:\n    - url: %s\n      source: %s\n", feedURL, source)
	default:
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
	return nodeFromYAML(snippet)
}

// --- yaml.Node helpers ---

func mappingGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mappingAppend(m *yaml.Node, key string, val *yaml.Node) {
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

// nodeFromYAML parses a small YAML snippet and returns its root content node.
func nodeFromYAML(s string) (*yaml.Node, error) {
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(s), &n); err != nil {
		return nil, err
	}
	if len(n.Content) == 0 {
		return nil, fmt.Errorf("empty yaml snippet")
	}
	return n.Content[0], nil
}

func writeDoc(path string, doc *yaml.Node) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	_ = enc.Close()
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func eq(a, b string, fold bool) bool {
	if fold {
		return strings.EqualFold(a, b)
	}
	return a == b
}
