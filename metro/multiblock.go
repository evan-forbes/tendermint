package metro

import (
	"bytes"
	"errors"
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	metroproto "github.com/tendermint/tendermint/proto/tendermint/metro"
	"github.com/tendermint/tendermint/types"
)

// MultiBlock defines the atomic unit of a Tendermint blockchain.
type MultiBlock struct {
	Blocks     []*SimpleBlock
	LastCommit *types.Commit `json:"last_commit"`
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
// Further validation is done using state#ValidateBlock.
func (b *MultiBlock) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}

	if len(b.Blocks) == 0 {
		return errors.New("multiblock must have at least one block")
	}

	var err error
	for _, sb := range b.Blocks {
		err = sb.ValidateBasic()
		if err != nil {
			return err
		}
	}

	// Validate the last commit and its hash.
	if b.LastCommit == nil {
		return errors.New("nil LastCommit")
	}
	if err := b.LastCommit.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastCommit: %v", err)
	}

	lastBlock := b.Blocks[len(b.Blocks)-1]

	if !bytes.Equal(lastBlock.LastCommitHash, b.LastCommit.Hash()) {
		return fmt.Errorf("wrong Header.LastCommitHash. Expected %v, got %v",
			b.LastCommit.Hash(),
			lastBlock.LastCommitHash,
		)
	}

	return nil
}

// Hash computes and returns the block hash of the last simple block. If the
// block is incomplete, block hash is nil for safety.
func (b *MultiBlock) Hash() tmbytes.HexBytes {
	if b == nil {
		return nil
	}

	if len(b.Blocks) == 0 {
		return nil
	}

	if b.LastCommit == nil {
		return nil
	}

	lastBlock := b.Blocks[len(b.Blocks)-1]

	return lastBlock.Header.Hash()
}

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *MultiBlock) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size returns size of the block in bytes.
func (b *MultiBlock) Size() int {
	pbb, err := b.ToProto()
	if err != nil {
		return 0
	}

	return pbb.Size()
}

// String returns a string representation of the block
//
// See StringIndented.
func (b *MultiBlock) String() string {
	return b.StringShort()
}

// StringShort returns a shortened string representation of the block.
func (b *MultiBlock) StringShort() string {
	if b == nil {
		return "nil-MultiBlock"
	}
	return fmt.Sprintf("MultiBlock#%X", b.Hash())
}

// ToProto converts MultiBlock to protobuf
func (b *MultiBlock) ToProto() (*metroproto.MultiBlock, error) {
	if b == nil {
		return nil, errors.New("nil MultiBlock")
	}

	pBlocks := make([]metroproto.SimpleBlock, len(b.Blocks))
	for i, sb := range b.Blocks {
		psb, err := sb.ToProto()
		if err != nil {
			return nil, err
		}
		pBlocks[i] = *psb
	}

	return &metroproto.MultiBlock{
		Blocks:     pBlocks,
		LastCommit: b.LastCommit.ToProto(),
	}, nil
}

// FromProto sets a protobuf MultiBlock to the given pointer.
// It returns an error if the block is invalid.
func MultiBlockFromProto(bp *metroproto.MultiBlock) (*MultiBlock, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(MultiBlock)

	if bp.LastCommit != nil {
		lc, err := types.CommitFromProto(bp.LastCommit)
		if err != nil {
			return nil, err
		}
		b.LastCommit = lc
	}

	return b, b.ValidateBasic()
}
