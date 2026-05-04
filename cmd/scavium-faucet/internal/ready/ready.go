// Package ready evaluates faucet readiness checks and aggregates their results.
package ready

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	Name       string `json:"name"`
	Status     Status `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Summary captures aggregate readiness counters without requiring clients to inspect each check.
type Summary struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Degraded int `json:"degraded"`
}

// Result is the aggregated readiness response returned by the API.
type Result struct {
	Status  Status        `json:"status"`
	Checks  []CheckResult `json:"checks"`
	Summary Summary       `json:"summary"`
	Time    string        `json:"time"`
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

type DBPinger interface {
	Ping(context.Context) error
}

type QueuePinger interface {
	PingQueue(context.Context) error
}

type RPCClient interface {
	ChainID(context.Context) (*big.Int, error)
}

type BalanceClient interface {
	BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
}

type AddressProvider interface {
	Address() common.Address
}

func DBCheck(db DBPinger) Check {
	return Check{Name: "db", Run: func(ctx context.Context) error {
		if db == nil {
			return ErrDegraded("db check is not configured")
		}
		if err := db.Ping(ctx); err != nil {
			return fmt.Errorf("db ping failed: %w", err)
		}
		return nil
	}}
}

func QueueCheck(queue QueuePinger) Check {
	return Check{Name: "queue", Run: func(ctx context.Context) error {
		if queue == nil {
			return ErrDegraded("queue check is not configured")
		}
		if err := queue.PingQueue(ctx); err != nil {
			return fmt.Errorf("queue check failed: %w", err)
		}
		return nil
	}}
}

func RPCCheck(client RPCClient) Check {
	return Check{Name: "rpc", Run: func(ctx context.Context) error {
		if client == nil {
			return ErrDegraded("rpc check is not configured")
		}
		if _, err := client.ChainID(ctx); err != nil {
			return fmt.Errorf("rpc chain id failed: %w", err)
		}
		return nil
	}}
}

func WalletCheck(client BalanceClient, signer AddressProvider) Check {
	return Check{Name: "wallet", Run: func(ctx context.Context) error {
		if client == nil || signer == nil {
			return ErrDegraded("wallet check is not configured")
		}
		balance, err := client.BalanceAt(ctx, signer.Address(), nil)
		if err != nil {
			return fmt.Errorf("wallet balance failed: %w", err)
		}
		if balance == nil {
			return ErrDegraded("wallet balance unavailable")
		}
		return nil
	}}
}

// Evaluate runs all checks, sorts them by name, and derives the overall status.
func Evaluate(ctx context.Context, checks []Check) Result {
	results := make([]CheckResult, 0, len(checks))
	status := StatusOK
	summary := Summary{Total: len(checks)}

	for _, check := range checks {
		startedAt := time.Now()
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

		result.DurationMS = time.Since(startedAt).Milliseconds()
		if result.Status != StatusOK {
			status = StatusDegraded
			summary.Degraded++
		} else {
			summary.OK++
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return Result{
		Status:  status,
		Checks:  results,
		Summary: summary,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
}

// ErrDegraded returns a readiness failure with the provided message.
func ErrDegraded(message string) error {
	return errors.New(message)
}
