package harness

import "testing"

func TestParseEnvelope_Basic(t *testing.T) {
	raw := []byte(`{"type":"result","subtype":"success","result":"done","session_id":"abc-123","total_cost_usd":0.0042,"duration_ms":3210,"num_turns":2}`)
	env, ok := parseEnvelope(raw)
	if !ok {
		t.Fatal("expected envelope to parse")
	}
	if env.Result != "done" {
		t.Errorf("Result = %q, want done", env.Result)
	}
	if env.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want abc-123", env.SessionID)
	}
	if env.TotalCostUSD != 0.0042 {
		t.Errorf("TotalCostUSD = %v, want 0.0042", env.TotalCostUSD)
	}
	if env.DurationMS != 3210 {
		t.Errorf("DurationMS = %v, want 3210", env.DurationMS)
	}
	if env.NumTurns != 2 {
		t.Errorf("NumTurns = %v, want 2", env.NumTurns)
	}
}

func TestParseEnvelope_StructuredOutput(t *testing.T) {
	raw := []byte(`{"result":"done","structured_output":{"foo":"bar"}}`)
	env, ok := parseEnvelope(raw)
	if !ok {
		t.Fatal("expected envelope to parse")
	}
	if string(env.StructuredOutput) != `{"foo":"bar"}` {
		t.Errorf("StructuredOutput = %s, want {\"foo\":\"bar\"}", env.StructuredOutput)
	}
}

func TestParseEnvelope_NotJSON(t *testing.T) {
	_, ok := parseEnvelope([]byte("not json at all, e.g. an invalid-flag error printed to stderr"))
	if ok {
		t.Fatal("expected parse to fail on non-JSON input")
	}
}
