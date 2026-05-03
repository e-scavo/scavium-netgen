package ready

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

func TestDBCheckReportsPingFailure(t *testing.T) {
	result := Evaluate(context.Background(), []Check{DBCheck(failingDB{})})

	if result.Status != StatusDegraded {
		t.Fatalf("status = %q, want degraded", result.Status)
	}
	if result.Checks[0].Name != "db" {
		t.Fatalf("check name = %q, want db", result.Checks[0].Name)
	}
	if result.Checks[0].Error == "" {
		t.Fatal("db check error is empty")
	}
}

func TestQueueCheckReportsStableName(t *testing.T) {
	result := Evaluate(context.Background(), []Check{QueueCheck(okQueue{})})

	if result.Status != StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Checks[0].Name != "queue" {
		t.Fatalf("check name = %q, want queue", result.Checks[0].Name)
	}
}

func TestRPCAndWalletChecks(t *testing.T) {
	client := okChainClient{}
	signer := staticAddressProvider{address: common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7")}

	result := Evaluate(context.Background(), []Check{
		RPCCheck(client),
		WalletCheck(client, signer),
	})

	if result.Status != StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Checks[0].Name != "rpc" || result.Checks[1].Name != "wallet" {
		t.Fatalf("checks = %#v", result.Checks)
	}
}

type failingDB struct{}

func (failingDB) Ping(context.Context) error {
	return errors.New("closed")
}

type okQueue struct{}

func (okQueue) PingQueue(context.Context) error {
	return nil
}

type okChainClient struct{}

func (okChainClient) ChainID(context.Context) (*big.Int, error) {
	return big.NewInt(31337), nil
}

func (okChainClient) BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	return big.NewInt(1), nil
}

type staticAddressProvider struct {
	address common.Address
}

func (p staticAddressProvider) Address() common.Address {
	return p.address
}
