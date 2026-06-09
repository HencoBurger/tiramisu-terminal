package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aymanbagabas/go-pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	err := session.pty.Close()
	<-session.done
	return err
}

// readTerminalOutput reads PTY output and emits events to the frontend.
func (a *App) readTerminalOutput(session *PTYSession) {
	defer close(session.done)

	buf := make([]byte, 4096)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			runtime.EventsEmit(a.ctx, "terminal:output", session.id, encoded)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("terminal read error (session %s): %v", session.id, err)
			}
			return
		}
	}
}
