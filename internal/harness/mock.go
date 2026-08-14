package harness

import (
	"context"
	"encoding/json"
	"sync"
)

type Mock struct {
	MockName         string
	Result           Result
	StructuredOutput json.RawMessage
	Err              error
	// GrantErr makes the mock refuse a grant, standing in for a harness that
	// cannot enforce one.
	GrantErr   error
	InvokeFunc func(ctx context.Context, root, prompt string, grant Grant) (Result, error)
	Calls      []MockCall
	mu         sync.Mutex
}

type MockCall struct {
	Root   string
	Prompt string
	Schema string
	Grant  Grant
}

func (m *Mock) Name() string {
	if m.MockName != "" {
		return m.MockName
	}
	return "mock"
}

func (m *Mock) CheckGrant(Grant) error { return m.GrantErr }

func (m *Mock) Invoke(ctx context.Context, root, prompt string, grant Grant) (Result, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCall{Root: root, Prompt: prompt, Grant: grant})
	m.mu.Unlock()
	if m.InvokeFunc != nil {
		return m.InvokeFunc(ctx, root, prompt, grant)
	}
	return m.Result, m.Err
}

func (m *Mock) InvokeStructured(ctx context.Context, root, prompt string, schema []byte, grant Grant) (StructuredResult, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCall{Root: root, Prompt: prompt, Schema: string(schema), Grant: grant})
	m.mu.Unlock()
	return StructuredResult{Result: m.Result, StructuredOutput: m.StructuredOutput}, m.Err
}
