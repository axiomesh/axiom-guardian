package core

import (
	"context"
	"encoding/json"

	"github.com/axiomesh/guardian/repo"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Client interface {
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)

	SubscribeFilterLogs(context.Context, ethereum.FilterQuery, chan<- types.Log) (ethereum.Subscription, error)
}

var _ Client = (*MockClient)(nil)

type MockClient struct {
}

func (mc *MockClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	log, err := generateLog()
	if err != nil {
		return nil, err
	}

	logs := []types.Log{*log}

	return logs, nil
}

func (mc *MockClient) SubscribeFilterLogs(context.Context, ethereum.FilterQuery, chan<- types.Log) (ethereum.Subscription, error) {
	return &MockSubscription{}, nil
}

func generateLog() (*types.Log, error) {
	nodeProposal := &NodeProposal{
		BaseProposal: BaseProposal{
			ID:          1,
			Type:        NodeUpgrade,
			Strategy:    SimpleStrategy,
			Proposer:    "0xff00000000000000000000000000000000001001",
			Title:       "mock title",
			Desc:        "mock desc",
			BlockNumber: 1000,
			TotalVotes:  4,
			PassVotes:   []string{"0x110000000000000000000000000000000000ffff", "0x220000000000000000000000000000000000ffff", "0x330000000000000000000000000000000000ffff"},
			RejectVotes: nil,
			Status:      Approved,
		},
		DownloadUrls: []string{"http://localhost:9111/axiom-dev.tar.gz", "http://localhost:9112/axiom-dev.tar.gz"},
		CheckHash:    "596d31575d39232ac8b80522e74d7e2c85ce177a0936a38f9616b8ceef3e97d1",
	}

	data, err := json.Marshal(nodeProposal)
	if err != nil {
		return nil, err
	}

	log := &types.Log{
		Address: common.HexToAddress(repo.NodeManagerContractAddr),
		Data:    data,
	}

	return log, nil
}

type MockSubscription struct {
}

func (ms *MockSubscription) Unsubscribe() {
}

func (ms *MockSubscription) Err() <-chan error {
	errChan := make(<-chan error)
	return errChan
}
