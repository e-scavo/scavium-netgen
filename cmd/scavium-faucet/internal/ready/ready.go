// Package ready evaluates faucet readiness checks and aggregates their results.
package ready

import (
	"context"
	"errors"
	"sort"
	"time"
)

// Status is the aggregate health state reported by readiness checks.
type Status string

// Readiness statuses returned by Evaluate.
const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
)

// CheckFunc runs one readiness probe.
type CheckFunc func(context.Context) error

// Check names a readiness probe and its execution function.
type Check struct {
	Name string
	Run  CheckFunc
}

// CheckResult is the JSON-friendly outcome of one readiness check.
type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Result is the aggregated readiness response returned by the API.
type Result struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks"`
	Time   string        `json:"time"`
}

// DefaultChecks returns the default startup readiness probes.
func DefaultChecks() []Check {
	return []Check{
		{Name: "db", Run: StubOK},
		{Name: "rpc", Run: StubOK},
		{Name: "wallet", Run: StubOK},
		{Name: "queue", Run: StubOK},
	}
}

// StubOK is a placeholder check that always succeeds.
func StubOK(context.Context) error {
	return nil
}

// Evaluate runs all checks, sorts them by name, and derives the overall status.
func Evaluate(ctx context.Context, checks []Check) Result {
	results := make([]CheckResult, 0, len(checks))
	status := StatusOK

	for _, check := range checks {
		result := CheckResult{
			Name:   check.Name,
			Status: StatusOK,
		}

		if check.Run == nil {
			result.Status = StatusDegraded
			result.Error = "check is not configured"
		} else if err := check.Run(ctx); err != nil {
			result.Status = StatusDegraded
			result.Error = err.Error()
		}

		if result.Status != StatusOK {
			status = StatusDegraded
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return Result{
		Status: status,
		Checks: results,
		Time:   time.Now().UTC().Format(time.RFC3339),
	}
}

// ErrDegraded returns a readiness failure with the provided message.
func ErrDegraded(message string) error {
	return errors.New(message)
}
