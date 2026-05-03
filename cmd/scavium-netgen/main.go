package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type NodeRole string

const (
	RoleBootnode  NodeRole = "bootnode"
	RoleValidator NodeRole = "validator"
	RoleRPC       NodeRole = "rpc"
)

type Node struct {
	Name     string   `json:"name"`
	Role     NodeRole `json:"role"`
	IP       string   `json:"ip"`
	P2PPort  int      `json:"p2p_port"`
	RPCPort  int      `json:"rpc_port,omitempty"`
	WSPort   int      `json:"ws_port,omitempty"`
	Metrics  int      `json:"metrics_port"`
	PrivKey  string   `json:"-"`
	EnodePub string   `json:"enode_pub,omitempty"`
	Address  string   `json:"address,omitempty"`
}

type Account struct {
	Name       string `json:"name"`
	PrivateKey string `json:"-"`
	Address    string `json:"address"`
	BalanceDec string `json:"balance_dec"`
	BalanceHex string `json:"balance_hex"`
}

type Config struct {
	BaseDir            string
	ChainName          string
	NetworkName        string
	ChainID            uint32
	GatewayIP          string
	GenesisPath        string
	BesuPath           string
	GenerateExtraData  bool
	GenerateSystemd    bool
	GenerateAccounts   bool
	TargetGasLimit     string
	GenesisGasLimitHex string
	BlockPeriod        int
	RequestTimeout     int
	EpochLength        int

	FaucetAddress   string
	DeployerAddress string

	FaucetName   string
	DeployerName string

	Verbose          bool
	Debug            bool
	NoKeygen         bool
	OverwriteConfigs bool
	InventoryOut     string
	HostsOut         string
	AccountsOut      string
	NodesFile        string

	BaseP2PPort     int
	BaseRPCPort     int
	BaseWSPort      int
	BaseMetricsPort int

	Nodes      []Node
	Bootnodes  []Node
	Validators []Node
	RPCs       []Node
	Accounts   []Account
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]

	switch subcmd {
	case "init":
		cfg, err := parseArgs(os.Args[2:])
		fatalIf(err)
		runInit(cfg)
	case "regen":
		cfg, err := parseArgs(os.Args[2:])
		fatalIf(err)
		runRegen(cfg)
	case "inventory":
		cfg, err := parseArgs(os.Args[2:])
		fatalIf(err)
		runInventory(cfg)
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Printf("Unknown subcommand: %s\n\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`scavium-netgen

Usage:
  scavium-netgen <subcommand> [flags]

Subcommands:
  init        Generate full network structure, keys, accounts, genesis, configs, static-nodes, systemd, readme
  regen       Regenerate derived artifacts (configs, static-nodes, readme, systemd, genesis, accounts inventory)
  inventory   Generate inventory/hosts/readme/accounts inventory only

Examples:
  scavium-netgen init \
    --base /opt/scavium/testnet-b \
    --chain-name SCAVIUM \
    --network testnet-b \
    --gateway 191.102.248.190 \
    --besu /usr/local/bin/besu \
    --p2p-port 31303 \
    --rpc-port 18545 \
    --ws-port 18546 \
    --metrics-port 19545 \
    --inventory-out /opt/scavium/testnet-b/inventory.json \
    --accounts-out /opt/scavium/testnet-b/accounts/accounts.json \
    --hosts-out /opt/scavium/testnet-b/hosts.generated

  scavium-netgen regen \
    --base /opt/scavium/testnet-b \
    --nodes-file ./nodes.json

  scavium-netgen inventory \
    --base /opt/scavium/testnet-b \
    --nodes-file ./nodes.json \
    --inventory-out /opt/scavium/testnet-b/inventory.json`)
}

func runInit(cfg *Config) {
	logf(cfg, "INFO", "[init] starting")

	err := ensureBase(cfg)
	fatalIf(err)

	if cfg.GenerateAccounts {
		err = ensureAccounts(cfg)
		fatalIf(err)
	}

	err = ensureNodeKeys(cfg)
	fatalIf(err)

	buildDerivedFields(cfg)

	err = writeQBFTValidatorsJSON(cfg)
	fatalIf(err)

	extraData, generated, err := maybeGenerateQBFTExtraData(cfg)
	fatalIf(err)

	err = writeGenesis(cfg, extraData, generated)
	fatalIf(err)

	err = writeStaticNodes(cfg)
	fatalIf(err)

	err = writeNodeConfigs(cfg)
	fatalIf(err)

	if cfg.GenerateSystemd {
		err = writeSystemdTemplate(cfg)
		fatalIf(err)
	}

	err = writeReadme(cfg, generated)
	fatalIf(err)

	if cfg.InventoryOut != "" {
		err = writeInventoryJSON(cfg, cfg.InventoryOut)
		fatalIf(err)
	}

	if cfg.AccountsOut != "" {
		err = writeAccountsInventoryJSON(cfg, cfg.AccountsOut)
		fatalIf(err)
	}

	if cfg.HostsOut != "" {
		err = writeHostsFile(cfg, cfg.HostsOut)
		fatalIf(err)
	}

	printSummary(cfg)
}

func runRegen(cfg *Config) {
	logf(cfg, "INFO", "[regen] starting")

	err := ensureBase(cfg)
	fatalIf(err)

	if cfg.GenerateAccounts {
		err = ensureAccounts(cfg)
		fatalIf(err)
	}

	err = ensureNodeKeys(cfg)
	fatalIf(err)

	buildDerivedFields(cfg)

	err = writeQBFTValidatorsJSON(cfg)
	fatalIf(err)

	extraData, generated, err := maybeGenerateQBFTExtraData(cfg)
	fatalIf(err)

	err = writeGenesis(cfg, extraData, generated)
	fatalIf(err)

	err = writeStaticNodes(cfg)
	fatalIf(err)

	err = writeNodeConfigs(cfg)
	fatalIf(err)

	if cfg.GenerateSystemd {
		err = writeSystemdTemplate(cfg)
		fatalIf(err)
	}

	err = writeReadme(cfg, generated)
	fatalIf(err)

	if cfg.InventoryOut != "" {
		err = writeInventoryJSON(cfg, cfg.InventoryOut)
		fatalIf(err)
	}

	if cfg.AccountsOut != "" {
		err = writeAccountsInventoryJSON(cfg, cfg.AccountsOut)
		fatalIf(err)
	}

	if cfg.HostsOut != "" {
		err = writeHostsFile(cfg, cfg.HostsOut)
		fatalIf(err)
	}

	printSummary(cfg)
}

func runInventory(cfg *Config) {
	logf(cfg, "INFO", "[inventory] starting")

	err := ensureBase(cfg)
	fatalIf(err)

	if cfg.GenerateAccounts {
		err = ensureAccounts(cfg)
		fatalIf(err)
	}

	err = ensureNodeKeys(cfg)
	fatalIf(err)

	buildDerivedFields(cfg)

	if cfg.InventoryOut != "" {
		err = writeInventoryJSON(cfg, cfg.InventoryOut)
		fatalIf(err)
	}

	if cfg.AccountsOut != "" {
		err = writeAccountsInventoryJSON(cfg, cfg.AccountsOut)
		fatalIf(err)
	}

	if cfg.HostsOut != "" {
		err = writeHostsFile(cfg, cfg.HostsOut)
		fatalIf(err)
	}

	err = writeReadme(cfg, false)
	fatalIf(err)

	printSummary(cfg)
}

func parseArgs(args []string) (*Config, error) {
	cfg := &Config{
		BaseDir:            "/opt/scavium/testnet",
		ChainName:          "SCAVIUM",
		NetworkName:        "testnet",
		GatewayIP:          "191.102.248.190",
		BesuPath:           "/usr/local/bin/besu",
		GenerateExtraData:  true,
		GenerateSystemd:    true,
		GenerateAccounts:   true,
		TargetGasLimit:     "30000000",
		GenesisGasLimitHex: "0x1C9C380",
		BlockPeriod:        2,
		RequestTimeout:     4,
		EpochLength:        30000,
		FaucetName:         "faucet",
		DeployerName:       "deployer",
		Verbose:            true,
		Debug:              false,
		NoKeygen:           false,
		OverwriteConfigs:   true,
		BaseP2PPort:        31303,
		BaseRPCPort:        18545,
		BaseWSPort:         18546,
		BaseMetricsPort:    19545,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--base":
			i++
			cfg.BaseDir = args[i]
		case "--chain-name":
			i++
			cfg.ChainName = args[i]
		case "--network":
			i++
			cfg.NetworkName = args[i]
		case "--gateway":
			i++
			cfg.GatewayIP = args[i]
		case "--besu":
			i++
			cfg.BesuPath = args[i]
		case "--generate-extradata":
			i++
			cfg.GenerateExtraData = parseBool(args[i])
		case "--generate-systemd":
			i++
			cfg.GenerateSystemd = parseBool(args[i])
		case "--generate-accounts":
			i++
			cfg.GenerateAccounts = parseBool(args[i])
		case "--faucet":
			i++
			cfg.FaucetAddress = strings.TrimSpace(args[i])
		case "--deployer":
			i++
			cfg.DeployerAddress = strings.TrimSpace(args[i])
		case "--faucet-name":
			i++
			cfg.FaucetName = strings.TrimSpace(args[i])
		case "--deployer-name":
			i++
			cfg.DeployerName = strings.TrimSpace(args[i])
		case "--verbose":
			i++
			cfg.Verbose = parseBool(args[i])
		case "--debug":
			i++
			cfg.Debug = parseBool(args[i])
		case "--no-keygen":
			i++
			cfg.NoKeygen = parseBool(args[i])
		case "--overwrite-configs":
			i++
			cfg.OverwriteConfigs = parseBool(args[i])
		case "--inventory-out":
			i++
			cfg.InventoryOut = strings.TrimSpace(args[i])
		case "--accounts-out":
			i++
			cfg.AccountsOut = strings.TrimSpace(args[i])
		case "--hosts-out":
			i++
			cfg.HostsOut = strings.TrimSpace(args[i])
		case "--nodes-file":
			i++
			cfg.NodesFile = strings.TrimSpace(args[i])
		case "--p2p-port":
			i++
			cfg.BaseP2PPort = mustAtoi(args[i], "p2p-port")
		case "--rpc-port":
			i++
			cfg.BaseRPCPort = mustAtoi(args[i], "rpc-port")
		case "--ws-port":
			i++
			cfg.BaseWSPort = mustAtoi(args[i], "ws-port")
		case "--metrics-port":
			i++
			cfg.BaseMetricsPort = mustAtoi(args[i], "metrics-port")
		default:
			return nil, fmt.Errorf("unknown flag: %s", a)
		}
	}

	cfg.ChainID = deriveChainID(cfg.ChainName, cfg.NetworkName)
	cfg.GenesisPath = filepath.Join(cfg.BaseDir, "genesis.json")

	if cfg.AccountsOut == "" {
		cfg.AccountsOut = filepath.Join(cfg.BaseDir, "accounts", "accounts.json")
	}

	if cfg.NodesFile != "" {
		nodes, err := loadNodesFromFile(cfg.NodesFile, cfg.BaseP2PPort, cfg.BaseRPCPort, cfg.BaseWSPort, cfg.BaseMetricsPort)
		if err != nil {
			return nil, err
		}
		cfg.Nodes = nodes
	} else {
		cfg.Nodes = defaultNodes(cfg.BaseP2PPort, cfg.BaseRPCPort, cfg.BaseWSPort, cfg.BaseMetricsPort)
	}

	cfg.Accounts = defaultAccounts()

	return cfg, nil
}

func loadNodesFromFile(path string, p2pPort, rpcPort, wsPort, metricsPort int) ([]Node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var nodes []Node
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("invalid nodes-file %s: %w", path, err)
	}

	for i := range nodes {
		if nodes[i].Name == "" {
			return nil, fmt.Errorf("nodes-file: node at index %d has empty name", i)
		}
		if nodes[i].IP == "" {
			return nil, fmt.Errorf("nodes-file: node %s has empty ip", nodes[i].Name)
		}
		if nodes[i].Role == "" {
			return nil, fmt.Errorf("nodes-file: node %s has empty role", nodes[i].Name)
		}

		nodes[i].P2PPort = p2pPort
		nodes[i].Metrics = metricsPort

		if nodes[i].Role == RoleRPC {
			nodes[i].RPCPort = rpcPort
			nodes[i].WSPort = wsPort
		}
	}

	return nodes, nil
}

func defaultNodes(p2pPort, rpcPort, wsPort, metricsPort int) []Node {
	return []Node{
		{Name: "B01", Role: RoleBootnode, IP: "191.102.248.161", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "B02", Role: RoleBootnode, IP: "191.102.248.162", P2PPort: p2pPort, Metrics: metricsPort},

		{Name: "V01", Role: RoleValidator, IP: "191.102.248.163", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V02", Role: RoleValidator, IP: "191.102.248.164", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V03", Role: RoleValidator, IP: "191.102.248.165", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V04", Role: RoleValidator, IP: "191.102.248.166", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V05", Role: RoleValidator, IP: "191.102.248.167", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V06", Role: RoleValidator, IP: "191.102.248.168", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V07", Role: RoleValidator, IP: "191.102.248.169", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V08", Role: RoleValidator, IP: "191.102.248.170", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V09", Role: RoleValidator, IP: "191.102.248.171", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V10", Role: RoleValidator, IP: "191.102.248.172", P2PPort: p2pPort, Metrics: metricsPort},
		{Name: "V11", Role: RoleValidator, IP: "191.102.248.173", P2PPort: p2pPort, Metrics: metricsPort},

		{Name: "R01", Role: RoleRPC, IP: "191.102.248.174", P2PPort: p2pPort, RPCPort: rpcPort, WSPort: wsPort, Metrics: metricsPort},
		{Name: "R02", Role: RoleRPC, IP: "191.102.248.175", P2PPort: p2pPort, RPCPort: rpcPort, WSPort: wsPort, Metrics: metricsPort},
	}
}

func defaultAccounts() []Account {
	return []Account{
		newDefaultAccount("faucet", "10000000"),
		newDefaultAccount("deployer", "1000000"),
		newDefaultAccount("treasury", "100000000"),
		newDefaultAccount("ops", "1000000"),
		newDefaultAccount("tester_01", "100000"),
		newDefaultAccount("tester_02", "100000"),
	}
}

func newDefaultAccount(name, balanceDec string) Account {
	return Account{
		Name:       name,
		BalanceDec: balanceDec,
		BalanceHex: decimalToHexWei(balanceDec, 18),
	}
}

func ensureBase(cfg *Config) error {
	logf(cfg, "INFO", "[ensureBase] ensuring base directories under %s", cfg.BaseDir)

	dirs := []string{
		cfg.BaseDir,
		filepath.Join(cfg.BaseDir, "network"),
		filepath.Join(cfg.BaseDir, "nodes"),
		filepath.Join(cfg.BaseDir, "accounts"),
	}
	for _, d := range dirs {
		logf(cfg, "DEBUG", "[ensureBase] mkdir -p %s", d)
		if err := os.MkdirAll(d, 0o750); err != nil {
			return err
		}
	}
	for _, n := range cfg.Nodes {
		nodeDataDir := filepath.Join(cfg.BaseDir, "nodes", n.Name, "data")
		logf(cfg, "DEBUG", "[ensureBase] mkdir -p %s", nodeDataDir)
		if err := os.MkdirAll(nodeDataDir, 0o750); err != nil {
			return err
		}
	}
	return nil
}

func ensureAccounts(cfg *Config) error {
	logf(cfg, "INFO", "[ensureAccounts] ensuring %d accounts", len(cfg.Accounts))

	for i := range cfg.Accounts {
		accDir := filepath.Join(cfg.BaseDir, "accounts")
		keyPath := filepath.Join(accDir, cfg.Accounts[i].Name+".key")

		keyHex, err := readOrCreateNodeKey(cfg, keyPath)
		if err != nil {
			return fmt.Errorf("account %s: %w", cfg.Accounts[i].Name, err)
		}
		cfg.Accounts[i].PrivateKey = keyHex

		priv, err := gethcrypto.HexToECDSA(keyHex)
		if err != nil {
			return fmt.Errorf("account %s invalid private key: %w", cfg.Accounts[i].Name, err)
		}

		cfg.Accounts[i].Address = gethcrypto.PubkeyToAddress(priv.PublicKey).Hex()

		addrPath := filepath.Join(accDir, cfg.Accounts[i].Name+".address")
		if err := os.WriteFile(addrPath, []byte(cfg.Accounts[i].Address+"\n"), 0o640); err != nil {
			return err
		}

		metaPath := filepath.Join(accDir, cfg.Accounts[i].Name+".json")
		meta, err := json.MarshalIndent(map[string]string{
			"name":        cfg.Accounts[i].Name,
			"address":     cfg.Accounts[i].Address,
			"balance_dec": cfg.Accounts[i].BalanceDec,
			"balance_hex": cfg.Accounts[i].BalanceHex,
		}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(metaPath, append(meta, '\n'), 0o640); err != nil {
			return err
		}

		logf(cfg, "INFO", "[ensureAccounts] account=%s address=%s balance=%s", cfg.Accounts[i].Name, cfg.Accounts[i].Address, cfg.Accounts[i].BalanceDec)
	}

	return nil
}

func ensureNodeKeys(cfg *Config) error {
	logf(cfg, "INFO", "[ensureNodeKeys] ensuring node keys")

	for i := range cfg.Nodes {
		nodeDir := filepath.Join(cfg.BaseDir, "nodes", cfg.Nodes[i].Name)
		keyPath := filepath.Join(nodeDir, "data", "key")

		keyHex, err := readOrCreateNodeKey(cfg, keyPath)
		if err != nil {
			return fmt.Errorf("%s: %w", cfg.Nodes[i].Name, err)
		}
		cfg.Nodes[i].PrivKey = keyHex

		priv, err := gethcrypto.HexToECDSA(keyHex)
		if err != nil {
			return fmt.Errorf("%s invalid private key: %w", cfg.Nodes[i].Name, err)
		}

		pubBytes := gethcrypto.FromECDSAPub(&priv.PublicKey)
		if len(pubBytes) != 65 {
			return fmt.Errorf("%s invalid public key length %d", cfg.Nodes[i].Name, len(pubBytes))
		}

		cfg.Nodes[i].EnodePub = hex.EncodeToString(pubBytes[1:])
		cfg.Nodes[i].Address = gethcrypto.PubkeyToAddress(priv.PublicKey).Hex()

		pubPath := filepath.Join(nodeDir, "data", "key.pub")
		if err := os.WriteFile(pubPath, []byte(cfg.Nodes[i].EnodePub+"\n"), 0o640); err != nil {
			return err
		}

		logf(cfg, "INFO", "[ensureNodeKeys] node=%s role=%s address=%s", cfg.Nodes[i].Name, cfg.Nodes[i].Role, cfg.Nodes[i].Address)
	}
	return nil
}

func buildDerivedFields(cfg *Config) {
	cfg.Bootnodes = nil
	cfg.Validators = nil
	cfg.RPCs = nil

	for _, n := range cfg.Nodes {
		switch n.Role {
		case RoleBootnode:
			cfg.Bootnodes = append(cfg.Bootnodes, n)
		case RoleValidator:
			cfg.Validators = append(cfg.Validators, n)
		case RoleRPC:
			cfg.RPCs = append(cfg.RPCs, n)
		}
	}

	if cfg.FaucetAddress == "" {
		addr, ok := findAccountAddress(cfg.Accounts, cfg.FaucetName)
		if !ok {
			fatalIf(fmt.Errorf("faucet account %q not found", cfg.FaucetName))
		}
		cfg.FaucetAddress = addr
	}
	if cfg.DeployerAddress == "" {
		addr, ok := findAccountAddress(cfg.Accounts, cfg.DeployerName)
		if !ok {
			fatalIf(fmt.Errorf("deployer account %q not found", cfg.DeployerName))
		}
		cfg.DeployerAddress = addr
	}

	logf(cfg, "INFO", "[buildDerivedFields] bootnodes=%d validators=%d rpcs=%d accounts=%d", len(cfg.Bootnodes), len(cfg.Validators), len(cfg.RPCs), len(cfg.Accounts))
	logf(cfg, "INFO", "[buildDerivedFields] faucet=%s deployer=%s", cfg.FaucetAddress, cfg.DeployerAddress)
}

func findAccountAddress(accounts []Account, name string) (string, bool) {
	for _, a := range accounts {
		if a.Name == name {
			return a.Address, true
		}
	}
	return "", false
}

func writeQBFTValidatorsJSON(cfg *Config) error {
	path := filepath.Join(cfg.BaseDir, "qbft_validators.json")
	logf(cfg, "INFO", "[writeQBFTValidatorsJSON] writing %s", path)

	var addrs []string
	for _, v := range cfg.Validators {
		addrs = append(addrs, strings.ToLower(v.Address))
	}
	sort.Strings(addrs)

	b, err := json.MarshalIndent(addrs, "", "  ")
	if err != nil {
		return err
	}
	return writeFileIfAllowed(cfg, path, append(b, '\n'), 0o640)
}

func maybeGenerateQBFTExtraData(cfg *Config) (string, bool, error) {
	if !cfg.GenerateExtraData {
		return "0x<REEMPLAZAR_CON_QBFT_EXTRADATA>", false, nil
	}

	_, err := exec.LookPath(cfg.BesuPath)
	if err != nil {
		logf(cfg, "INFO", "[maybeGenerateQBFTExtraData] besu not found, leaving placeholder")
		return "0x<REEMPLAZAR_CON_QBFT_EXTRADATA>", false, nil
	}

	in := filepath.Join(cfg.BaseDir, "qbft_validators.json")
	out := filepath.Join(cfg.BaseDir, "extraData.txt")

	cmd := exec.Command(
		cfg.BesuPath,
		"rlp", "encode",
		"--from="+in,
		"--to="+out,
		"--type=QBFT_EXTRA_DATA",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	logf(cfg, "INFO", "[maybeGenerateQBFTExtraData] running besu rlp encode")
	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("besu rlp encode failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		return "", false, err
	}

	encoded := strings.TrimSpace(string(raw))
	if encoded == "" {
		return "", false, errors.New("empty extraData generated by besu")
	}

	if !strings.HasPrefix(encoded, "0x") {
		encoded = "0x" + encoded
	}

	logf(cfg, "INFO", "[maybeGenerateQBFTExtraData] extraData generated len=%d", len(encoded))
	return encoded, true, nil
}

func writeGenesis(cfg *Config, extraData string, generated bool) error {
	type allocEntry struct {
		Balance string `json:"balance"`
	}
	type genesis struct {
		Config struct {
			ChainID uint32 `json:"chainId"`
			London  int    `json:"londonBlock"`
			QBFT    struct {
				BlockPeriod    int `json:"blockperiodseconds"`
				EpochLength    int `json:"epochlength"`
				RequestTimeout int `json:"requesttimeoutseconds"`
			} `json:"qbft"`
		} `json:"config"`
		Nonce      string                `json:"nonce"`
		Timestamp  string                `json:"timestamp"`
		ExtraData  string                `json:"extraData"`
		GasLimit   string                `json:"gasLimit"`
		Difficulty string                `json:"difficulty"`
		MixHash    string                `json:"mixHash"`
		Coinbase   string                `json:"coinbase"`
		Alloc      map[string]allocEntry `json:"alloc"`
	}

	var g genesis
	g.Config.ChainID = cfg.ChainID
	g.Config.London = 0
	g.Config.QBFT.BlockPeriod = cfg.BlockPeriod
	g.Config.QBFT.EpochLength = cfg.EpochLength
	g.Config.QBFT.RequestTimeout = cfg.RequestTimeout
	g.Nonce = "0x0"
	g.Timestamp = "0x0"
	g.ExtraData = extraData
	g.GasLimit = cfg.GenesisGasLimitHex
	g.Difficulty = "0x1"
	g.MixHash = "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365"
	g.Coinbase = "0x0000000000000000000000000000000000000000"
	g.Alloc = map[string]allocEntry{}

	for _, acc := range cfg.Accounts {
		g.Alloc[acc.Address] = allocEntry{Balance: acc.BalanceHex}
		logf(cfg, "INFO", "[writeGenesis] alloc account=%s address=%s balance=%s", acc.Name, acc.Address, acc.BalanceDec)
	}

	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}

	logf(cfg, "INFO", "[writeGenesis] writing %s generatedExtraData=%v", cfg.GenesisPath, generated)
	return writeFileIfAllowed(cfg, cfg.GenesisPath, append(b, '\n'), 0o640)
}

func writeStaticNodes(cfg *Config) error {
	var staticNodes []string
	for _, b := range cfg.Bootnodes {
		staticNodes = append(staticNodes, fmt.Sprintf("enode://%s@%s:%d", b.EnodePub, b.IP, b.P2PPort))
	}

	content, err := json.MarshalIndent(staticNodes, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	netPath := filepath.Join(cfg.BaseDir, "network", "static-nodes.json")
	logf(cfg, "INFO", "[writeStaticNodes] writing %s", netPath)
	if err := writeFileIfAllowed(cfg, netPath, content, 0o640); err != nil {
		return err
	}

	for _, n := range cfg.Nodes {
		dst := filepath.Join(cfg.BaseDir, "nodes", n.Name, "data", "static-nodes.json")
		if err := writeFileIfAllowed(cfg, dst, content, 0o640); err != nil {
			return err
		}
	}
	return nil
}

func writeNodeConfigs(cfg *Config) error {
	bootEnodes := make([]string, 0, len(cfg.Bootnodes))
	for _, b := range cfg.Bootnodes {
		bootEnodes = append(bootEnodes, fmt.Sprintf("enode://%s@%s:%d", b.EnodePub, b.IP, b.P2PPort))
	}

	for _, n := range cfg.Nodes {
		nodeDir := filepath.Join(cfg.BaseDir, "nodes", n.Name)
		cfgPath := filepath.Join(nodeDir, "config.toml")

		var rendered string
		switch n.Role {
		case RoleBootnode:
			rendered = renderBootnodeConfig(cfg, n)
		case RoleValidator:
			rendered = renderValidatorConfig(cfg, n, bootEnodes)
		case RoleRPC:
			rendered = renderRPCConfig(cfg, n, bootEnodes)
		default:
			return fmt.Errorf("unknown role for %s", n.Name)
		}

		logf(cfg, "INFO", "[writeNodeConfigs] writing %s", cfgPath)
		if err := writeFileIfAllowed(cfg, cfgPath, []byte(rendered), 0o640); err != nil {
			return err
		}
	}
	return nil
}

func renderBootnodeConfig(cfg *Config, n Node) string {
	return strings.TrimSpace(fmt.Sprintf(`
data-path="data"
genesis-file=%q

data-storage-format="BONSAI"

p2p-host="0.0.0.0"
p2p-port=%d

max-peers=100

rpc-http-enabled=false
rpc-ws-enabled=false

metrics-enabled=true
metrics-host="0.0.0.0"
metrics-port=%d

host-allowlist=["127.0.0.1","localhost"]

logging="INFO"
`, cfg.GenesisPath, n.P2PPort, n.Metrics)) + "\n"
}

func renderValidatorConfig(cfg *Config, n Node, bootEnodes []string) string {
	return strings.TrimSpace(fmt.Sprintf(`
data-path="data"
genesis-file=%q

data-storage-format="BONSAI"

p2p-host="0.0.0.0"
p2p-port=%d

bootnodes=%s

max-peers=100
target-gas-limit=%q

rpc-http-enabled=false
rpc-ws-enabled=false

metrics-enabled=true
metrics-host="0.0.0.0"
metrics-port=%d

host-allowlist=["127.0.0.1","localhost"]

logging="INFO"
`, cfg.GenesisPath, n.P2PPort, tomlArray(bootEnodes), cfg.TargetGasLimit, n.Metrics)) + "\n"
}

func renderRPCConfig(cfg *Config, n Node, bootEnodes []string) string {
	return strings.TrimSpace(fmt.Sprintf(`
data-path="data"
genesis-file=%q

data-storage-format="BONSAI"

p2p-host="0.0.0.0"
p2p-port=%d

bootnodes=%s

max-peers=150
target-gas-limit=%q

rpc-http-enabled=true
rpc-http-host="0.0.0.0"
rpc-http-port=%d
rpc-http-api=["ETH","NET","WEB3","TXPOOL","QBFT"]
host-allowlist=["*"]

rpc-ws-enabled=true
rpc-ws-host="0.0.0.0"
rpc-ws-port=%d
rpc-ws-api=["ETH","NET","WEB3","TXPOOL","QBFT"]

tx-pool="layered"
tx-pool-layer-max-capacity="256000000"
tx-pool-max-future-by-sender=64
tx-pool-price-bump=10
tx-pool-min-gas-price="1000"

metrics-enabled=true
metrics-host="0.0.0.0"
metrics-port=%d

logging="INFO"
`, cfg.GenesisPath, n.P2PPort, tomlArray(bootEnodes), cfg.TargetGasLimit, n.RPCPort, n.WSPort, n.Metrics)) + "\n"
}

func writeSystemdTemplate(cfg *Config) error {
	path := filepath.Join(cfg.BaseDir, "scavium-besu@.service")
	unit := `[Unit]
Description=SCAVIUM Besu Node (%i)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=scavium
Group=scavium

WorkingDirectory=` + filepath.Join(cfg.BaseDir, "nodes") + `/%i
ExecStart=` + cfg.BesuPath + ` --config-file=` + filepath.Join(cfg.BaseDir, "nodes") + `/%i/config.toml

Restart=always
RestartSec=5
TimeoutStopSec=60
LimitNOFILE=1048576

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=` + filepath.Join(cfg.BaseDir, "nodes") + `/%i

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
	logf(cfg, "INFO", "[writeSystemdTemplate] writing %s", path)
	return writeFileIfAllowed(cfg, path, []byte(unit), 0o640)
}

func writeReadme(cfg *Config, extraDataGenerated bool) error {
	type row struct {
		Name    string
		Role    string
		IP      string
		Address string
	}
	type accRow struct {
		Name    string
		Address string
		Balance string
	}

	var rows []row
	for _, n := range cfg.Nodes {
		rows = append(rows, row{
			Name:    n.Name,
			Role:    string(n.Role),
			IP:      n.IP,
			Address: n.Address,
		})
	}

	var accRows []accRow
	for _, a := range cfg.Accounts {
		accRows = append(accRows, accRow{
			Name:    a.Name,
			Address: a.Address,
			Balance: a.BalanceDec,
		})
	}

	tpl := template.Must(template.New("readme").Parse(readmeTemplate))
	var buf bytes.Buffer
	err := tpl.Execute(&buf, map[string]any{
		"ChainName":          cfg.ChainName,
		"NetworkName":        cfg.NetworkName,
		"ChainID":            cfg.ChainID,
		"GatewayIP":          cfg.GatewayIP,
		"GenesisPath":        cfg.GenesisPath,
		"BaseDir":            cfg.BaseDir,
		"ExtraDataGenerated": extraDataGenerated,
		"FaucetAddress":      cfg.FaucetAddress,
		"DeployerAddress":    cfg.DeployerAddress,
		"Rows":               rows,
		"Accounts":           accRows,
		"P2PPort":            cfg.BaseP2PPort,
		"RPCPort":            cfg.BaseRPCPort,
		"WSPort":             cfg.BaseWSPort,
		"MetricsPort":        cfg.BaseMetricsPort,
	})
	if err != nil {
		return err
	}

	path := filepath.Join(cfg.BaseDir, "README.generated.md")
	logf(cfg, "INFO", "[writeReadme] writing %s", path)
	return writeFileIfAllowed(cfg, path, buf.Bytes(), 0o640)
}

func writeInventoryJSON(cfg *Config, path string) error {
	type inventory struct {
		ChainName   string `json:"chain_name"`
		NetworkName string `json:"network_name"`
		ChainID     uint32 `json:"chain_id"`
		GatewayIP   string `json:"gateway_ip"`
		P2PPort     int    `json:"p2p_port"`
		RPCPort     int    `json:"rpc_port"`
		WSPort      int    `json:"ws_port"`
		MetricsPort int    `json:"metrics_port"`
		Nodes       []Node `json:"nodes"`
	}

	out := inventory{
		ChainName:   cfg.ChainName,
		NetworkName: cfg.NetworkName,
		ChainID:     cfg.ChainID,
		GatewayIP:   cfg.GatewayIP,
		P2PPort:     cfg.BaseP2PPort,
		RPCPort:     cfg.BaseRPCPort,
		WSPort:      cfg.BaseWSPort,
		MetricsPort: cfg.BaseMetricsPort,
		Nodes:       cfg.Nodes,
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	logf(cfg, "INFO", "[writeInventoryJSON] writing %s", path)
	return writeFileIfAllowed(cfg, path, append(b, '\n'), 0o640)
}

func writeAccountsInventoryJSON(cfg *Config, path string) error {
	type accountsInventory struct {
		ChainName      string    `json:"chain_name"`
		NetworkName    string    `json:"network_name"`
		ChainID        uint32    `json:"chain_id"`
		FaucetName     string    `json:"faucet_name"`
		FaucetAddress  string    `json:"faucet_address"`
		DeployerName   string    `json:"deployer_name"`
		DeployerAdress string    `json:"deployer_address"`
		Accounts       []Account `json:"accounts"`
	}

	out := accountsInventory{
		ChainName:      cfg.ChainName,
		NetworkName:    cfg.NetworkName,
		ChainID:        cfg.ChainID,
		FaucetName:     cfg.FaucetName,
		FaucetAddress:  cfg.FaucetAddress,
		DeployerName:   cfg.DeployerName,
		DeployerAdress: cfg.DeployerAddress,
		Accounts:       cfg.Accounts,
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	logf(cfg, "INFO", "[writeAccountsInventoryJSON] writing %s", path)
	return writeFileIfAllowed(cfg, path, append(b, '\n'), 0o640)
}

func writeHostsFile(cfg *Config, path string) error {
	var lines []string
	for _, n := range cfg.Nodes {
		lines = append(lines, fmt.Sprintf("%s scavium-%s", n.IP, strings.ToLower(n.Name)))
	}
	content := strings.Join(lines, "\n") + "\n"
	logf(cfg, "INFO", "[writeHostsFile] writing %s", path)
	return writeFileIfAllowed(cfg, path, []byte(content), 0o640)
}

const readmeTemplate = `# {{ .ChainName }} / {{ .NetworkName }}

- Chain ID: {{ .ChainID }}
- Gateway: {{ .GatewayIP }}
- Base dir: {{ .BaseDir }}
- Genesis: {{ .GenesisPath }}
- Faucet alloc: {{ .FaucetAddress }}
- Deployer alloc: {{ .DeployerAddress }}
- extraData auto-generated: {{ .ExtraDataGenerated }}

## Default Ports

- P2P: {{ .P2PPort }}
- RPC HTTP: {{ .RPCPort }}
- RPC WS: {{ .WSPort }}
- Metrics: {{ .MetricsPort }}

## Nodes

| Name | Role | IP | Address |
|---|---|---|---|
{{- range .Rows }}
| {{ .Name }} | {{ .Role }} | {{ .IP }} | {{ .Address }} |
{{- end }}

## Accounts

| Name | Address | Balance |
|---|---|---|
{{- range .Accounts }}
| {{ .Name }} | {{ .Address }} | {{ .Balance }} |
{{- end }}

## Systemd template
Generated at:
- {{ .BaseDir }}/scavium-besu@.service

Install manually to:
- /etc/systemd/system/scavium-besu@.service

## Suggested start order
1. B01, B02
2. V01..V11
3. R01, R02
`

func deriveChainID(name, network string) uint32 {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(name)) + "-" + strings.ToLower(strings.TrimSpace(network))))
	v := binary.BigEndian.Uint32(sum[:4])
	const min uint32 = 10_000_000
	const span uint32 = 1_990_000_000
	return min + (v % span)
}

func readOrCreateNodeKey(cfg *Config, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		s := strings.TrimSpace(string(b))
		s = strings.TrimPrefix(strings.ToLower(s), "0x")
		if len(s) != 64 {
			return "", fmt.Errorf("existing key in %s is not 64 hex chars", path)
		}
		if _, err := hex.DecodeString(s); err != nil {
			return "", fmt.Errorf("existing key in %s is invalid hex: %w", path, err)
		}
		logf(cfg, "DEBUG", "[readOrCreateNodeKey] existing key reused %s", path)
		return s, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	if cfg.NoKeygen {
		return "", fmt.Errorf("missing key in %s and --no-keygen enabled", path)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	keyHex := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(keyHex+"\n"), 0o600); err != nil {
		return "", err
	}

	logf(cfg, "INFO", "[readOrCreateNodeKey] new key created %s", path)
	return keyHex, nil
}

func writeFileIfAllowed(cfg *Config, path string, data []byte, mode os.FileMode) error {
	if !cfg.OverwriteConfigs {
		if _, err := os.Stat(path); err == nil {
			logf(cfg, "INFO", "[writeFileIfAllowed] skipping existing file: %s", path)
			return nil
		}
	}
	return os.WriteFile(path, data, mode)
}

func tomlArray(items []string) string {
	var quoted []string
	for _, v := range items {
		quoted = append(quoted, fmt.Sprintf("%q", v))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func mustAtoi(v, name string) int {
	var out int
	_, err := fmt.Sscanf(v, "%d", &out)
	if err != nil {
		fatalIf(fmt.Errorf("invalid %s: %s", name, v))
	}
	return out
}

func decimalToHexWei(amount string, decimals int) string {
	base := new(big.Int)
	_, ok := base.SetString(strings.TrimSpace(amount), 10)
	if !ok {
		fatalIf(fmt.Errorf("invalid decimal amount: %s", amount))
	}

	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wei := new(big.Int).Mul(base, multiplier)
	return "0x" + strings.ToUpper(wei.Text(16))
}

func printSummary(cfg *Config) {
	fmt.Println("OK: SCAVIUM netgen completed")
	fmt.Printf("Base dir: %s\n", cfg.BaseDir)
	fmt.Printf("Chain: %s / %s\n", cfg.ChainName, cfg.NetworkName)
	fmt.Printf("Chain ID: %d\n", cfg.ChainID)
	fmt.Printf("Genesis: %s\n", cfg.GenesisPath)
	fmt.Printf("Ports: p2p=%d rpc=%d ws=%d metrics=%d\n", cfg.BaseP2PPort, cfg.BaseRPCPort, cfg.BaseWSPort, cfg.BaseMetricsPort)
	fmt.Printf("Nodes: bootnodes=%d validators=%d rpcs=%d total=%d\n", len(cfg.Bootnodes), len(cfg.Validators), len(cfg.RPCs), len(cfg.Nodes))
	fmt.Printf("Accounts: %d (faucet=%s deployer=%s)\n", len(cfg.Accounts), cfg.FaucetAddress, cfg.DeployerAddress)
}

func logf(cfg *Config, level string, format string, args ...any) {
	switch level {
	case "DEBUG":
		if cfg != nil && cfg.Debug {
			log.Printf(format, args...)
		}
	case "INFO":
		if cfg == nil || cfg.Verbose || cfg.Debug {
			log.Printf(format, args...)
		}
	default:
		log.Printf(format, args...)
	}
}

func fatalIf(err error) {
	if err != nil {
		log.Printf("[fatal] %v", err)
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
