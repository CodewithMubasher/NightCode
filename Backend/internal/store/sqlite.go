package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type MessageRow struct {
	ID        string
	ChatID    string
	Role      string
	Segments  json.RawMessage
	CreatedAt time.Time
}

type ChatRow struct {
	ID          string
	WorkspaceID string
	Title       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			role TEXT NOT NULL,
			segments TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
	`)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertChat creates or updates a chat row.
func (s *Store) UpsertChat(id, workspaceID, title string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO chats (id, workspace_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title=excluded.title, updated_at=excluded.updated_at
	`, id, workspaceID, title, now, now)
	if err != nil {
		log.Printf("upsert chat error: %v", err)
	}
	return err
}

// InsertMessage persists a message with segments JSON.
func (s *Store) InsertMessage(id, chatID, role string, segments json.RawMessage) error {
	_, err := s.db.Exec(`
		INSERT INTO messages (id, chat_id, role, segments)
		VALUES (?, ?, ?, ?)
	`, id, chatID, role, string(segments))
	if err != nil {
		log.Printf("insert message error: %v", err)
	}
	return err
}

// GetChatsForWorkspace returns all chats for a workspace.
func (s *Store) GetChatsForWorkspace(workspaceID string) ([]ChatRow, error) {
	rows, err := s.db.Query(
		`SELECT id, workspace_id, title, created_at, updated_at FROM chats WHERE workspace_id = ? ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []ChatRow
	for rows.Next() {
		var c ChatRow
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

// GetMessagesForChat returns all messages for a chat.
func (s *Store) GetMessagesForChat(chatID string) ([]MessageRow, error) {
	rows, err := s.db.Query(
		`SELECT id, chat_id, role, segments, created_at FROM messages WHERE chat_id = ? ORDER BY created_at ASC`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []MessageRow
	for rows.Next() {
		var m MessageRow
		var segStr string
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &segStr, &m.CreatedAt); err != nil {
			log.Printf("scan message error: %v", err)
			return nil, err
		}
		m.Segments = json.RawMessage(segStr)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetChat returns a single chat.
func (s *Store) GetChat(id string) (*ChatRow, error) {
	var c ChatRow
	err := s.db.QueryRow(
		`SELECT id, workspace_id, title, created_at, updated_at FROM chats WHERE id = ?`, id,
	).Scan(&c.ID, &c.WorkspaceID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
