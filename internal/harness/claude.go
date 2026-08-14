package harness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
)

type Claude struct{}

func (Claude) Name() string { return "claude" }

// CheckGrant accepts every grant: the CLI enforces both halves natively
// through --allowedTools and --disallowedTools.
func (Claude) CheckGrant(Grant) error { return nil }

func (Claude) Invoke(ctx context.Context, root, prompt string, grant Grant) (Result, error) {
	args := append([]string{"--output-format", "json", "--permission-mode", "dontAsk"}, grantArgs(grant)...)
	result, _, err := runClaude(ctx, root, prompt, args...)
	return result, err
}

func (Claude) InvokeStructured(ctx context.Context, root, prompt string, schema []byte, grant Grant) (StructuredResult, error) {
	cliSchema, err := stripSchemaMeta(schema)
	if err != nil {
		return StructuredResult{}, err
	}
	args := append([]string{"--output-format", "json", "--permission-mode", "dontAsk", "--json-schema", string(cliSchema)}, grantArgs(grant)...)
	result, structuredOutput, err := runClaude(ctx, root, prompt, args...)
	return StructuredResult{Result: result, StructuredOutput: structuredOutput}, err
}

// grantArgs passes each entry as its own argv value. Both flags are variadic,
// and an argument pattern contains spaces and can contain a comma, so joining
// the list into one string would split entries in the wrong places.
func grantArgs(grant Grant) []string {
	var args []string
	if len(grant.Allow) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, grant.Allow...)
	}
	if len(grant.Deny) > 0 {
		args = append(args, "--disallowedTools")
		args = append(args, grant.Deny...)
	}
	return args
}

// $schema makes `claude --json-schema` fail closed - it treats it as an unresolved ref.
func stripSchemaMeta(schema []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		return nil, err
	}
	delete(m, "$schema")
	return json.Marshal(m)
}

func runClaude(ctx context.Context, root, prompt string, extraArgs ...string) (Result, json.RawMessage, error) {
	// The guard settings live in a directory that goes away with the run, so
	// a crashed invocation cannot leave a hook wired up for the next one.
	tmp, err := os.MkdirTemp("", "hr-guard-")
	if err != nil {
		return Result{}, nil, err
	}
	defer os.RemoveAll(tmp)

	guardArgs, guardEnv, err := guardSettings(ctx, tmp)
	if err != nil {
		return Result{}, nil, err
	}

	args := append([]string{"-p"}, extraArgs...)
	args = append(args, guardArgs...)
	args = append(args, "--", prompt)

	argv, err := confine(root, append([]string{"claude"}, args...))
	if err != nil {
		return Result{}, nil, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	if len(guardEnv) > 0 {
		cmd.Env = append(os.Environ(), guardEnv...)
	}
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		return Result{}, nil, err
	}

	result := Result{OK: exitCode == 0, ExitCode: exitCode, Output: string(out)}
	var structuredOutput json.RawMessage
	if env, ok := parseEnvelope(out); ok {
		result.Output = env.Result
		result.CostUSD = env.TotalCostUSD
		result.Tokens = env.Usage.total()
		result.DurationMS = env.DurationMS
		result.NumTurns = env.NumTurns
		result.SessionID = env.SessionID
		structuredOutput = env.StructuredOutput
	}
	return result, structuredOutput, nil
}
