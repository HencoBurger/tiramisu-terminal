package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Notification sounds are played from the Go side via a system audio player.
// WebKitGTK refuses to play media served over the Wails asset scheme
// (NotSupportedError), so HTML5 <audio> is unreliable here.

//go:embed frontend/public/sounds/*.wav
var soundFS embed.FS

type soundPlayer struct {
	bin  string
	args []string // args that precede the file path
}

// Tried in order; first one found on PATH wins. All take the file path last.
var soundPlayerCandidates = []soundPlayer{
	{"pw-play", nil},               // PipeWire (Ubuntu 24.04 default)
	{"paplay", nil},                // PulseAudio
	{"aplay", []string{"-q"}},      // ALSA
	{"canberra-gtk-play", []string{"-f"}},
}

var (
	soundOnce   sync.Once
	soundDir    string
	soundPlayerFound *soundPlayer
	soundSetupErr error
)

func soundSetup() {
	dir, err := os.MkdirTemp("", "tiramisu-sounds")
	if err != nil {
		soundSetupErr = err
		return
	}
	soundDir = dir
	for i := range soundPlayerCandidates {
		if _, err := exec.LookPath(soundPlayerCandidates[i].bin); err == nil {
			soundPlayerFound = &soundPlayerCandidates[i]
			return
		}
	}
	soundSetupErr = fmt.Errorf("no audio player found (install pipewire-utils or alsa-utils)")
}

// PlaySound plays a bundled notification sound (ding/chime/pop). Non-blocking;
// returns an error the frontend can surface.
func (a *App) PlaySound(name string) error {
	switch name {
	case "ding", "chime", "pop":
	default:
		return fmt.Errorf("unknown sound %q", name)
	}

	soundOnce.Do(soundSetup)
	if soundSetupErr != nil {
		return soundSetupErr
	}

	// Materialize the embedded wav to a temp file once.
	path := filepath.Join(soundDir, name+".wav")
	if _, err := os.Stat(path); err != nil {
		data, err := soundFS.ReadFile("frontend/public/sounds/" + name + ".wav")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}

	p := soundPlayerFound
	cmd := exec.Command(p.bin, append(append([]string{}, p.args...), path)...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", p.bin, err)
	}
	go cmd.Wait() // reap when playback finishes
	return nil
}
