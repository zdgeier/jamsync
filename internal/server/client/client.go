package client

import (
	"bytes"
	"context"
	"io"
	"path/filepath"

	"github.com/cespare/xxhash"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/rsync"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	api       pb.JamsyncAPIClient
	projectId uint64
	changeId  uint64
	committed bool
}

func NewClient(apiClient pb.JamsyncAPIClient, projectId uint64, changeId uint64) *Client {
	return &Client{
		api:       apiClient,
		projectId: projectId,
		changeId:  changeId,
	}
}

func (c *Client) CreateChange() error {
	resp, err := c.api.CreateChange(context.Background(), &pb.CreateChangeRequest{
		ProjectId: c.projectId,
	})
	if err != nil {
		return err
	}
	c.changeId = resp.GetChangeId()
	return err
}

func (c *Client) CommitChange() error {
	_, err := c.api.CommitChange(context.Background(), &pb.CommitChangeRequest{
		ProjectId: c.projectId,
		ChangeId:  c.changeId,
	})
	if err != nil {
		return err
	}
	c.committed = true
	return nil
}

func (c *Client) UploadFile(ctx context.Context, filePath string, sourceReader io.Reader) error {
	blockHashResp, err := c.api.ReadBlockHashes(ctx, &pb.ReadBlockHashesRequest{
		ProjectId: c.projectId,
		ChangeId:  c.changeId,
		PathHash:  pathToHash(filePath),
		ModTime:   timestamppb.Now(),
	})
	if err != nil {
		return err
	}

	opsOut := make(chan *rsync.Operation)
	rsDelta := &rsync.RSync{UniqueHasher: xxhash.New()}
	go func() {
		var blockCt, blockRangeCt, dataCt, bytes int
		defer close(opsOut)
		err := rsDelta.CreateDelta(sourceReader, PbBlockHashesToRsync(blockHashResp.GetBlockHashes()), func(op rsync.Operation) error {
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

	writeStream, err := c.api.WriteOperationStream(ctx)
	if err != nil {
		return err
	}
	sent := 0
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

		err = writeStream.Send(&pb.Operation{
			ProjectId:     c.projectId,
			ChangeId:      c.changeId,
			PathHash:      pathToHash(filePath),
			Type:          opPbType,
			BlockIndex:    op.BlockIndex,
			BlockIndexEnd: op.BlockIndexEnd,
			Data:          op.Data,
		})
		if err != nil {
			return err
		}
		sent += 1
	}
	// We have to send a tombstone if we have not generated any ops (empty file)
	if sent == 0 {
		writeStream.Send(&pb.Operation{
			ProjectId:     c.projectId,
			ChangeId:      c.changeId,
			PathHash:      pathToHash(filePath),
			Type:          pb.Operation_OpData,
			BlockIndex:    0,
			BlockIndexEnd: 0,
			Data:          []byte{},
		})
	}
	_, err = writeStream.CloseAndRecv()
	return err
}

func (c *Client) UpdateFile(ctx context.Context, path string, sourceReader io.Reader) error {
	err := c.CreateChange()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(sourceReader)
	if err != nil {
		return err
	}
	h := xxhash.New()
	h.Write(data)

	newFile := &pb.File{
		ModTime: timestamppb.Now(),
		Dir:     false,
		Hash:    h.Sum64(),
	}

	metadataReader := bytes.NewReader([]byte{})
	metadataResult := new(bytes.Buffer)
	err = c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return err
	}

	remoteFileMetadata.GetFiles()[path] = newFile

	err = c.UploadFile(ctx, path, bytes.NewReader(data))
	if err != nil {
		return err
	}

	metadataBytes, err := proto.Marshal(remoteFileMetadata)
	if err != nil {
		return err
	}

	newMetadataReader := bytes.NewReader(metadataBytes)
	err = c.UploadFile(ctx, ".jamsyncfilelist", newMetadataReader)
	if err != nil {
		return err
	}

	err = c.CommitChange()
	if err != nil {
		return err
	}
	return nil
}

func PbBlockHashesToRsync(pbBlockHashes []*pb.BlockHash) []rsync.BlockHash {
	blockHashes := make([]rsync.BlockHash, 0)
	for _, pbBlockHash := range pbBlockHashes {
		blockHashes = append(blockHashes, rsync.BlockHash{
			Index:      pbBlockHash.GetIndex(),
			StrongHash: pbBlockHash.GetStrongHash(),
			WeakHash:   pbBlockHash.GetWeakHash(),
		})
	}
	return blockHashes
}

func (c *Client) DiffLocalToRemote(ctx context.Context, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return nil, err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return nil, err
	}

	fileMetadataDiff := make(map[string]*pb.FileMetadataDiff_FileDiff, len(remoteFileMetadata.GetFiles()))
	for remoteFilePath := range remoteFileMetadata.GetFiles() {
		fileMetadataDiff[remoteFilePath] = &pb.FileMetadataDiff_FileDiff{
			Type: pb.FileMetadataDiff_Delete,
		}
	}

	for filePath, file := range fileMetadata.GetFiles() {
		var diffFile *pb.File
		diffType := pb.FileMetadataDiff_Delete
		remoteFile, found := remoteFileMetadata.GetFiles()[filePath]
		if found && proto.Equal(file, remoteFile) {
			diffType = pb.FileMetadataDiff_NoOp
		} else if found {
			diffFile = file
			diffType = pb.FileMetadataDiff_Update
		} else {
			diffFile = file
			diffType = pb.FileMetadataDiff_Create
		}

		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: diffType,
			File: diffFile,
		}
	}

	return &pb.FileMetadataDiff{
		Diffs: fileMetadataDiff,
	}, err
}

func (c *Client) DiffRemoteToLocal(ctx context.Context, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return nil, err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return nil, err
	}

	fileMetadataDiff := make(map[string]*pb.FileMetadataDiff_FileDiff, len(fileMetadata.GetFiles()))
	for filePath := range fileMetadata.GetFiles() {
		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: pb.FileMetadataDiff_Delete,
		}
	}

	for filePath, file := range remoteFileMetadata.GetFiles() {
		var diffFile *pb.File
		diffType := pb.FileMetadataDiff_Delete
		remoteFile, found := fileMetadata.GetFiles()[filePath]
		if found && proto.Equal(file, remoteFile) {
			diffType = pb.FileMetadataDiff_NoOp
		} else if found {
			diffFile = file
			diffType = pb.FileMetadataDiff_Update
		} else {
			diffFile = file
			diffType = pb.FileMetadataDiff_Create
		}

		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: diffType,
			File: diffFile,
		}
	}

	return &pb.FileMetadataDiff{
		Diffs: fileMetadataDiff,
	}, err
}

func (c *Client) DownloadFile(ctx context.Context, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
	rs := rsync.RSync{UniqueHasher: xxhash.New()}
	blockHashes := make([]*pb.BlockHash, 0)
	err := rs.CreateSignature(localReader, func(bl rsync.BlockHash) error {
		blockHashes = append(blockHashes, &pb.BlockHash{
			Index:      bl.Index,
			StrongHash: bl.StrongHash,
			WeakHash:   bl.WeakHash,
		})
		return nil
	})
	if err != nil {
		return err
	}

	readFileClient, err := c.api.ReadFile(ctx, &pb.ReadFileRequest{
		ProjectId:   c.projectId,
		ChangeId:    c.changeId,
		PathHash:    pathToHash(filePath),
		ModTime:     timestamppb.Now(),
		BlockHashes: blockHashes,
	})
	if err != nil {
		return err
	}

	numOps := 0
	ops := make(chan rsync.Operation)
	go func() {
		for {
			in, err := readFileClient.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			ops <- rsync.PbOperationToRsync(in)
			numOps += 1
		}
		close(ops)
	}()
	localReader.Seek(0, 0)
	err = rs.ApplyDelta(localWriter, localReader, ops)
	if err != nil {
		return err
	}

	return err
}

func (c *Client) ProjectConfig() *pb.ProjectConfig {
	return &pb.ProjectConfig{
		CurrentChange: c.changeId,
		ProjectId:     c.projectId,
	}
}

func (c *Client) BrowseProject(path string) (*pb.BrowseProjectResponse, error) {
	ctx := context.Background()
	metadataResult := new(bytes.Buffer)
	err := c.DownloadFile(ctx, ".jamsyncfilelist", bytes.NewReader([]byte{}), metadataResult)
	if err != nil {
		return nil, err
	}
	fileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), fileMetadata)
	if err != nil {
		return nil, err
	}

	directoryNames := make([]string, 0, len(fileMetadata.GetFiles()))
	fileNames := make([]string, 0, len(fileMetadata.GetFiles()))
	requestPath := filepath.Clean(path)
	for path, file := range fileMetadata.GetFiles() {
		pathDir := filepath.Dir(path)
		if (path == "" && pathDir == ".") || pathDir == requestPath {
			if file.GetDir() {
				directoryNames = append(directoryNames, filepath.Base(path))
			} else {
				fileNames = append(fileNames, filepath.Base(path))
			}
		}
	}

	return &pb.BrowseProjectResponse{
		Directories: directoryNames,
		Files:       fileNames,
	}, err
}

func pathToHash(path string) uint64 {
	h := xxhash.New()
	h.Write([]byte(path))
	return h.Sum64()
}
