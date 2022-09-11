package metro

import (
	"bytes"
	"errors"
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	metroproto "github.com/tendermint/tendermint/proto/tendermint/metro"
	"github.com/tendermint/tendermint/types"
)

// Block defines the atomic unit of a Tendermint blockchain.
type SimpleBlock struct {
	mtx tmsync.Mutex

	types.Header `json:"header"`
	types.Data   `json:"data"`
	Evidence     types.EvidenceData `json:"evidence"`
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
// Further validation is done using state#ValidateBlock.
func (b *SimpleBlock) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()

	if err := b.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// NOTE: b.Data.Txs may be nil, but b.Data.Hash() still works fine.
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return fmt.Errorf(
			"wrong Header.DataHash. Expected %v, got %v",
			b.Data.Hash(),
			b.DataHash,
		)
	}

	// NOTE: b.Evidence.Evidence may be nil, but we're just looping.
	for i, ev := range b.Evidence.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}

	if !bytes.Equal(b.EvidenceHash, b.Evidence.Hash()) {
		return fmt.Errorf("wrong Header.EvidenceHash. Expected %v, got %v",
			b.EvidenceHash,
			b.Evidence.Hash(),
		)
	}

	return nil
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *SimpleBlock) fillHeader() {
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
	if b.EvidenceHash == nil {
		b.EvidenceHash = b.Evidence.Hash()
	}
}

// Hash computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *SimpleBlock) Hash() tmbytes.HexBytes {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.fillHeader()
	return b.Header.Hash()
}

// // MakePartSet returns a PartSet containing parts of a serialized block.
// // This is the form in which the block is gossipped to peers.
// // CONTRACT: partSize is greater than zero.
// func (b *SimpleBlock) MakePartSet(partSize uint32) *types.PartSet {
// 	if b == nil {
// 		return nil
// 	}
// 	b.mtx.Lock()
// 	defer b.mtx.Unlock()

// 	pbb, err := b.ToProto()
// 	if err != nil {
// 		panic(err)
// 	}
// 	bz, err := proto.Marshal(pbb)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return types.NewPartSetFromData(bz, partSize)
// }

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *SimpleBlock) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size returns size of the block in bytes.
func (b *SimpleBlock) Size() int {
	pbb, err := b.ToProto()
	if err != nil {
		return 0
	}

	return pbb.Size()
}

// String returns a string representation of the block
//
// See StringIndented.
func (b *SimpleBlock) String() string {
	return b.StringIndented("")
}

// StringIndented returns an indented String.
//
// Header
// Data
// Evidence
// Hash
func (b *SimpleBlock) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Evidence.StringIndented(indent+"  "),
		indent, b.Hash())
}

// StringShort returns a shortened string representation of the block.
func (b *SimpleBlock) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%X", b.Hash())
}

// ToProto converts Block to protobuf
func (b *SimpleBlock) ToProto() (*metroproto.SimpleBlock, error) {
	if b == nil {
		return nil, errors.New("nil Block")
	}

	pb := new(metroproto.SimpleBlock)

	pb.Header = *b.Header.ToProto()
	pb.Data = b.Data.ToProto()

	protoEvidence, err := b.Evidence.ToProto()
	if err != nil {
		return nil, err
	}
	pb.Evidence = *protoEvidence

	return pb, nil
}

// FromProto sets a protobuf Block to the given pointer.
// It returns an error if the block is invalid.
func BlockFromProto(bp *metroproto.SimpleBlock) (*SimpleBlock, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(SimpleBlock)
	h, err := types.HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.Header = h
	data, err := types.DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.Data = data
	if err := b.Evidence.FromProto(&bp.Evidence); err != nil {
		return nil, err
	}

	return b, b.ValidateBasic()
}
