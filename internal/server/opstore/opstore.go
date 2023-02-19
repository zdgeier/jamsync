package opstore

import (
	"fmt"
	"os"
)

type LocalStore struct {
	directory     string
	openFileCache map[string]*os.File
}

func NewLocalStore(directory string) LocalStore {
	return LocalStore{
		directory:     directory,
		openFileCache: make(map[string]*os.File),
	}
}

func (s LocalStore) changeDirectory(projectId uint64, ownerId string) string {
	return fmt.Sprintf("%s/%s/%d/opdata", s.directory, ownerId, projectId)
}
func (s LocalStore) filePath(projectId uint64, ownerId string, changeId uint64, pathHash uint64) string {
	return fmt.Sprintf("%s/%d.jb", s.changeDirectory(projectId, ownerId), pathHash)
}
func (s LocalStore) Read(projectId uint64, ownerId string, changeId uint64, pathHash uint64, offset uint64, length uint64) (data []byte, err error) {
	filePath := s.filePath(projectId, ownerId, changeId, pathHash)
	currFile, cached := s.openFileCache[filePath]
	if !cached {
		currFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, err
		}
		s.openFileCache[filePath] = currFile
	}
	b := make([]byte, length)
	_, err = currFile.ReadAt(b, int64(offset))
	if err != nil {
		return nil, err
	}
	return b, nil
}
func (s LocalStore) Write(projectId uint64, ownerId string, changeId uint64, pathHash uint64, data []byte) (offset uint64, length uint64, err error) {
	err = os.MkdirAll(s.changeDirectory(projectId, ownerId), os.ModePerm)
	if err != nil {
		return 0, 0, err
	}
	filePath := s.filePath(projectId, ownerId, changeId, pathHash)
	currFile, cached := s.openFileCache[filePath]
	if !cached {
		currFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return 0, 0, err
		}
		s.openFileCache[filePath] = currFile
	}
	info, err := currFile.Stat()
	if err != nil {
		return 0, 0, err
	}
	writtenBytes, err := currFile.Write(data)
	if err != nil {
		return 0, 0, err
	}
	return uint64(info.Size()), uint64(writtenBytes), nil
}
