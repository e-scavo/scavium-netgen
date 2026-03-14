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
		fmt.Println("  wallet-balance <rpc-url> <address>")
		os.Exit(1)
	}

	rpcURL := os.Args[1]
	address := common.HexToAddress(os.Args[2])

	client, err := ethutil.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer client.Close()

	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		log.Fatalf("get balance: %v", err)
	}

	fmt.Println("Address:", address.Hex())
	fmt.Println("BalanceWei:", balance.String())
}
