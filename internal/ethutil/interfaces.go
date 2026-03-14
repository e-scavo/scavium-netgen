package ethutil

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

type HeaderReader interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
}
