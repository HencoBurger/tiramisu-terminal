package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

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
	Type          string `json:"type"`
}

type AppConfig struct {
	DefaultSound   string      `json:"defaultSound"`
	Theme          string      `json:"theme"`
	PermissionMode string      `json:"permissionMode"` // "default", "acceptEdits", "bypassPermissions"
	ProjectName    string      `json:"projectName"`
	Tabs           []TabConfig `json:"tabs"`
	Profiles       []Profile   `json:"profiles"`
}

type App struct {
	ctx        context.Context
	sessions   map[string]*ClaudeSession
	sessMu     sync.RWMutex
	terminals  map[string]*PTYSession
	termMu     sync.RWMutex
	config     AppConfig
	configPath string
}

func NewApp() *App {
	return &App{
		sessions:  make(map[string]*ClaudeSession),
		terminals: make(map[string]*PTYSession),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
}

func (a *App) shutdown(ctx context.Context) {
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

	a.saveConfig()
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

func (a *App) loadConfig() {
	dir, err := a.configDir()
	if err != nil {
		return
	}
	a.configPath = filepath.Join(dir, "config.json")
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		a.config = AppConfig{
			DefaultSound: "ding",
			Theme:        "dark",
		}
		return
	}
	_ = json.Unmarshal(data, &a.config)
}

func (a *App) saveConfig() error {
	if a.configPath == "" {
		dir, err := a.configDir()
		if err != nil {
			return err
		}
		a.configPath = filepath.Join(dir, "config.json")
	}
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

func (a *App) GetConfig() AppConfig {
	return a.config
}

func (a *App) SaveConfig(config AppConfig) error {
	a.config = config
	return a.saveConfig()
}

func (a *App) SaveTabConfigs(tabs []TabConfig) error {
	a.config.Tabs = tabs
	return a.saveConfig()
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Working Directory",
	})
}

func (a *App) SetWindowTitle(title string) {
	runtime.WindowSetTitle(a.ctx, title)
}

func (a *App) GetProjectName() string {
	if a.config.ProjectName == "" {
		return "Tiramisu"
	}
	return a.config.ProjectName
}

func (a *App) SetProjectName(name string) error {
	a.config.ProjectName = name
	if name == "" {
		name = "Tiramisu"
	}
	runtime.WindowSetTitle(a.ctx, name)
	return a.saveConfig()
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
