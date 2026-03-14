package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"scavium-netgen/internal/ethutil"

	"github.com/ethereum/go-ethereum/core/types"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing env: %s", key)
	}
	return v
}

func main() {
	log.SetFlags(0)

	if len(os.Args) != 3 {
		fmt.Println("Usage:")
		fmt.Println("  SCAVIUM_RPC_URL=http://127.0.0.1:18545 SCAVIUM_FAUCET_KEY=<key> faucet-send <to-address> <value-wei>")
		os.Exit(1)
	}

	rpcURL := mustEnv("SCAVIUM_RPC_URL")
	privateKeyHex := mustEnv("SCAVIUM_FAUCET_KEY")
	toAddressHex := os.Args[1]
	valueWeiText := os.Args[2]

	client, err := ethutil.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer client.Close()

	privateKey, err := ethutil.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}

	fromAddress := ethutil.AddressFromPrivateKey(privateKey)
	toAddress := ethutil.MustAddress(toAddressHex)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("chain id: %v", err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatalf("nonce: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatalf("gas price: %v", err)
	}

	value, err := ethutil.ParseAmountWei(valueWeiText)
	if err != nil {
		log.Fatalf("value: %v", err)
	}

	tx := types.NewTransaction(
		nonce,
		toAddress,
		value,
		21000,
		gasPrice,
		nil,
	)

	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privateKey)
	if err != nil {
		log.Fatalf("sign tx: %v", err)
	}

	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		log.Fatalf("send tx: %v", err)
	}

	fmt.Println("Faucet:", fromAddress.Hex())
	fmt.Println("To:", toAddress.Hex())
	fmt.Println("ValueWei:", value.String())
	fmt.Println("TxHash:", signedTx.Hash().Hex())

	receipt, err := ethutil.WaitReceipt(client, signedTx.Hash(), 90*time.Second)
	if err != nil {
		log.Fatalf("wait receipt: %v", err)
	}

	fmt.Println("ReceiptStatus:", receipt.Status)
	fmt.Println("BlockNumber:", receipt.BlockNumber.String())
	fmt.Println("GasUsed:", receipt.GasUsed)
}
