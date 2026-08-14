package dispatch

import (
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
)

// NewRequest records one model invocation: the harness, the prompt by
// digest, the tool grant, and the procedure pinned to its content. An
// unreadable procedure drops ProcedureDigest rather than failing dispatch.
func NewRequest(root string, h harness.Harness, prompt string, grant harness.Grant, procedure string) *ledger.Request {
	allow := grant.Allow
	if allow == nil {
		allow = []string{}
	}
	req := &ledger.Request{
		Harness:      h.Name(),
		PromptDigest: ledger.TextDigest(prompt),
		Tools:        allow,
		DeniedTools:  grant.Deny,
		Procedure:    procedure,
	}
	if procedure != "" {
		if raw, err := os.ReadFile(filepath.Join(root, procedure)); err == nil {
			req.ProcedureDigest = ledger.TextDigest(string(raw))
		}
	}
	return req
}
