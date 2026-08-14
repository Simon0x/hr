package harness

import "encoding/json"

type usage struct {
	InputTokens              float64 `json:"input_tokens"`
	OutputTokens             float64 `json:"output_tokens"`
	CacheCreationInputTokens float64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     float64 `json:"cache_read_input_tokens"`
}

func (u usage) total() float64 {
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

type envelope struct {
	Result           string          `json:"result"`
	SessionID        string          `json:"session_id"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	DurationMS       int64           `json:"duration_ms"`
	NumTurns         int             `json:"num_turns"`
	Usage            usage           `json:"usage"`
	StructuredOutput json.RawMessage `json:"structured_output"`
}

func parseEnvelope(raw []byte) (envelope, bool) {
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return envelope{}, false
	}
	return e, true
}
