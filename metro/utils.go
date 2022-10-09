package metro

import (
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func MarshalBlock(block *types.Block) ([]byte, error) {
	pb, err := block.ToProto()
	if err != nil {
		return nil, err
	}
	return pb.Marshal()
}

func UnmarshalBlock(data []byte) (*types.Block, error) {
	var pb tmproto.Block
	err := pb.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return types.BlockFromProto(&pb)

}
