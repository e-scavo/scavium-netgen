package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"scavium-netgen/internal/ethutil"

	"github.com/ethereum/go-ethereum/core/types"
)

type sendRequest struct {
	To       string `json:"to"`
	ValueWei string `json:"valueWei"`
}

type sendResponse struct {
	TxHash string `json:"txHash"`
}

var (
	rpcURL     string
	privateKey string
)

func main() {

	rpcURL = mustEnv("SCAVIUM_RPC_URL")
	privateKey = mustEnv("SCAVIUM_FAUCET_KEY")

	addr := envOr("SCAVIUM_BIND_ADDR", ":8080")

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/send", sendHandler)

	log.Println("SCAVIUM Faucet listening on", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{
		"status": "ok",
	}

	json.NewEncoder(w).Encode(resp)
}

func sendHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req sendRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	client, err := ethutil.Dial(rpcURL)
	if err != nil {
		http.Error(w, "rpc connection failed", 500)
		return
	}
	defer client.Close()

	privKey, err := ethutil.HexToECDSA(privateKey)
	if err != nil {
		http.Error(w, "invalid faucet key", 500)
		return
	}

	from := ethutil.AddressFromPrivateKey(privKey)
	to := ethutil.MustAddress(req.To)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		http.Error(w, fmt.Sprintf("chain id error: %+v", err), 500)
		return
	}

	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		http.Error(w, fmt.Sprintf("nonce error: %+v", err), 500)
		return
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		http.Error(w, "gas price error", 500)
		return
	}

	value, err := ethutil.ParseAmountWei(req.ValueWei)
	if err != nil {
		http.Error(w, "invalid value", 400)
		return
	}

	tx := types.NewTransaction(
		nonce,
		to,
		value,
		21000,
		gasPrice,
		nil,
	)

	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privKey)
	if err != nil {
		http.Error(w, "sign tx error", 500)
		return
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		http.Error(w, "send tx error", 500)
		return
	}

	resp := sendResponse{
		TxHash: signedTx.Hash().Hex(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func mustEnv(key string) string {

	v := os.Getenv(key)

	if v == "" {
		log.Fatalf("missing env: %s", key)
	}

	return v
}

func envOr(key string, fallback string) string {

	v := os.Getenv(key)

	if v == "" {
		return fallback
	}

	return v
}
