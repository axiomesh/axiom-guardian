package repo

import (
	"encoding/binary"
	"testing"

	"github.com/axiomesh/axiom-kit/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestConfig(t *testing.T) {
	sig := "vote(uint64,uint8,bytes)"

	typeHash := make([]byte, 2)
	binary.BigEndian.PutUint16(typeHash, uint16(1))
	thash := types.NewHash(typeHash)

	t.Logf("type hash: %s", thash.ETHHash().Hex())
	t.Logf("type direct hash: %s", thash.String())

	sigKec := crypto.Keccak256([]byte(sig))
	sigHash := types.NewHash(sigKec)
	t.Logf("sig hash: %s", sigHash.ETHHash().Hex())
	t.Logf("sig direct hash: %s", sigHash.String())
}
