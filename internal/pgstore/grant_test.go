package pgstore

import "testing"

func TestMayAct_EmptyGrantIsNoDepartmentsNotAll(t *testing.T) {
	var unscoped Identity
	if unscoped.MayAct("QA") {
		t.Error("an identity with no grant may act on QA - an empty grant must fail closed")
	}
}

func TestMayAct_NamedAndUnscopedGrants(t *testing.T) {
	scoped := Identity{Departments: []string{"Engineering", "QA"}}
	all := Identity{Departments: []string{UnscopedDepartment}}

	cases := []struct {
		id    Identity
		dept  string
		grant bool
	}{
		{scoped, "QA", true},
		{scoped, "qa", true}, // capability names are compared case-insensitively
		{scoped, "Engineering", true},
		{scoped, "Release", false},
		{scoped, "*", false}, // asking for the sentinel is not a grant to it
		{all, "Release", true},
		{all, "anything", true},
	}
	for _, tc := range cases {
		if got := tc.id.MayAct(tc.dept); got != tc.grant {
			t.Errorf("Identity%v.MayAct(%q) = %v, want %v", tc.id.Departments, tc.dept, got, tc.grant)
		}
	}
}

func TestGrantedFrom_NarrowsToTheGrant(t *testing.T) {
	scoped := Identity{Departments: []string{"QA"}}

	got := scoped.GrantedFrom([]string{"Engineering", "QA", "Release"})
	if len(got) != 1 || got[0] != "QA" {
		t.Errorf("GrantedFrom = %v, want [QA]", got)
	}
	if none := scoped.GrantedFrom([]string{"Engineering", "Release"}); len(none) != 0 {
		t.Errorf("GrantedFrom = %v, want empty when nothing requested is granted", none)
	}
}
