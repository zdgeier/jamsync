package oplocstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	"google.golang.org/protobuf/proto"
)

type LocalOpLocStore struct {
	directory string
}

func (s LocalOpLocStore) opLocDirectory(projectId uint64, ownerId string, changeId uint64) string {
	return fmt.Sprintf("%s/%s/%d/oplocs/%d", s.directory, ownerId, projectId, changeId)
}
func (s LocalOpLocStore) filePath(projectId uint64, ownerId string, changeId uint64, pathHash uint64) string {
	return fmt.Sprintf("%s/%d.locs", s.opLocDirectory(projectId, ownerId, changeId), pathHash)
}

func NewLocalOpLocStore(directory string) LocalOpLocStore {
	return LocalOpLocStore{
		directory: directory,
	}
}

func (s LocalOpLocStore) InsertOperationLocations(opLocs *pb.OperationLocations) error {
	err := os.MkdirAll(s.opLocDirectory(opLocs.GetProjectId(), opLocs.GetOwnerId(), opLocs.GetChangeId()), os.ModePerm)
	if err != nil {
		return err
	}
	filePath := s.filePath(opLocs.GetProjectId(), opLocs.GetOwnerId(), opLocs.GetChangeId(), opLocs.GetPathHash())
	currFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	bytes, err := proto.Marshal(opLocs)
	if err != nil {
		return err
	}
	_, err = currFile.Write(bytes)
	currFile.Close()
	return err
}
func (s LocalOpLocStore) ListOperationLocations(projectId uint64, ownerId string, pathHash uint64, changeId uint64) (opLocs *pb.OperationLocations, err error) {
	filePath := s.filePath(projectId, ownerId, changeId, pathHash)

	_, err = os.Stat(filePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	currFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	currFile.Seek(0, 0)
	_, err = io.Copy(buf, currFile)
	if err != nil {
		return nil, err
	}

	opLocs = &pb.OperationLocations{}
	err = proto.Unmarshal(buf.Bytes(), opLocs)
	return opLocs, err
}
