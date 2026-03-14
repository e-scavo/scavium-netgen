package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"scavium-netgen/internal/ethutil"

	"github.com/ethereum/go-ethereum/common"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) != 3 {
		fmt.Println("Usage:")
		fmt.Println("  tx-receipt <rpc-url> <tx-hash>")
		os.Exit(1)
	}

	rpcURL := os.Args[1]
	txHash := common.HexToHash(os.Args[2])

	client, err := ethutil.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer client.Close()

	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		log.Fatalf("receipt: %v", err)
	}

	fmt.Println("TxHash:", txHash.Hex())
	fmt.Println("Status:", receipt.Status)
	fmt.Println("BlockNumber:", receipt.BlockNumber.String())
	fmt.Println("GasUsed:", receipt.GasUsed)
	fmt.Println("TransactionIndex:", receipt.TransactionIndex)
}
