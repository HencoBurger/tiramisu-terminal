package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Tool is a function the model can call. Mutating tools go through the permission
// gate unless the permission mode auto-approves them.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON schema
	Mutating    bool
	Run         func(ctx context.Context, workDir string, args map[string]interface{}) (string, error)
}

func resolvePath(workDir, p string) string {
	if p == "" {
		return workDir
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(workDir, p)
}

func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

var bashRedirectRe = regexp.MustCompile(`(>>?)\s*("?)([^\s"';|&<>]+)`)

// bashOverwriteTargets returns resolved paths a command appears to TRUNCATE-overwrite
// via `>` redirection and that exist as regular files. Best-effort: append (>>) is
// allowed (doesn't clobber); /dev/* and fd dups (>&) are skipped; other write
// mechanisms (sed -i, tee, cp/mv, scripts) aren't detected here and rely on the
// permission gate. A `>` inside a quoted string can cause a spurious match — that
// only ever errs toward requiring a read, never toward an unguarded overwrite.
func bashOverwriteTargets(cmd, workDir string) []string {
	var out []string
	for _, m := range bashRedirectRe.FindAllStringSubmatch(cmd, -1) {
		op, tok := m[1], m[3]
		if op != ">" {
			continue // append (>>) doesn't overwrite existing content
		}
		if tok == "" || strings.HasPrefix(tok, "&") || strings.HasPrefix(tok, "/dev/") {
			continue
		}
		p := resolvePath(workDir, tok)
		if isRegularFile(p) {
			out = append(out, p)
		}
	}
	return out
}

func findTool(tools []Tool, name string) *Tool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

// defaultTools returns the M1 tool set. M2 appends the delegate tool.
func defaultTools() []Tool {
	return []Tool{
		{
			Name:        "read_file",
			Description: "Read a file and return its contents.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "File path, absolute or relative to the working directory."},
				},
				"required": []string{"path"},
			},
			Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
				data, err := os.ReadFile(resolvePath(workDir, argString(args, "path")))
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
		{
			Name:        "list_directory",
			Description: "List the entries in a directory (directories first, with a trailing slash).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Directory path, absolute or relative to the working directory."},
				},
				"required": []string{"path"},
			},
			Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
				entries, err := os.ReadDir(resolvePath(workDir, argString(args, "path")))
				if err != nil {
					return "", err
				}
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					if e.IsDir() {
						names = append(names, e.Name()+"/")
					} else {
						names = append(names, e.Name())
					}
				}
				sort.Strings(names)
				if len(names) == 0 {
					return "(empty)", nil
				}
				return strings.Join(names, "\n"), nil
			},
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a single file with the given content. The path must be a file (including its filename), never a directory.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to a single file INCLUDING its filename (e.g. src/app.go), not a directory. Absolute or relative to the working directory."},
					"content": map[string]interface{}{"type": "string", "description": "Full file content to write."},
				},
				"required": []string{"path", "content"},
			},
			Mutating: true,
			Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
				path := resolvePath(workDir, argString(args, "path"))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return "", err
				}
				if err := os.WriteFile(path, []byte(argString(args, "content")), 0644); err != nil {
					return "", err
				}
				return "Wrote " + path, nil
			},
		},
		{
			Name:        "bash",
			Description: "Run a bash command in the working directory. Returns combined stdout+stderr.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "The bash command to run."},
				},
				"required": []string{"command"},
			},
			Mutating: true,
			Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
				cmd := exec.CommandContext(ctx, "bash", "-lc", argString(args, "command"))
				cmd.Dir = workDir
				out, err := cmd.CombinedOutput()
				s := string(out)
				if err != nil {
					return s, fmt.Errorf("command failed: %v", err)
				}
				if strings.TrimSpace(s) == "" {
					return "(no output)", nil
				}
				return s, nil
			},
		},
	}
}
