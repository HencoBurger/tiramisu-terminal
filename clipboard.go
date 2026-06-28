package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"time"
)

// GetClipboardImage returns an image currently on the system clipboard as a data URL,
// or an error if there is none. The WebKitGTK webview doesn't reliably expose pasted
// images to JS, so we read the clipboard via xclip (X11) / wl-paste (Wayland).
func (a *App) GetClipboardImage() (string, error) {
	type attempt struct {
		bin  string
		args []string
		mime string
	}
	attempts := []attempt{
		{"xclip", []string{"-selection", "clipboard", "-t", "image/png", "-o"}, "image/png"},
		{"wl-paste", []string{"--type", "image/png"}, "image/png"},
		{"xclip", []string{"-selection", "clipboard", "-t", "image/jpeg", "-o"}, "image/jpeg"},
		{"wl-paste", []string{"--type", "image/jpeg"}, "image/jpeg"},
	}

	for _, at := range attempts {
		if _, err := exec.LookPath(at.bin); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := exec.CommandContext(ctx, at.bin, at.args...).Output()
		cancel()
		if err != nil || len(out) == 0 {
			continue
		}
		return fmt.Sprintf("data:%s;base64,%s", at.mime, base64.StdEncoding.EncodeToString(out)), nil
	}
	return "", fmt.Errorf("no image on clipboard")
}
