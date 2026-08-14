package dispatch

import (
	"context"
	"sync"

	"github.com/Simon0x/hr/internal/budget"
	"github.com/Simon0x/hr/internal/guard"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/persistence"
)

const (
	fallbackLeadTokenEstimate = 50000
	fallbackFanOutConcurrency = 3
)

type Lead struct {
	Name   string
	Prompt string
	Grant  harness.Grant
}

type LeadResult struct {
	Lead       Lead
	Result     harness.Result
	Blocked    bool
	GuardToken string
}

func FanOut(ctx context.Context, st persistence.Store, root string, leads []Lead, goal string, h harness.Harness, concurrency int, leadTokenEstimate float64) ([]LeadResult, budget.CheckResult, error) {
	if concurrency <= 0 {
		concurrency = fallbackFanOutConcurrency
	}
	if leadTokenEstimate <= 0 {
		leadTokenEstimate = fallbackLeadTokenEstimate
	}

	var checkResult budget.CheckResult
	if goal != "" {
		estimate := float64(len(leads)) * leadTokenEstimate
		check, err := budget.Check(ctx, st, root, goal, estimate)
		if err != nil {
			return nil, budget.CheckResult{}, err
		}
		checkResult = check
		if check.Refused {
			return nil, checkResult, nil
		}
	}

	results := make([]LeadResult, len(leads))
	if len(leads) < concurrency {
		concurrency = len(leads)
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, lead := range leads {
		wg.Add(1)
		go func(i int, lead Lead) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Each lead gets its own guard token: they run concurrently,
			// and a shared denial log would mix their refusals together.
			leadCtx := ctx
			token := ""
			if gc, ok := guard.FromContext(ctx); ok {
				token = guard.NewToken()
				gc.Token = token
				leadCtx = guard.WithContext(ctx, gc)
			}
			res, err := h.Invoke(leadCtx, root, lead.Prompt, lead.Grant)
			results[i].GuardToken = token
			lr := LeadResult{Lead: lead}
			if err != nil {
				lr.Result = harness.Result{OK: false, Output: err.Error()}
			} else {
				lr.Result = res
				lr.Blocked = !res.OK && harness.LooksPermissionBlocked(res.Output)
			}
			results[i] = lr
		}(i, lead)
	}
	wg.Wait()

	return results, checkResult, nil
}
