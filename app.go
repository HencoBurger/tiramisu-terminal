package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Profile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	HomeDir string `json:"homeDir"`
}

type TabConfig struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	WorkDir       string `json:"workDir"`
	SessionID     string `json:"sessionId"`
	SoundOverride string `json:"soundOverride"`
	ProfileID     string `json:"profileId"`
	Model         string `json:"model"`
	Provider      string `json:"provider,omitempty"`
	WorkerModel   string `json:"workerModel,omitempty"`
	Type          string `json:"type"`
	// IDE tabs: the open file paths and which one was active, for restore.
	OpenFiles  []string `json:"openFiles,omitempty"`
	ActiveFile string   `json:"activeFile,omitempty"`
}

// AppConfig is kept for backward compatibility during migration.
type AppConfig struct {
	DefaultSound   string      `json:"defaultSound"`
	Theme          string      `json:"theme"`
	PermissionMode string      `json:"permissionMode"`
	ProjectName    string      `json:"projectName"`
	Tabs           []TabConfig `json:"tabs"`
	Profiles       []Profile   `json:"profiles"`
}

// GlobalConfig holds shared defaults across all windows.
type GlobalConfig struct {
	DefaultSound   string    `json:"defaultSound"`
	Theme          string    `json:"theme"`
	PermissionMode string    `json:"permissionMode"`
	Profiles       []Profile `json:"profiles"`
	// Provider settings for the native chat runtime. The OpenRouter API key is NOT
	// here — it lives in secrets.json (see secrets.go).
	OllamaBaseURL    string            `json:"ollamaBaseURL"`
	EnabledProviders []string          `json:"enabledProviders"`
	DefaultModels    map[string]string `json:"defaultModels"`
	// Suppress reasoning-model "thinking" (sends reasoning_effort:none to Ollama) —
	// gives direct answers and avoids verbose reasoning overflowing the context.
	DisableThinking bool `json:"disableThinking"`
	// Extra instructions appended to the native agent's system prompt (editable preprompt).
	CustomInstructions string `json:"customInstructions"`
	// Token budget at which the native runtime auto-summarizes older turns (0 = default).
	ContextBudgetTokens int `json:"contextBudgetTokens"`
}

// WindowSession holds per-window state.
type WindowSession struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Tabs             []TabConfig `json:"tabs"`
	DefaultWorkDir   string      `json:"defaultWorkDir,omitempty"`
	ThemeOverride    string      `json:"themeOverride,omitempty"`
	SoundOverride    string      `json:"soundOverride,omitempty"`
	PermModeOverride string      `json:"permModeOverride,omitempty"`
	CreatedAt        int64       `json:"createdAt"`
	LastOpenedAt     int64       `json:"lastOpenedAt"`
}

// WindowSessionSummary is returned by ListWindowSessions.
type WindowSessionSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TabCount     int    `json:"tabCount"`
	LastOpenedAt int64  `json:"lastOpenedAt"`
}

// EffectiveConfig merges global defaults with session overrides.
type EffectiveConfig struct {
	Theme          string    `json:"theme"`
	DefaultSound   string    `json:"defaultSound"`
	PermissionMode string    `json:"permissionMode"`
	Profiles       []Profile `json:"profiles"`
}

type App struct {
	ctx              context.Context
	sessions         map[string]*ClaudeSession
	sessMu           sync.RWMutex
	agentSessions    map[string]*AgentSession
	agentMu          sync.RWMutex
	pendingPerms     map[string]chan bool
	permMu           sync.Mutex
	terminals        map[string]*PTYSession
	termMu           sync.RWMutex
	globalConfig     GlobalConfig
	globalConfigPath string
	windowSession    *WindowSession
	closing          bool
	closingMu        sync.RWMutex
}

func NewApp() *App {
	return &App{
		sessions:      make(map[string]*ClaudeSession),
		agentSessions: make(map[string]*AgentSession),
		pendingPerms:  make(map[string]chan bool),
		terminals:     make(map[string]*PTYSession),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadGlobalConfig()
	a.migrateOldConfig()
	a.cleanupOrphanScrollback()
}

// cleanupOrphanScrollback removes persisted terminal scrollback files for tabs
// that are no longer referenced by any saved window session. This reclaims disk
// for tabs the user permanently closed, while keeping files for tabs that simply
// survived an app quit.
func (a *App) cleanupOrphanScrollback() {
	scrollDir, err := a.scrollbackDir()
	if err != nil {
		return
	}
	scrollEntries, err := os.ReadDir(scrollDir)
	if err != nil {
		return
	}

	// Collect every tab ID referenced across all session files.
	referenced := make(map[string]bool)
	if sessDir, err := a.sessionsDir(); err == nil {
		if sessEntries, err := os.ReadDir(sessDir); err == nil {
			for _, entry := range sessEntries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(sessDir, entry.Name()))
				if err != nil {
					continue
				}
				var ws WindowSession
				if err := json.Unmarshal(data, &ws); err != nil {
					continue
				}
				for _, tab := range ws.Tabs {
					referenced[tab.ID] = true
				}
			}
		}
	}

	for _, entry := range scrollEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".ansi" {
			continue
		}
		tabID := strings.TrimSuffix(entry.Name(), ".ansi")
		if !referenced[tabID] {
			_ = os.Remove(filepath.Join(scrollDir, entry.Name()))
		}
	}
}

func (a *App) safeEmit(eventName string, data ...interface{}) {
	a.closingMu.RLock()
	defer a.closingMu.RUnlock()
	if a.closing {
		return
	}
	runtime.EventsEmit(a.ctx, eventName, data...)
}

func (a *App) shutdown(ctx context.Context) {
	a.closingMu.Lock()
	a.closing = true
	a.closingMu.Unlock()

	a.sessMu.Lock()
	sessions := make([]*ClaudeSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.sessions = make(map[string]*ClaudeSession)
	a.sessMu.Unlock()

	for _, s := range sessions {
		a.stopSession(s)
	}

	a.agentMu.Lock()
	agents := make([]*AgentSession, 0, len(a.agentSessions))
	for _, s := range a.agentSessions {
		agents = append(agents, s)
	}
	a.agentSessions = make(map[string]*AgentSession)
	a.agentMu.Unlock()

	for _, s := range agents {
		a.stopAgent(s)
	}

	a.termMu.Lock()
	terminals := make([]*PTYSession, 0, len(a.terminals))
	for _, t := range a.terminals {
		terminals = append(terminals, t)
	}
	a.terminals = make(map[string]*PTYSession)
	a.termMu.Unlock()

	for _, t := range terminals {
		if err := a.closePTY(t); err != nil {
			log.Printf("error closing terminal %s: %v", t.id, err)
		}
	}

	if a.windowSession != nil {
		_ = a.saveWindowSessionFile(a.windowSession)
	}
	_ = a.saveGlobalConfigFile()
}

func (a *App) configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tiramisu")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) sessionsDir() (string, error) {
	cfgDir, err := a.configDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) loadGlobalConfig() {
	dir, err := a.configDir()
	if err != nil {
		return
	}
	a.globalConfigPath = filepath.Join(dir, "config.json")
	data, err := os.ReadFile(a.globalConfigPath)
	if err != nil {
		a.globalConfig = GlobalConfig{
			DefaultSound:  "ding",
			Theme:         "dark",
			OllamaBaseURL: defaultOllamaBaseURL,
		}
		return
	}
	_ = json.Unmarshal(data, &a.globalConfig)
	if a.globalConfig.DefaultSound == "" {
		a.globalConfig.DefaultSound = "ding"
	}
	if a.globalConfig.Theme == "" {
		a.globalConfig.Theme = "dark"
	}
	if a.globalConfig.OllamaBaseURL == "" {
		a.globalConfig.OllamaBaseURL = defaultOllamaBaseURL
	}
}

func (a *App) saveGlobalConfigFile() error {
	if a.globalConfigPath == "" {
		dir, err := a.configDir()
		if err != nil {
			return err
		}
		a.globalConfigPath = filepath.Join(dir, "config.json")
	}
	data, err := json.MarshalIndent(a.globalConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.globalConfigPath, data, 0644)
}

// migrateOldConfig migrates old AppConfig (with tabs/projectName) to the new format.
func (a *App) migrateOldConfig() {
	if a.globalConfigPath == "" {
		return
	}
	data, err := os.ReadFile(a.globalConfigPath)
	if err != nil {
		return
	}
	var old AppConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return
	}
	if len(old.Tabs) == 0 {
		return
	}

	// Create a session from the old tabs
	now := time.Now().Unix()
	name := old.ProjectName
	if name == "" {
		name = "Migrated Session"
	}
	ws := &WindowSession{
		ID:           uuid.New().String(),
		Name:         name,
		Tabs:         old.Tabs,
		CreatedAt:    now,
		LastOpenedAt: now,
	}
	if err := a.saveWindowSessionFile(ws); err != nil {
		log.Printf("migration: failed to save session: %v", err)
		return
	}

	// Copy global fields from old config
	a.globalConfig = GlobalConfig{
		DefaultSound:   old.DefaultSound,
		Theme:          old.Theme,
		PermissionMode: old.PermissionMode,
		Profiles:       old.Profiles,
	}
	if a.globalConfig.DefaultSound == "" {
		a.globalConfig.DefaultSound = "ding"
	}
	if a.globalConfig.Theme == "" {
		a.globalConfig.Theme = "dark"
	}
	_ = a.saveGlobalConfigFile()
	log.Printf("migration: migrated %d tabs to session %q (%s)", len(old.Tabs), name, ws.ID)
}

func (a *App) saveWindowSessionFile(ws *WindowSession) error {
	dir, err := a.sessionsDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ws.ID+".json"), data, 0644)
}

// --- Exported methods (auto-bound by Wails) ---

func (a *App) GetGlobalConfig() GlobalConfig {
	return a.globalConfig
}

func (a *App) SaveGlobalConfig(config GlobalConfig) error {
	a.globalConfig = config
	return a.saveGlobalConfigFile()
}

func (a *App) ListWindowSessions() ([]WindowSessionSummary, error) {
	dir, err := a.sessionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var summaries []WindowSessionSummary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var ws WindowSession
		if err := json.Unmarshal(data, &ws); err != nil {
			continue
		}
		summaries = append(summaries, WindowSessionSummary{
			ID:           ws.ID,
			Name:         ws.Name,
			TabCount:     len(ws.Tabs),
			LastOpenedAt: ws.LastOpenedAt,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastOpenedAt > summaries[j].LastOpenedAt
	})

	return summaries, nil
}

func (a *App) CreateWindowSession(name string) (WindowSession, error) {
	now := time.Now().Unix()
	ws := WindowSession{
		ID:           uuid.New().String(),
		Name:         name,
		Tabs:         []TabConfig{},
		CreatedAt:    now,
		LastOpenedAt: now,
	}
	if err := a.saveWindowSessionFile(&ws); err != nil {
		return WindowSession{}, err
	}
	a.windowSession = &ws
	runtime.WindowSetTitle(a.ctx, name)
	return ws, nil
}

func (a *App) LoadWindowSession(id string) (WindowSession, error) {
	dir, err := a.sessionsDir()
	if err != nil {
		return WindowSession{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return WindowSession{}, err
	}
	var ws WindowSession
	if err := json.Unmarshal(data, &ws); err != nil {
		return WindowSession{}, err
	}
	ws.LastOpenedAt = time.Now().Unix()
	if err := a.saveWindowSessionFile(&ws); err != nil {
		return WindowSession{}, err
	}
	a.windowSession = &ws
	runtime.WindowSetTitle(a.ctx, ws.Name)
	return ws, nil
}

func (a *App) SaveWindowSession(session WindowSession) error {
	a.windowSession = &session
	return a.saveWindowSessionFile(&session)
}

func (a *App) DeleteWindowSession(id string) error {
	dir, err := a.sessionsDir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, id+".json"))
}

func (a *App) GetEffectiveConfig() EffectiveConfig {
	ec := EffectiveConfig{
		Theme:          a.globalConfig.Theme,
		DefaultSound:   a.globalConfig.DefaultSound,
		PermissionMode: a.globalConfig.PermissionMode,
		Profiles:       a.globalConfig.Profiles,
	}
	if a.windowSession != nil {
		if a.windowSession.ThemeOverride != "" {
			ec.Theme = a.windowSession.ThemeOverride
		}
		if a.windowSession.SoundOverride != "" {
			ec.DefaultSound = a.windowSession.SoundOverride
		}
		if a.windowSession.PermModeOverride != "" {
			ec.PermissionMode = a.windowSession.PermModeOverride
		}
	}
	return ec
}

func (a *App) effectivePermissionMode() string {
	if a.windowSession != nil && a.windowSession.PermModeOverride != "" {
		return a.windowSession.PermModeOverride
	}
	return a.globalConfig.PermissionMode
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Working Directory",
	})
}

func (a *App) SetWindowTitle(title string) {
	runtime.WindowSetTitle(a.ctx, title)
}

func (a *App) GetHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (a *App) NewWindow() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func (a *App) Log(msg string) {
	log.Println(msg)
}
