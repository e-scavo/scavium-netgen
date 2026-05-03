package ready

import (
	"context"
	"testing"
	"time"
)

func TestEvaluateReportsOK(t *testing.T) {
	result := Evaluate(context.Background(), DefaultChecks())

	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if len(result.Checks) != 4 {
		t.Fatalf("checks length = %d, want 4", len(result.Checks))
	}
	if _, err := time.Parse(time.RFC3339, result.Time); err != nil {
		t.Fatalf("time = %q, want RFC3339: %v", result.Time, err)
	}
	for _, check := range result.Checks {
		if check.Status != StatusOK {
			t.Fatalf("check %q status = %q, want %q", check.Name, check.Status, StatusOK)
		}
	}
}

func TestEvaluateReportsDegradedCheck(t *testing.T) {
	result := Evaluate(context.Background(), []Check{
		{Name: "rpc", Run: func(context.Context) error {
			return ErrDegraded("rpc unavailable")
		}},
		{Name: "db", Run: StubOK},
	})

	if result.Status != StatusDegraded {
		t.Fatalf("status = %q, want %q", result.Status, StatusDegraded)
	}
	if result.Checks[0].Name != "db" {
		t.Fatalf("first check = %q, want db", result.Checks[0].Name)
	}
	if result.Checks[1].Name != "rpc" {
		t.Fatalf("second check = %q, want rpc", result.Checks[1].Name)
	}
	if result.Checks[1].Error != "rpc unavailable" {
		t.Fatalf("rpc error = %q", result.Checks[1].Error)
	}
}

func TestEvaluateReportsMissingCheckFunction(t *testing.T) {
	result := Evaluate(context.Background(), []Check{{Name: "wallet"}})

	if result.Status != StatusDegraded {
		t.Fatalf("status = %q, want %q", result.Status, StatusDegraded)
	}
	if result.Checks[0].Error != "check is not configured" {
		t.Fatalf("error = %q", result.Checks[0].Error)
	}
}
