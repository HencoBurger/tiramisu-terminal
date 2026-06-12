package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aymanbagabas/go-pty"
)

// maxScrollbackBytes caps how much per-tab terminal scrollback we persist.
const maxScrollbackBytes = 256 * 1024

// PTYSession holds a single terminal session.
type PTYSession struct {
	id   string
	pty  pty.Pty
	cmd  *pty.Cmd
	done chan struct{}
}

// TerminalStart creates a new PTY session running the user's shell.
func (a *App) TerminalStart(sessionID string, cols, rows int, workDir string) error {
	a.termMu.Lock()
	defer a.termMu.Unlock()

	if _, exists := a.terminals[sessionID]; exists {
		return fmt.Errorf("terminal session %s already exists", sessionID)
	}

	p, err := pty.New()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	if err := p.Resize(cols, rows); err != nil {
		p.Close()
		return fmt.Errorf("failed to resize pty: %w", err)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := p.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if workDir != "" {
		cmd.Dir = workDir
	}

	if err := cmd.Start(); err != nil {
		p.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	session := &PTYSession{
		id:   sessionID,
		pty:  p,
		cmd:  cmd,
		done: make(chan struct{}),
	}
	a.terminals[sessionID] = session

	go a.readTerminalOutput(session)

	return nil
}

// TerminalInput writes user input to the PTY.
func (a *App) TerminalInput(sessionID string, base64Data string) error {
	a.termMu.RLock()
	session, ok := a.terminals[sessionID]
	a.termMu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal session %s not found", sessionID)
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode input: %w", err)
	}

	_, err = session.pty.Write(data)
	return err
}

// TerminalResize resizes the PTY.
func (a *App) TerminalResize(sessionID string, cols, rows int) error {
	a.termMu.RLock()
	session, ok := a.terminals[sessionID]
	a.termMu.RUnlock()

	if !ok {
		return fmt.Errorf("terminal session %s not found", sessionID)
	}

	return session.pty.Resize(cols, rows)
}

// TerminalStop closes and removes a PTY session.
func (a *App) TerminalStop(sessionID string) error {
	a.termMu.Lock()
	session, ok := a.terminals[sessionID]
	if ok {
		delete(a.terminals, sessionID)
	}
	a.termMu.Unlock()

	if !ok {
		return nil
	}

	return a.closePTY(session)
}

func (a *App) closePTY(session *PTYSession) error {
	// Kill the shell process explicitly — pty.Close() only sends SIGHUP
	// which the shell may ignore.
	if session.cmd != nil && session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}

	err := session.pty.Close()

	// Wait for the reader goroutine with a timeout
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		log.Printf("terminal %s: timed out waiting for reader goroutine", session.id)
	}

	// Reap the process to avoid zombies
	if session.cmd != nil {
		_ = session.cmd.Wait()
	}

	return err
}

// scrollbackDir returns ~/.tiramisu/scrollback, creating it if needed.
func (a *App) scrollbackDir() (string, error) {
	cfgDir, err := a.configDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, "scrollback")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// scrollbackPath returns the on-disk path for a tab's scrollback snapshot, or an
// error if the tab ID is unsafe to use as a filename.
func (a *App) scrollbackPath(tabID string) (string, error) {
	if tabID == "" || strings.ContainsAny(tabID, "/\\") || strings.Contains(tabID, "..") {
		return "", fmt.Errorf("invalid tab id %q", tabID)
	}
	dir, err := a.scrollbackDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tabID+".ansi"), nil
}

// TerminalSaveScrollback persists a serialized snapshot of a tab's terminal
// buffer. The newest data is kept; anything beyond maxScrollbackBytes is dropped
// from the front.
func (a *App) TerminalSaveScrollback(tabID string, base64Data string) error {
	path, err := a.scrollbackPath(tabID)
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode scrollback: %w", err)
	}
	if len(data) > maxScrollbackBytes {
		data = data[len(data)-maxScrollbackBytes:]
	}
	return os.WriteFile(path, data, 0644)
}

// TerminalLoadScrollback returns a tab's persisted scrollback snapshot,
// base64-encoded, or "" if none exists.
func (a *App) TerminalLoadScrollback(tabID string) (string, error) {
	path, err := a.scrollbackPath(tabID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// TerminalDeleteScrollback removes a tab's persisted scrollback snapshot.
func (a *App) TerminalDeleteScrollback(tabID string) error {
	path, err := a.scrollbackPath(tabID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readTerminalOutput reads PTY output and emits events to the frontend.
func (a *App) readTerminalOutput(session *PTYSession) {
	defer close(session.done)

	buf := make([]byte, 4096)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			a.safeEmit("terminal:output", session.id, encoded)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("terminal read error (session %s): %v", session.id, err)
			}
			return
		}
	}
}
