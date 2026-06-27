package main

import "os/exec"

// Notify shows a desktop notification via notify-send (libnotify). Non-blocking;
// silently does nothing if notify-send isn't installed.
func (a *App) Notify(title, body string) error {
	bin, err := exec.LookPath("notify-send")
	if err != nil {
		return nil
	}
	cmd := exec.Command(bin, "-a", "Tiramisu", title, body)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap
	return nil
}
