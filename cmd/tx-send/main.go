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

func main() {
	log.SetFlags(0)

	if len(os.Args) != 5 {
		fmt.Println("Usage:")
		fmt.Println("  tx-send <rpc-url> <private-key> <to-address> <value-wei>")
		os.Exit(1)
	}

	rpcURL := os.Args[1]
	privateKeyHex := os.Args[2]
	toAddressHex := os.Args[3]
	valueWeiText := os.Args[4]

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

	fmt.Println("From:", fromAddress.Hex())
	fmt.Println("To:", toAddress.Hex())
	fmt.Println("Nonce:", nonce)
	fmt.Println("GasPrice:", gasPrice.String())
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
