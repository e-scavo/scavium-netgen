package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	log.SetFlags(0)

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("generate key: %v", err)
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := fmt.Sprintf("%x", privateKeyBytes)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot cast public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	fmt.Println("Address:", address.Hex())
	fmt.Println("PrivateKey:", privateKeyHex)
}
