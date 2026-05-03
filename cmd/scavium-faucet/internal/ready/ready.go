package ready

import (
	"context"
	"errors"
	"sort"
	"time"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
)

type CheckFunc func(context.Context) error

type Check struct {
	Name string
	Run  CheckFunc
}

type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Result struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks"`
	Time   string        `json:"time"`
}

func DefaultChecks() []Check {
	return []Check{
		{Name: "db", Run: StubOK},
		{Name: "rpc", Run: StubOK},
		{Name: "wallet", Run: StubOK},
		{Name: "queue", Run: StubOK},
	}
}

func StubOK(context.Context) error {
	return nil
}

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

func ErrDegraded(message string) error {
	return errors.New(message)
}
