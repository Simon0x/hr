package budget

import (
	"testing"

	"github.com/Simon0x/hr/internal/store"
)

func TestFindGoal_MatchesByArtifactID(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "abc123def456", Kind: "goal", Predicate: map[string]any{"outcome": "grow retention", "owner": "simon", "budget": "1000"}},
	}

	got := FindGoal(artifacts, "abc123def456")
	if got == nil {
		t.Fatal("expected FindGoal to match by artifact ID")
	}
	if got.ID != "abc123def456" {
		t.Errorf("matched artifact ID = %q, want abc123def456", got.ID)
	}
}

func TestFindGoal_StillMatchesByOutcomeAndOwner(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "111111111111", Kind: "goal", Predicate: map[string]any{"outcome": "grow retention", "owner": "simon"}},
		{ID: "222222222222", Kind: "goal", Predicate: map[string]any{"outcome": "cut latency", "owner": "priya"}},
	}

	if got := FindGoal(artifacts, "grow retention"); got == nil || got.ID != "111111111111" {
		t.Errorf("outcome match failed, got %+v", got)
	}
	if got := FindGoal(artifacts, "priya"); got == nil || got.ID != "222222222222" {
		t.Errorf("owner match failed, got %+v", got)
	}
}

func TestFindGoal_NoMatchReturnsNil(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "111111111111", Kind: "goal", Predicate: map[string]any{"outcome": "grow retention"}},
	}
	if got := FindGoal(artifacts, "does-not-exist"); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
