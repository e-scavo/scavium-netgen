package domain

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

func ValidateAddress(address string) (common.Address, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return common.Address{}, fmt.Errorf("address is required")
	}
	if !strings.HasPrefix(trimmed, "0x") {
		return common.Address{}, fmt.Errorf("address must have 0x prefix")
	}
	if !common.IsHexAddress(trimmed) {
		return common.Address{}, fmt.Errorf("address must be a valid EVM address")
	}
	return common.HexToAddress(trimmed), nil
}

func MustValidateAddress(address string) common.Address {
	parsed, err := ValidateAddress(address)
	if err != nil {
		panic(err)
	}
	return parsed
}
