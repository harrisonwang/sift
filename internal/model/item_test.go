package model

import "testing"

func TestID_StableAndIdentityScoped(t *testing.T) {
	a := Item{Provider: "twitter", ExternalID: "123"}
	b := Item{Provider: "twitter", ExternalID: "123", Title: "different title"}
	if a.ID() != b.ID() {
		t.Error("ID must depend only on provider + external_id")
	}

	c := Item{Provider: "reddit", ExternalID: "123"}
	if a.ID() == c.ID() {
		t.Error("same external_id under different providers must not collide")
	}

	d := Item{Provider: "twitter", ExternalID: "456"}
	if a.ID() == d.ID() {
		t.Error("different external_id must produce different ID")
	}
}

func TestTitleOrSummary(t *testing.T) {
	if got := (Item{Title: "  hi  "}).TitleOrSummary(); got != "hi" {
		t.Errorf("title path: %q", got)
	}
	if got := (Item{Summary: "fallback"}).TitleOrSummary(); got != "fallback" {
		t.Errorf("summary fallback: %q", got)
	}
	if got := (Item{}).TitleOrSummary(); got != "(untitled)" {
		t.Errorf("empty: %q", got)
	}
}
