package server

import (
	"bytes"
	"context"
	"io"

	"github.com/cespare/xxhash"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/rsync"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (s JamsyncServer) CreateChange(ctx context.Context, in *pb.CreateChangeRequest) (*pb.CreateChangeResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	changeId, err := s.changestore.AddChange(in.GetProjectId(), userId)
	if err != nil {
		return nil, err
	}
	return &pb.CreateChangeResponse{
		ChangeId: changeId,
	}, nil
}

func (s JamsyncServer) WriteOperationStream(srv pb.JamsyncAPI_WriteOperationStreamServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	projectOwner := ""
	operationProject := uint64(0)
	var projectId, changeId, pathHash uint64
	opLocs := make([]*pb.OperationLocations_OperationLocation, 0)
	for {
		in, err := srv.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data, err := proto.Marshal(in)
		if err != nil {
			return err
		}
		projectId = in.GetProjectId()
		changeId = in.GetChangeId()
		pathHash = in.GetPathHash()
		if operationProject == 0 {
			owner, err := s.db.GetProjectOwner(projectId)
			if err != nil {
				return err
			}
			if userId != owner {
				return status.Errorf(codes.Unauthenticated, "unauthorized")
			}
			projectOwner = owner
			operationProject = projectId
		}

		if operationProject != projectId {
			return status.Errorf(codes.Unauthenticated, "unauthorized")
		}

		offset, length, err := s.opstore.Write(projectId, userId, changeId, pathHash, data)
		if err != nil {
			return err
		}
		operationLocation := &pb.OperationLocations_OperationLocation{
			Offset: offset,
			Length: length,
		}
		opLocs = append(opLocs, operationLocation)
	}
	err = s.oplocstore.InsertOperationLocations(&pb.OperationLocations{
		ProjectId: projectId,
		OwnerId:   projectOwner,
		ChangeId:  changeId,
		PathHash:  pathHash,
		OpLocs:    opLocs,
	})
	if err != nil {
		return err
	}

	return srv.SendAndClose(&pb.WriteOperationStreamResponse{})
}

func (s JamsyncServer) ReadBlockHashes(ctx context.Context, in *pb.ReadBlockHashesRequest) (*pb.ReadBlockHashesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectId() != 1 {
			return nil, err
		}
	}

	targetBuffer, err := s.regenFile(in.GetProjectId(), userId, in.GetPathHash(), in.GetChangeId())
	if err != nil {
		return nil, err
	}

	rs := rsync.RSync{UniqueHasher: xxhash.New()}
	sig := make([]*pb.BlockHash, 0)
	err = rs.CreateSignature(targetBuffer, func(bl rsync.BlockHash) error {
		sig = append(sig, &pb.BlockHash{
			Index:      bl.Index,
			StrongHash: bl.StrongHash,
			WeakHash:   bl.WeakHash,
		})
		return nil
	})
	return &pb.ReadBlockHashesResponse{
		BlockHashes: sig,
	}, err
}

func (s JamsyncServer) regenFile(projectId uint64, userId string, pathHash uint64, changeId uint64) (*bytes.Reader, error) {
	rs := rsync.RSync{UniqueHasher: xxhash.New()}
	targetBuffer := bytes.NewBuffer([]byte{})
	result := new(bytes.Buffer)
	for i := uint64(1); i <= changeId; i++ {
		operationLocations, err := s.oplocstore.ListOperationLocations(projectId, userId, pathHash, i)
		if err != nil {
			return nil, err
		}
		if operationLocations == nil {
			continue
		}
		ops := make([]rsync.Operation, 0, len(operationLocations.GetOpLocs()))
		for _, loc := range operationLocations.GetOpLocs() {
			b, err := s.opstore.Read(projectId, userId, operationLocations.ChangeId, pathHash, loc.GetOffset(), loc.GetLength())
			if err != nil {
				panic(err)
			}

			op := new(pb.Operation)
			err = proto.Unmarshal(b, op)
			if err != nil {
				panic(err)
			}
			ops = append(ops, rsync.PbOperationToRsync(op))
		}
		err = rs.ApplyDeltaBatch(result, bytes.NewReader(targetBuffer.Bytes()), ops)
		if err != nil {
			panic(err)
		}
		targetBuffer.Reset()
		targetBuffer.Write(result.Bytes())
		result.Reset()
	}
	return bytes.NewReader(targetBuffer.Bytes()), nil
}

func (s JamsyncServer) ReadFile(in *pb.ReadFileRequest, srv pb.JamsyncAPI_ReadFileServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		// jamsync
		if in.GetProjectId() != 1 {
			return err
		}
	}

	sourceBuffer, err := s.regenFile(in.GetProjectId(), userId, in.GetPathHash(), in.GetChangeId())
	if err != nil {
		return err
	}

	opsOut := make(chan *rsync.Operation)
	rsDelta := &rsync.RSync{UniqueHasher: xxhash.New()}
	go func() {
		var blockCt, blockRangeCt, dataCt, bytes int
		defer close(opsOut)
		err := rsDelta.CreateDelta(sourceBuffer, rsync.PbBlockHashesToRsync(in.GetBlockHashes()), func(op rsync.Operation) error {
			switch op.Type {
			case rsync.OpBlockRange:
				blockRangeCt++
			case rsync.OpBlock:
				blockCt++
			case rsync.OpData:
				b := make([]byte, len(op.Data))
				copy(b, op.Data)
				op.Data = b
				dataCt++
				bytes += len(op.Data)
			}
			opsOut <- &op
			return nil
		})
		if err != nil {
			panic(err)
		}
	}()

	for op := range opsOut {
		var opPbType pb.Operation_Type
		switch op.Type {
		case rsync.OpBlock:
			opPbType = pb.Operation_OpBlock
		case rsync.OpData:
			opPbType = pb.Operation_OpData
		case rsync.OpHash:
			opPbType = pb.Operation_OpHash
		case rsync.OpBlockRange:
			opPbType = pb.Operation_OpBlockRange
		}

		err = srv.Send(&pb.Operation{
			ProjectId:     in.GetProjectId(),
			ChangeId:      in.GetChangeId(),
			PathHash:      in.GetPathHash(),
			Type:          opPbType,
			BlockIndex:    op.BlockIndex,
			BlockIndexEnd: op.BlockIndexEnd,
			Data:          op.Data,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s JamsyncServer) CommitChange(ctx context.Context, in *pb.CommitChangeRequest) (*pb.CommitChangeResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.changestore.CommitChange(in.GetProjectId(), userId, in.GetChangeId())
	if err != nil {
		return nil, err
	}

	s.hub.Broadcast(&pb.ChangeStreamMessage{
		ProjectId: in.GetProjectId(),
		UserId:    userId,
	})

	return &pb.CommitChangeResponse{}, nil
}

func (s JamsyncServer) ListCommittedChanges(ctx context.Context, in *pb.ListCommittedChangesRequest) (*pb.ListCommittedChangesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectName() != "jamsync" {
			return nil, err
		}
	}

	projectId, err := s.db.GetProjectId(in.GetProjectName(), userId)
	if err != nil {
		return nil, err
	}

	changeIds, err := s.changestore.ListCommittedChanges(projectId, userId)
	if err != nil {
		return nil, err
	}

	return &pb.ListCommittedChangesResponse{
		ChangeIds: changeIds,
	}, nil
}

func (s JamsyncServer) ChangeStream(in *pb.ChangeStreamRequest, srv pb.JamsyncAPI_ChangeStreamServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	client := s.hub.Register(in.ProjectId, userId)

	for changeStreamMessage := range client.Send {
		err = srv.Send(changeStreamMessage)
		if err != nil {
			return err
		}
	}

	return nil
}
