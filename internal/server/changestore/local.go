package changestore

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

type LocalChangeStore struct {
	dbs map[uint64]*sql.DB
}

func NewLocalChangeStore() LocalChangeStore {
	return LocalChangeStore{
		dbs: make(map[uint64]*sql.DB, 0),
	}
}

func (s LocalChangeStore) getLocalProjectDB(projectId uint64, ownerId string) (*sql.DB, error) {
	if db, ok := s.dbs[projectId]; ok {
		return db, nil
	}

	dir := fmt.Sprintf("jb/%s/%d", ownerId, projectId)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	localDB, err := sql.Open("sqlite3", dir+"/jamsyncproject.db")
	if err != nil {
		return nil, err
	}
	err = setup(localDB)
	if err != nil {
		return nil, err
	}

	s.dbs[projectId] = localDB
	return localDB, nil
}

func (s LocalChangeStore) AddChange(projectId uint64, ownerId string) (uint64, error) {
	db, err := s.getLocalProjectDB(projectId, ownerId)
	if err != nil {
		return 0, err
	}
	return addChange(db)
}
func (s LocalChangeStore) GetCurrentChange(projectId uint64, ownerId string) (uint64, time.Time, error) {
	db, err := s.getLocalProjectDB(projectId, ownerId)
	if err != nil {
		return 0, time.Time{}, err
	}
	return getCurrentChange(db)
}
func (s LocalChangeStore) CommitChange(projectId uint64, ownerId string, changeId uint64) error {
	db, err := s.getLocalProjectDB(projectId, ownerId)
	if err != nil {
		return err
	}
	return commitChange(db, changeId)
}
func (s LocalChangeStore) ListCommittedChanges(projectId uint64, ownerId string) ([]uint64, error) {
	db, err := s.getLocalProjectDB(projectId, ownerId)
	if err != nil {
		return nil, err
	}
	return listCommittedChanges(db)
}
