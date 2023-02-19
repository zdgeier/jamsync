// Author: Zach Geier zach@jamsync.dev
package rsync

import "github.com/zdgeier/jamsync/gen/pb"

func PbBlockHashesToRsync(pbBlockHashes []*pb.BlockHash) []BlockHash {
	blockHashes := make([]BlockHash, 0)
	for _, pbBlockHash := range pbBlockHashes {
		blockHashes = append(blockHashes, BlockHash{
			Index:      pbBlockHash.GetIndex(),
			StrongHash: pbBlockHash.GetStrongHash(),
			WeakHash:   pbBlockHash.GetWeakHash(),
		})
	}
	return blockHashes
}

func PbOperationToRsync(op *pb.Operation) Operation {
	var opType OpType
	switch op.Type {
	case pb.Operation_OpBlock:
		opType = OpBlock
	case pb.Operation_OpData:
		opType = OpData
	case pb.Operation_OpHash:
		opType = OpHash
	case pb.Operation_OpBlockRange:
		opType = OpBlockRange
	}

	return Operation{
		Type:          opType,
		BlockIndex:    op.GetBlockIndex(),
		BlockIndexEnd: op.GetBlockIndexEnd(),
		Data:          op.GetData(),
	}
}
