package core

type ProposalStatus uint8

const (
	Voting ProposalStatus = iota
	Approved
	Rejected
)

type ProposalType uint8

const (
	// CouncilElect is a proposal for elect the council
	CouncilElect ProposalType = iota

	// NodeUpgrade is a proposal for update or upgrade the node
	NodeUpgrade

	// NodeAdd is a proposal for adding a new node
	NodeAdd

	// NodeRemove is a proposal for removing a node
	NodeRemove
)

type ProposalStrategy uint8

const (
	// SimpleStrategy means proposal is approved if pass votes is greater than half of total votes
	SimpleStrategy ProposalStrategy = iota
)

type BaseProposal struct {
	ID          uint64
	Type        ProposalType
	Strategy    ProposalStrategy
	Proposer    string
	Title       string
	Desc        string
	BlockNumber uint64

	// totalVotes is total votes for this proposal
	// attention: some users may not vote for this proposal
	TotalVotes uint64

	// passVotes record user address for passed vote
	PassVotes []string

	RejectVotes []string
	Status      ProposalStatus
}

type NodeProposal struct {
	BaseProposal
	DownloadUrls []string
	CheckHash    string
}
