package browser

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"ds2api/internal/config"
)

type Manager struct {
	mu         sync.Mutex
	token      string
	tokenReady chan struct{}
}

func NewManager() *Manager {
	return &Manager{
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

func (m *Manager) Start(_ context.Context, _ time.Duration) (string, error) {
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
