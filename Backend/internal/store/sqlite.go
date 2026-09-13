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
	ID        string          `json:"id"`
	ChatID    string          `json:"chat_id"`
	Role      string          `json:"role"`
	Segments  json.RawMessage `json:"segments"`
	CreatedAt time.Time       `json:"created_at"`
}

type ChatRow struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceRow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
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
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tool_calls (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			message_id TEXT NOT NULL DEFAULT '',
			tool_call_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			input TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL DEFAULT 0,
			completed_at INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			name TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

// InsertToolCall persists a single tool call audit row.
func (s *Store) InsertToolCall(id, chatID, messageID, toolCallID, name, status, input, output, errStr string, startedAt, completedAt int64) error {
	_, err := s.db.Exec(`
		INSERT INTO tool_calls (id, chat_id, message_id, tool_call_id, name, status, input, output, error, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, chatID, messageID, toolCallID, name, status, input, output, errStr, startedAt, completedAt)
	if err != nil {
		log.Printf("insert tool_call error: %v", err)
	}
	return err
}

// InsertArtifact persists an artifact row.
func (s *Store) InsertArtifact(id, chatID, name, artifactType, language, content string) error {
	_, err := s.db.Exec(`
		INSERT INTO artifacts (id, chat_id, name, artifact_type, language, content)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, chatID, name, artifactType, language, content)
	if err != nil {
		log.Printf("insert artifact error: %v", err)
	}
	return err
}

// UpsertWorkspace creates or updates a workspace.
func (s *Store) UpsertWorkspace(id, name, description string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO workspaces (id, name, description, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description
	`, id, name, description, now)
	if err != nil {
		log.Printf("upsert workspace error: %v", err)
	}
	return err
}

// GetWorkspace returns a single workspace.
func (s *Store) GetWorkspace(id string) (*WorkspaceRow, error) {
	var w WorkspaceRow
	err := s.db.QueryRow(
		`SELECT id, name, description, created_at FROM workspaces WHERE id = ?`, id,
	).Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetWorkspaces returns all workspaces.
func (s *Store) GetWorkspaces() ([]WorkspaceRow, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, created_at FROM workspaces ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []WorkspaceRow
	for rows.Next() {
		var w WorkspaceRow
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, rows.Err()
}

// DeleteWorkspace deletes a workspace.
func (s *Store) DeleteWorkspace(id string) error {
	_, err := s.db.Exec(`DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		log.Printf("delete workspace error: %v", err)
	}
	return err
}

// ArtifactRow represents a stored artifact.
type ArtifactRow struct {
	ID           string    `json:"id"`
	ChatID       string    `json:"chat_id"`
	Name         string    `json:"name"`
	ArtifactType string    `json:"artifact_type"`
	Language     string    `json:"language"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetArtifactsForChat returns all artifacts for a chat.
func (s *Store) GetArtifactsForChat(chatID string) ([]ArtifactRow, error) {
	rows, err := s.db.Query(
		`SELECT id, chat_id, name, artifact_type, language, content, created_at FROM artifacts WHERE chat_id = ? ORDER BY created_at ASC`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []ArtifactRow
	for rows.Next() {
		var a ArtifactRow
		if err := rows.Scan(&a.ID, &a.ChatID, &a.Name, &a.ArtifactType, &a.Language, &a.Content, &a.CreatedAt); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}
