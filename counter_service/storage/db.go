package storage

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type DB struct {
	Conn *sql.DB
}

func NewDB(dataSourceName string) *DB {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping the database: %v", err)
	}

	return &DB{Conn: db}
}

func (db *DB) GetCounter(userID string) (int64, error) {
	var count int64
	err := db.Conn.QueryRow("SELECT unread_count FROM message_counters WHERE user_id = $1", userID).Scan(&count)
	return count, err
}

func (db *DB) UpdateCounter(userID string, count int64) error {
	_, err := db.Conn.Exec("UPDATE message_counters SET unread_count = $1 WHERE user_id = $2", count, userID)
	return err
}
