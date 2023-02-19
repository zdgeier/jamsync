package changestore

import (
	"database/sql"
	"errors"
	"time"
)

func setup(db *sql.DB) error {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS committed_changes (change_id INTEGER, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE IF NOT EXISTS changes (id INTEGER, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);
	`
	_, err := db.Exec(sqlStmt)
	return err
}

func getCurrentChange(db *sql.DB) (uint64, time.Time, error) {
	row := db.QueryRow("SELECT c.id, c.timestamp FROM changes AS c ORDER BY c.id DESC LIMIT 1")
	if row.Err() != nil {
		return 0, time.Time{}, row.Err()
	}

	var id uint64
	var timestamp time.Time
	err := row.Scan(&id, &timestamp)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, time.Time{}, nil
	}
	return id, timestamp, err
}

func addChange(db *sql.DB) (uint64, error) {
	changeId, _, err := getCurrentChange(db)
	if !errors.Is(sql.ErrNoRows, err) && err != nil {
		return 0, err
	}
	newId := changeId + 1
	_, err = db.Exec("INSERT INTO changes(id) VALUES(?)", newId)
	if err != nil {
		return 0, err
	}

	return uint64(newId), nil
}

func commitChange(db *sql.DB, changeId uint64) error {
	_, err := db.Exec("INSERT INTO committed_changes(change_id) VALUES(?)", changeId)
	if err != nil {
		return err
	}

	return err
}

func listCommittedChanges(db *sql.DB) ([]uint64, error) {
	//rows, err := db.Query("SELECT c.change_id FROM committed_changes AS c WHERE timestamp < ? ORDER BY c.timestamp ASC", timestamp)
	rows, err := db.Query("SELECT change_id FROM committed_changes ORDER BY timestamp ASC")
	if err != nil {
		return nil, err
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	defer rows.Close()

	changeIds := make([]uint64, 0)
	for rows.Next() {
		var changeId uint64
		rows.Scan(&changeId)
		changeIds = append(changeIds, changeId)
	}
	return changeIds, nil
}
