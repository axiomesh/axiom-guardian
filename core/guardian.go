package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/axiomesh/axiom-kit/log"
	"github.com/axiomesh/axiom-kit/storage"
	"github.com/axiomesh/axiom-kit/storage/leveldb"
	"github.com/axiomesh/guardian/repo"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
)

const (
	LogChanMaxSize = 1000

	nextFromBlockKey   = "nextFromBlock"
	nextUpgradeVersion = "nextUpgradeVersion"
)

type Guardian struct {
	Ctx    context.Context
	Client Client
	Logger *logrus.Logger
	DB     storage.Storage
	Config *repo.Config

	// Subscribe log
	FromBlock *big.Int
	ToBlock   *big.Int
	Addresses []common.Address
	Topics    [][]common.Hash

	LogChan             chan types.Log
	LogSub              ethereum.Subscription
	nextUpgradeProposal *NodeProposal
	nextUpgradeVersion  string
}

func NewGuardian(ctx context.Context, config *repo.Config, client Client) (*Guardian, error) {
	logger := log.New()
	logger.SetLevel(log.ParseLevel(config.Log.Level))

	var fromBlock, toBlock *big.Int
	if config.Subscribe.FromBlock != 0 {
		fromBlock = big.NewInt(int64(config.Subscribe.FromBlock))
	}

	if config.Subscribe.ToBlock != 0 {
		toBlock = big.NewInt(int64(config.Subscribe.ToBlock))
	}

	var addresses []common.Address
	for _, addr := range config.Subscribe.Addresses {
		addresses = append(addresses, common.HexToAddress(addr))
	}
	var topics [][]common.Hash
	for _, topic := range config.Subscribe.Topics {
		var dstTopic []common.Hash
		for _, s := range topic {
			dstTopic = append(dstTopic, common.HexToHash(s))
		}
		topics = append(topics, dstTopic)
	}

	// new leveldb
	db, err := leveldb.New(filepath.Join(config.RepoRoot, "leveldb"))
	if err != nil {
		return nil, err
	}

	logChan := make(chan types.Log, LogChanMaxSize)

	return &Guardian{
		Ctx:       ctx,
		Client:    client,
		Logger:    logger,
		DB:        db,
		Config:    config,
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: addresses,
		Topics:    topics,
		LogChan:   logChan,
	}, nil
}

func (g *Guardian) Start() error {
	if err := g.fetchHistoryLog(); err != nil {
		return err
	}

	if err := g.subscribeLog(); err != nil {
		return err
	}

	go g.listenEvents()

	go g.downloadAndRestart()

	return nil
}

func (g *Guardian) fetchHistoryLog() error {
	fromBlock := g.getNewestFromBlock()

	// TODO: to block sub from block should be less than 10000
	logs, err := g.Client.FilterLogs(g.Ctx, ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   g.ToBlock,
		Addresses: g.Addresses,
		Topics:    g.Topics,
	})
	if err != nil {
		return err
	}

	g.Logger.Debugf("logs is: %v", logs)

	for _, log := range logs {
		g.handleProposalLog(&log)
	}

	return nil
}

func (g *Guardian) subscribeLog() error {
	var err error
	g.LogSub, err = g.Client.SubscribeFilterLogs(g.Ctx, ethereum.FilterQuery{
		FromBlock: g.FromBlock,
		ToBlock:   g.ToBlock,
		Addresses: g.Addresses,
		Topics:    g.Topics,
	}, g.LogChan)

	return err
}

func (g *Guardian) handleProposalLog(log *types.Log) {
	proposal := &NodeProposal{}
	if err := json.Unmarshal(log.Data, proposal); err != nil {
		g.Logger.Errorf("unmarshal error: %s", err)
		return
	}

	// check proposal is node upgrade and apporved
	if proposal.Type == NodeUpgrade && proposal.Status == Approved {
		g.nextUpgradeProposal = proposal
	}
}

func (g *Guardian) getNewestFromBlock() *big.Int {
	data := g.DB.Get([]byte(nextFromBlockKey))

	if data != nil {
		nextFromBlock := binary.BigEndian.Uint64(data)

		if nextFromBlock > g.FromBlock.Uint64() {
			g.FromBlock = big.NewInt(int64(nextFromBlock))
		}
	}

	return g.FromBlock
}

func (g *Guardian) listenEvents() {
	g.Logger.Info("listen events")

	for {
		select {
		case <-g.Ctx.Done():
			g.Logger.Info("context done")
		case log := <-g.LogChan:
			g.Logger.Infof("subscribe log: %+v", log)
			go func() {
				g.handleProposalLog(&log)
				g.downloadAndRestart()
			}()
		}
	}
}

func (g *Guardian) downloadAndRestart() {
	// first check axiomledger current version if is newest
	currentVersion, err := g.getAxiomLedgerCurrentVersion(filepath.Join(g.Config.AxiomPath, "axiom"))
	if err != nil {
		g.Logger.Errorf("get axiomledger current version error: %s", err)
		return
	}

	newVersion := g.getNextUpgradeVersion()
	if newVersion != "" && currentVersion == newVersion {
		g.Logger.Infof("current version %s is newest, no need upgrade", currentVersion)
		return
	}

	// second download
	downloadFilePath, err := g.download()
	if err != nil {
		g.Logger.Errorf("download error: %s", err)
		return
	}

	// third restart
	if err := g.restart(downloadFilePath); err != nil {
		g.Logger.Errorf("restart error: %s", err)
		return
	}
}

func (g *Guardian) download() (string, error) {
	if g.nextUpgradeProposal == nil {
		g.Logger.Info("nothing to download")
		return "", nil
	}

	downloadUrls := g.nextUpgradeProposal.DownloadUrls
	checkHash := g.nextUpgradeProposal.CheckHash

	maxInt := big.NewInt(int64(len(downloadUrls)))

	if maxInt.Int64() <= 0 {
		return "", errors.New("download url list is empty")
	}

	var filePath string

	handle := func(urls []string) error {
		index, err := rand.Int(rand.Reader, maxInt)
		if err != nil {
			return err
		}

		downloadUrl := downloadUrls[index.Uint64()]
		g.Logger.Debugf("download url: %s", downloadUrl)

		downloadPath := filepath.Join(g.Config.RepoRoot, "download")
		if _, err := os.Stat(downloadPath); err != nil {
			if err := os.Mkdir(downloadPath, 0775); err != nil {
				return err
			}
		}

		resp, err := http.Get(downloadUrl)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get download url error, status code: %v", resp.StatusCode)
		}

		filename := path.Base(downloadUrl)
		filePath = filepath.Join(downloadPath, filename)
		downloadFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		defer downloadFile.Close()

		_, err = io.Copy(downloadFile, resp.Body)
		if err != nil {
			return err
		}

		return nil
	}

	// retry download if failed
	action := func(attempt uint) error {
		if err := handle(downloadUrls); err != nil {
			return err
		}

		return nil
	}
	// TODO: need retry when network down
	if err := retry.Retry(action, strategy.Limit(5), strategy.Backoff(backoff.Fibonacci(5*time.Second))); err != nil {
		return "", err
	}

	if !g.checkFileHash(filePath, checkHash) {
		return "", errors.New("hash check failed")
	}

	g.Logger.Infof("download file hash check passed")

	// get axiomledger version
	axiomLedgerPath := g.decompress(filePath)
	g.Logger.Debugf("axiom ledger path: %s", axiomLedgerPath)
	nextUpgradeVersion, err := g.getAxiomLedgerCurrentVersion(filepath.Join(axiomLedgerPath, "axiom"))
	if err != nil {
		return "", err
	}
	g.nextUpgradeVersion = nextUpgradeVersion

	g.nextUpgradeProposal = nil

	return axiomLedgerPath, nil
}

func (g *Guardian) checkFileHash(filePath, hash string) bool {
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0755)
	if err != nil {
		g.Logger.Errorf("open download file error: %s", err)
		return false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		g.Logger.Errorf("copy file to sha256 error: %s", err)
		return false
	}

	sum := fmt.Sprintf("%x", h.Sum(nil))
	if sum != hash {
		g.Logger.Errorf("file hash mismatch, source file hash: %s, target file hash: %s", hash, sum)
		return false
	}

	return true
}

func (g *Guardian) restart(downloadFilePath string) error {
	if g.nextUpgradeVersion == "" {
		return nil
	}

	// check restart.sh existence
	if _, err := os.Stat(filepath.Join(g.Config.AxiomPath, "restart.sh")); err != nil {
		return err
	}

	// execute restart shell
	execCmd := fmt.Sprintf("cd %s && bash restart.sh %s", g.Config.AxiomPath, filepath.Join(downloadFilePath, "axiom"))

	g.Logger.Debugf("exec restart command: %s", execCmd)

	cmd := exec.Command("bash", "-c", execCmd)
	if _, err := cmd.Output(); err != nil {
		return err
	}

	// record restart version
	g.DB.Put([]byte(nextUpgradeVersion), []byte(g.nextUpgradeVersion))

	// reconnect new axiom after restart
	if err := g.reconnect(); err != nil {
		return err
	}

	g.Logger.Infof("restart successful")
	return nil
}

func (g *Guardian) reconnect() error {
	var client Client
	var err error

	action := func(attempt uint) error {
		client, err = ethclient.DialContext(g.Ctx, g.Config.DialUrl)
		if err != nil {
			return err
		}

		return nil
	}

	if err = retry.Retry(action, strategy.Limit(5), strategy.Backoff(backoff.Fibonacci(5*time.Second))); err != nil {
		return err
	}

	g.Client = client

	if err := g.subscribeLog(); err != nil {
		return err
	}
	return nil
}

func (g *Guardian) Stop() error {
	g.LogSub.Unsubscribe()

	return nil
}

func (g *Guardian) getAxiomLedgerCurrentVersion(p string) (string, error) {
	dir := filepath.Dir(p)

	// check version.sh existence
	if _, err := os.Stat(filepath.Join(dir, "version.sh")); err != nil {
		return "", err
	}

	execCmd := fmt.Sprintf("cd %s && bash version.sh", dir)
	cmd := exec.Command("bash", "-c", execCmd)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	strList := strings.Split(string(out), ": ")
	if len(strList) < 2 {
		return "", errors.New("version command not contains :")
	}
	strList = strings.Split(strList[1], "\n")

	g.Logger.Infof("version is: %s", strList[0])

	return strList[0], nil
}

func (g *Guardian) getNextUpgradeVersion() string {
	versionData := g.DB.Get([]byte(nextUpgradeVersion))
	g.nextUpgradeVersion = string(versionData)
	return g.nextUpgradeVersion
}

func (g *Guardian) decompress(p string) string {
	dir := filepath.Dir(p)
	filename := filepath.Base(p)

	// decompress to new directory
	dstDirName := fmt.Sprintf("axiom-%d", time.Now().Unix())
	dstPath := filepath.Join(dir, dstDirName)
	if err := os.Mkdir(dstPath, 0755); err != nil {
		g.Logger.Errorf("mkdir %s error: %s", dstPath, err)
		return ""
	}

	execCmd := fmt.Sprintf("cd %s && tar -zxvf %s -C ./%s", dir, filename, dstDirName)
	g.Logger.Debugf("execute command: %s", execCmd)
	cmd := exec.Command("bash", "-c", execCmd)

	if _, err := cmd.Output(); err != nil {
		g.Logger.Errorf("decompress file error: %s", err)
		return dstPath
	}

	return dstPath
}
