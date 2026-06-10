package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type ClaudeSession struct {
	TabID     string
	SessionID string
	WorkDir   string
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	done      chan struct{}
	mu        sync.Mutex
}

func (a *App) SessionStart(tabID, workDir, prompt, profileID, model string) error {
	a.sessMu.Lock()
	existing, exists := a.sessions[tabID]
	a.sessMu.Unlock()

	if exists {
		a.stopSession(existing)
	}

	return a.spawnClaude(tabID, workDir, "", prompt, profileID, model)
}

func (a *App) SessionSend(tabID, message, profileID, model string) error {
	a.sessMu.RLock()
	session, ok := a.sessions[tabID]
	a.sessMu.RUnlock()

	if ok {
		session.mu.Lock()
		sessionID := session.SessionID
		workDir := session.WorkDir
		session.mu.Unlock()

		if sessionID == "" {
			return fmt.Errorf("session %s has no Claude session ID yet", tabID)
		}

		// Stop the old process (it should already be done, but just in case)
		a.stopSession(session)

		return a.spawnClaude(tabID, workDir, sessionID, message, profileID, model)
	}

	// Session not in Go's map — frontend must provide sessionId and workDir
	return fmt.Errorf("session %s not found", tabID)
}

// SessionResume spawns a new Claude process resuming an existing session.
// Used when the app restarts and the Go session map is empty but the
// frontend still has the sessionId from config.
func (a *App) SessionResume(tabID, workDir, sessionID, message, profileID, model string) error {
	return a.spawnClaude(tabID, workDir, sessionID, message, profileID, model)
}

func (a *App) RunGitDiff(workDir string) (string, error) {
	cmd := exec.Command("git", "diff")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return string(output), nil
		}
		return "", fmt.Errorf("git diff failed: %s", string(output))
	}
	return string(output), nil
}

func (a *App) spawnClaude(tabID, workDir, sessionID, prompt, profileID, model string) error {
	ctx, cancel := context.WithCancel(context.Background())

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
	}

	// Apply permission mode from effective config
	permMode := a.effectivePermissionMode()
	if permMode == "bypassPermissions" {
		args = append(args, "--dangerously-skip-permissions")
	} else if permMode == "acceptEdits" {
		args = append(args, "--permission-mode", "acceptEdits")
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	args = append(args, prompt)

	log.Printf("[claude spawn %s] args: %v", tabID, args)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = workDir

	// Apply profile env vars if a profile is set
	if profileID != "" {
		profile := a.findProfile(profileID)
		if profile != nil && profile.HomeDir != "" {
			cmd.Env = buildProfileEnv(profile.HomeDir)
			log.Printf("[claude spawn %s] using profile %q (HOME=%s)", tabID, profile.Name, profile.HomeDir)
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start claude: %w", err)
	}

	session := &ClaudeSession{
		TabID:     tabID,
		SessionID: sessionID,
		WorkDir:   workDir,
		cmd:       cmd,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	a.sessMu.Lock()
	a.sessions[tabID] = session
	a.sessMu.Unlock()

	// Read stderr in background for logging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[claude stderr %s] %s", tabID, scanner.Text())
		}
	}()

	// Read stdout NDJSON and emit events
	go func() {
		defer close(session.done)

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large outputs

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Try to extract session_id from init events
			var generic map[string]interface{}
			if err := json.Unmarshal([]byte(line), &generic); err == nil {
				if sid, ok := generic["session_id"].(string); ok && sid != "" {
					session.mu.Lock()
					session.SessionID = sid
					session.mu.Unlock()
				}
			}

			a.safeEmit("session:event", tabID, line)
		}

		if err := scanner.Err(); err != nil {
			log.Printf("scanner error for session %s: %v", tabID, err)
		}

		// Wait for process to exit
		exitErr := cmd.Wait()
		exitCode := 0
		if exitErr != nil {
			if exitError, ok := exitErr.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}

		a.safeEmit("session:done", tabID, exitCode)
	}()

	return nil
}

func (a *App) SessionStop(tabID string) error {
	a.sessMu.Lock()
	session, ok := a.sessions[tabID]
	if ok {
		delete(a.sessions, tabID)
	}
	a.sessMu.Unlock()

	if !ok {
		return nil
	}

	a.stopSession(session)
	return nil
}

func (a *App) stopSession(session *ClaudeSession) {
	// Cancel the context — sends SIGKILL via exec.CommandContext
	session.cancel()

	// If the process is still alive, kill the entire process group
	if session.cmd != nil && session.cmd.Process != nil {
		_ = syscall.Kill(-session.cmd.Process.Pid, syscall.SIGKILL)
	}

	// Wait with a timeout so shutdown never blocks forever
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		log.Printf("session %s: timed out waiting for process to exit", session.TabID)
	}

	a.sessMu.Lock()
	if s, ok := a.sessions[session.TabID]; ok && s == session {
		delete(a.sessions, session.TabID)
	}
	a.sessMu.Unlock()
}

func (a *App) GetSessionID(tabID string) string {
	a.sessMu.RLock()
	session, ok := a.sessions[tabID]
	a.sessMu.RUnlock()

	if !ok {
		return ""
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	return session.SessionID
}

// findProfile looks up a profile by ID from the config.
func (a *App) findProfile(id string) *Profile {
	for i := range a.globalConfig.Profiles {
		if a.globalConfig.Profiles[i].ID == id {
			return &a.globalConfig.Profiles[i]
		}
	}
	return nil
}

// buildProfileEnv creates an env slice with HOME overridden to the profile's directory.
// It inherits all current env vars but overrides HOME-related ones.
func buildProfileEnv(homeDir string) []string {
	env := os.Environ()
	result := make([]string, 0, len(env)+3)

	// Filter out HOME-related vars, we'll add our own
	for _, e := range env {
		key := e
		if idx := len(e); idx > 0 {
			for i, c := range e {
				if c == '=' {
					key = e[:i]
					break
				}
			}
		}
		switch key {
		case "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME":
			continue
		default:
			result = append(result, e)
		}
	}

	result = append(result, "HOME="+homeDir)
	return result
}
