package browser

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ds2api/internal/config"
)

const tokenFileName = ".browser_token"

type Manager struct {
	mu         sync.Mutex
	token      string
	tokenFile  string
	tokenReady chan struct{}
}

func NewManager() *Manager {
	exe, _ := os.Executable()
	tokenFile := filepath.Join(filepath.Dir(exe), tokenFileName)
	return &Manager{
		tokenFile:  tokenFile,
		tokenReady: make(chan struct{}),
	}
}

func (m *Manager) Token() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token
}

func (m *Manager) setToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

func (m *Manager) loadSavedToken() string {
	data, err := os.ReadFile(m.tokenFile)
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(data))
	if token != "" {
		config.Logger.Info("[browser] loaded saved token from disk")
	}
	return token
}

func (m *Manager) saveToken(token string) {
	if token == "" {
		return
	}
	if err := os.WriteFile(m.tokenFile, []byte(token), 0o600); err != nil {
		config.Logger.Warn("[browser] failed to save token", "error", err)
	} else {
		config.Logger.Info("[browser] token saved to disk")
	}
}

func (m *Manager) Start(_ context.Context, _ time.Duration) (string, error) {
	if saved := m.loadSavedToken(); saved != "" {
		m.setToken(saved)
		close(m.tokenReady)
		return saved, nil
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  DeepSeek Token Required")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("1. Open https://chat.deepseek.com in your browser")
	fmt.Println("2. Press F12 -> Application -> Local Storage -> chat.deepseek.com")
	fmt.Println("3. Find 'userToke' key, copy its value")
	fmt.Println("   (or open DevTools Console and run: localStorage.getItem('userToke'))")
	fmt.Println()
	fmt.Print("Paste token here: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	token := strings.TrimSpace(line)
	if token == "" {
		return "", fmt.Errorf("no token provided")
	}

	m.setToken(token)
	m.saveToken(token)
	close(m.tokenReady)

	config.Logger.Info("[browser] token accepted")
	return token, nil
}

func (m *Manager) WaitForToken(ctx context.Context) (string, error) {
	if t := m.Token(); t != "" {
		return t, nil
	}
	select {
	case <-m.tokenReady:
		return m.Token(), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (m *Manager) Stop() {}
