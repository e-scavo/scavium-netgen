package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"scavium-netgen/internal/ethutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type wallet struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
	NonceStart uint64
}

type job struct {
	WalletIndex int
	Sequence    int
}

type result struct {
	TxHash         string
	SentAt         time.Time
	ReceiptAt      time.Time
	SendErr        error
	ReceiptErr     error
	Confirmed      bool
	BlockNumber    uint64
	WalletAddress  string
	SequenceNumber int
}

func main() {
	log.SetFlags(0)

	if len(os.Args) != 6 {
		fmt.Println("Usage:")
		fmt.Println("  scavium-stress-test <rpc-url> <keys-file> <tx-per-wallet> <parallel> <value-wei>")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  scavium-stress-test http://191.102.248.174:18545 ./stress_keys.txt 100 20 1")
		os.Exit(1)
	}

	rpcURL := os.Args[1]
	keysFile := os.Args[2]

	txPerWallet, err := strconv.Atoi(os.Args[3])
	if err != nil || txPerWallet <= 0 {
		log.Fatalf("invalid tx-per-wallet: %s", os.Args[3])
	}

	parallel, err := strconv.Atoi(os.Args[4])
	if err != nil || parallel <= 0 {
		log.Fatalf("invalid parallel: %s", os.Args[4])
	}

	valueWei, err := ethutil.ParseAmountWei(os.Args[5])
	if err != nil {
		log.Fatalf("invalid value-wei: %v", err)
	}

	client, err := ethutil.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("chain id: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("gas price: %v", err)
	}

	wallets, err := loadWallets(ctx, client, keysFile)
	if err != nil {
		log.Fatalf("load wallets: %v", err)
	}
	if len(wallets) == 0 {
		log.Fatal("no wallets loaded")
	}

	totalTx := len(wallets) * txPerWallet

	fmt.Println("====================================")
	fmt.Println("SCAVIUM STRESS TEST V2")
	fmt.Println("====================================")
	fmt.Println("RPC:", rpcURL)
	fmt.Println("Wallets:", len(wallets))
	fmt.Println("TX per wallet:", txPerWallet)
	fmt.Println("Total TX:", totalTx)
	fmt.Println("Parallel workers:", parallel)
	fmt.Println("GasPrice:", gasPrice.String())
	fmt.Println("Value per tx:", valueWei.String(), "wei")
	fmt.Println("====================================")

	jobs := make(chan job, totalTx)
	results := make(chan result, totalTx)

	for wi := range wallets {
		for seq := 0; seq < txPerWallet; seq++ {
			jobs <- job{
				WalletIndex: wi,
				Sequence:    seq,
			}
		}
	}
	close(jobs)

	var wg sync.WaitGroup
	started := time.Now()

	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go worker(
			&wg,
			client,
			chainID,
			gasPrice,
			valueWei,
			wallets,
			jobs,
			results,
		)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var sentCount uint64
	var acceptedCount uint64
	var confirmedCount uint64
	var failedSendCount uint64
	var failedReceiptCount uint64
	var totalLatencyNs int64

	for r := range results {
		atomic.AddUint64(&sentCount, 1)

		if r.SendErr != nil {
			atomic.AddUint64(&failedSendCount, 1)
			log.Printf("SEND_FAIL wallet=%s seq=%d err=%v", r.WalletAddress, r.SequenceNumber, r.SendErr)
			continue
		}

		atomic.AddUint64(&acceptedCount, 1)

		if r.Confirmed {
			atomic.AddUint64(&confirmedCount, 1)
			atomic.AddInt64(&totalLatencyNs, r.ReceiptAt.Sub(r.SentAt).Nanoseconds())
		} else {
			atomic.AddUint64(&failedReceiptCount, 1)
			log.Printf("RECEIPT_FAIL wallet=%s seq=%d tx=%s err=%v",
				r.WalletAddress, r.SequenceNumber, r.TxHash, r.ReceiptErr)
		}
	}

	totalDuration := time.Since(started)

	var confirmedTPS float64
	if totalDuration.Seconds() > 0 {
		confirmedTPS = float64(confirmedCount) / totalDuration.Seconds()
	}

	var avgLatency time.Duration
	if confirmedCount > 0 {
		avgLatency = time.Duration(totalLatencyNs / int64(confirmedCount))
	}

	fmt.Println("====================================")
	fmt.Println("RESULTS")
	fmt.Println("====================================")
	fmt.Println("Sent:            ", sentCount)
	fmt.Println("Accepted by RPC: ", acceptedCount)
	fmt.Println("Confirmed:       ", confirmedCount)
	fmt.Println("Send failed:     ", failedSendCount)
	fmt.Println("Receipt failed:  ", failedReceiptCount)
	fmt.Println("Duration:        ", totalDuration)
	fmt.Printf("Confirmed TPS:   %.2f\n", confirmedTPS)
	fmt.Println("Avg latency:     ", avgLatency)
	fmt.Println("====================================")
}

func worker(
	wg *sync.WaitGroup,
	client *ethclient.Client,
	chainID *big.Int,
	gasPrice *big.Int,
	valueWei *big.Int,
	wallets []wallet,
	jobs <-chan job,
	results chan<- result,
) {
	defer wg.Done()

	for j := range jobs {
		w := wallets[j.WalletIndex]
		nonce := w.NonceStart + uint64(j.Sequence)

		tx := types.NewTransaction(
			nonce,
			w.Address, // self transfer
			valueWei,
			21000,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), w.PrivateKey)
		if err != nil {
			results <- result{
				SendErr:        err,
				WalletAddress:  w.Address.Hex(),
				SequenceNumber: j.Sequence,
			}
			continue
		}

		sentAt := time.Now()

		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			results <- result{
				TxHash:         signedTx.Hash().Hex(),
				SentAt:         sentAt,
				SendErr:        err,
				WalletAddress:  w.Address.Hex(),
				SequenceNumber: j.Sequence,
			}
			continue
		}

		receipt, err := ethutil.WaitReceipt(client, signedTx.Hash(), 120*time.Second)
		if err != nil {
			results <- result{
				TxHash:         signedTx.Hash().Hex(),
				SentAt:         sentAt,
				ReceiptErr:     err,
				WalletAddress:  w.Address.Hex(),
				SequenceNumber: j.Sequence,
			}
			continue
		}

		results <- result{
			TxHash:         signedTx.Hash().Hex(),
			SentAt:         sentAt,
			ReceiptAt:      time.Now(),
			Confirmed:      receipt.Status == 1,
			BlockNumber:    receipt.BlockNumber.Uint64(),
			WalletAddress:  w.Address.Hex(),
			SequenceNumber: j.Sequence,
		}
	}
}

func loadWallets(ctx context.Context, client *ethclient.Client, keysFile string) ([]wallet, error) {
	f, err := os.Open(keysFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var wallets []wallet

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pk, err := ethutil.HexToECDSA(line)
		if err != nil {
			return nil, fmt.Errorf("invalid key %q: %w", line, err)
		}

		addr := ethutil.AddressFromPrivateKey(pk)

		nonce, err := client.PendingNonceAt(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("pending nonce for %s: %w", addr.Hex(), err)
		}

		wallets = append(wallets, wallet{
			PrivateKey: pk,
			Address:    addr,
			NonceStart: nonce,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return wallets, nil
}
