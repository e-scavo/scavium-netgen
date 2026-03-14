package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"scavium-netgen/internal/ethutil"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type inspectResult struct {
	ChainID         *big.Int
	BlockNumber     uint64
	PeerCount       uint64
	GasPriceWei     *big.Int
	Syncing         any
	AvgBlockTimeSec float64
	Validators      []string
}

func main() {
	log.SetFlags(0)

	if len(os.Args) != 2 {
		fmt.Println("Usage:")
		fmt.Println("  scavium-net-inspect <rpc-url>")
		os.Exit(1)
	}

	rpcURL := os.Args[1]

	result, err := inspectNetwork(rpcURL)
	if err != nil {
		log.Fatalf("inspect network: %v", err)
	}

	printResult(rpcURL, result)
}

func inspectNetwork(rpcURL string) (*inspectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ethClient, err := ethutil.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial eth client: %w", err)
	}
	defer ethClient.Close()

	rpcClient, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc client: %w", err)
	}
	defer rpcClient.Close()

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}

	blockNumber, err := ethClient.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("block number: %w", err)
	}

	gasPrice, err := ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}

	var peerCountHex hexutil.Uint64
	if err := rpcClient.CallContext(ctx, &peerCountHex, "net_peerCount"); err != nil {
		return nil, fmt.Errorf("peer count: %w", err)
	}

	var syncing any
	if err := rpcClient.CallContext(ctx, &syncing, "eth_syncing"); err != nil {
		return nil, fmt.Errorf("syncing: %w", err)
	}

	avgBlockTimeSec, err := calculateAverageBlockTime(ctx, ethClient, blockNumber, 20)
	if err != nil {
		avgBlockTimeSec = 0
	}

	validators, _ := getValidators(ctx, rpcClient)

	return &inspectResult{
		ChainID:         chainID,
		BlockNumber:     blockNumber,
		PeerCount:       uint64(peerCountHex),
		GasPriceWei:     gasPrice,
		Syncing:         syncing,
		AvgBlockTimeSec: avgBlockTimeSec,
		Validators:      validators,
	}, nil
}

func calculateAverageBlockTime(ctx context.Context, client ethutil.HeaderReader, latest uint64, sample int) (float64, error) {
	if latest == 0 || sample < 2 {
		return 0, fmt.Errorf("not enough blocks")
	}

	if int(latest) < sample {
		sample = int(latest)
	}
	if sample < 2 {
		return 0, fmt.Errorf("not enough blocks")
	}

	start := latest - uint64(sample) + 1

	firstHeader, err := client.HeaderByNumber(ctx, big.NewInt(int64(start)))
	if err != nil {
		return 0, fmt.Errorf("first header: %w", err)
	}

	lastHeader, err := client.HeaderByNumber(ctx, big.NewInt(int64(latest)))
	if err != nil {
		return 0, fmt.Errorf("last header: %w", err)
	}

	firstTs := int64(firstHeader.Time)
	lastTs := int64(lastHeader.Time)
	if lastTs <= firstTs {
		return 0, fmt.Errorf("invalid timestamps")
	}

	intervals := float64(sample - 1)
	return float64(lastTs-firstTs) / intervals, nil
}

func getValidators(ctx context.Context, rpcClient *rpc.Client) ([]string, error) {
	var validators []string
	if err := rpcClient.CallContext(ctx, &validators, "qbft_getValidatorsByBlockNumber", "latest"); err != nil {
		return nil, err
	}
	return validators, nil
}

func printResult(rpcURL string, r *inspectResult) {
	fmt.Println("SCAVIUM NETWORK STATUS")
	fmt.Println("======================")
	fmt.Println("RPC URL:       ", rpcURL)
	fmt.Println("Chain ID:      ", r.ChainID.String())
	fmt.Println("Block Number:  ", r.BlockNumber)
	fmt.Println("Peer Count:    ", r.PeerCount)
	fmt.Println("Gas Price Wei: ", r.GasPriceWei.String())

	switch v := r.Syncing.(type) {
	case bool:
		fmt.Println("Syncing:       ", v)
	default:
		fmt.Printf("Syncing:        %+v\n", v)
	}

	if r.AvgBlockTimeSec > 0 {
		fmt.Printf("Avg Block Time: %.2f sec\n", r.AvgBlockTimeSec)
	} else {
		fmt.Println("Avg Block Time: unavailable")
	}

	if len(r.Validators) > 0 {
		fmt.Println("Validators:")
		for _, v := range r.Validators {
			fmt.Println(" -", v)
		}
	} else {
		fmt.Println("Validators:    unavailable (QBFT RPC API not enabled)")
	}
}
