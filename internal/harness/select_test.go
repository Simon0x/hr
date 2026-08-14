package harness

import (
	"context"
	"testing"
)

// inner looks through the grant guard Select wraps every harness in.
func inner(h Harness) Harness {
	if g, ok := h.(guarded); ok {
		return g.Harness
	}
	return h
}

func TestSelect_DefaultsAndNamesResolveToTheRightType(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
		check   func(Harness) bool
	}{
		{name: "", check: func(h Harness) bool { _, ok := inner(h).(Claude); return ok }},
		{name: "claude", check: func(h Harness) bool { _, ok := inner(h).(Claude); return ok }},
		{name: "mock", check: func(h Harness) bool { _, ok := inner(h).(*Mock); return ok }},
		{name: "nonexistent", wantErr: true},
	}

	for _, c := range cases {
		h, err := Select(c.name)
		if c.wantErr {
			if err == nil {
				t.Errorf("Select(%q): expected an error, got none", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("Select(%q): %v", c.name, err)
			continue
		}
		if !c.check(h) {
			t.Errorf("Select(%q) returned %T, wrong type", c.name, h)
		}
	}
}

func TestSelect_MockSucceedsWithoutARealAgent(t *testing.T) {
	h, err := Select("mock")
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.Invoke(context.Background(), "", "prompt", Grant{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Error("mock harness should report OK without invoking any CLI")
	}
}
