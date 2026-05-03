package domain

import "testing"

func TestValidateAddress(t *testing.T) {
	address, err := ValidateAddress("  0x52908400098527886E0F7030069857D2E4169EE7  ")
	if err != nil {
		t.Fatalf("validate address: %v", err)
	}

	if got := address.Hex(); got != "0x52908400098527886E0F7030069857D2E4169EE7" {
		t.Fatalf("address = %q", got)
	}
}

func TestValidateAddressRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "empty", address: ""},
		{name: "missing prefix", address: "52908400098527886E0F7030069857D2E4169EE7"},
		{name: "too short", address: "0x52908400098527886E0F7030069857D2E4169E"},
		{name: "too long", address: "0x52908400098527886E0F7030069857D2E4169EE700"},
		{name: "not hex", address: "0xZZ908400098527886E0F7030069857D2E4169EE7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateAddress(tt.address); err == nil {
				t.Fatal("validate address returned nil")
			}
		})
	}
}

func TestMustValidateAddressPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = MustValidateAddress("not an address")
}
