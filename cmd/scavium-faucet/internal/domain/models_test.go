package domain

import "testing"

func TestClaimStatusValidation(t *testing.T) {
	valid := []ClaimStatus{
		ClaimStatusReceived,
		ClaimStatusValidated,
		ClaimStatusQueued,
		ClaimStatusSending,
		ClaimStatusSent,
		ClaimStatusConfirmed,
		ClaimStatusFailed,
		ClaimStatusRejected,
		ClaimStatusPaused,
	}

	for _, status := range valid {
		if !IsValidClaimStatus(status) {
			t.Fatalf("status %q should be valid", status)
		}
	}

	if IsValidClaimStatus("bogus") {
		t.Fatal("bogus status should be invalid")
	}
}

func TestTerminalClaimStatus(t *testing.T) {
	for _, status := range []ClaimStatus{ClaimStatusConfirmed, ClaimStatusFailed, ClaimStatusRejected} {
		if !IsTerminalClaimStatus(status) {
			t.Fatalf("status %q should be terminal", status)
		}
	}

	for _, status := range []ClaimStatus{ClaimStatusReceived, ClaimStatusQueued, ClaimStatusSent, ClaimStatusPaused} {
		if IsTerminalClaimStatus(status) {
			t.Fatalf("status %q should not be terminal", status)
		}
	}
}

func TestFaucetStatusValidation(t *testing.T) {
	for _, status := range []FaucetStatus{
		FaucetStatusActive,
		FaucetStatusPaused,
		FaucetStatusMaintenance,
		FaucetStatusNoFunds,
	} {
		if !IsValidFaucetStatus(status) {
			t.Fatalf("status %q should be valid", status)
		}
	}

	if IsValidFaucetStatus("draining") {
		t.Fatal("unknown faucet status should be invalid")
	}
}
