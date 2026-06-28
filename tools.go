package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Tool is a function the model can call. Mutating tools go through the permission
// gate unless the permission mode auto-approves them.
type Tool struct {
	Name         string
	Description  string
	Parameters   map[string]interface{} // JSON schema
	Mutating     bool
	Summarizable bool // if true, oversized output is summarized (not truncated) before returning
	Run          func(ctx context.Context, workDir string, args map[string]interface{}) (string, error)
}

var (
	reHTMLDrop     = regexp.MustCompile(`(?is)<(script|style|head|nav|footer|svg|noscript)\b[^>]*>.*?</\s*(script|style|head|nav|footer|svg|noscript)\s*>`)
	reHTMLComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLBlockEnd = regexp.MustCompile(`(?i)</(p|div|li|h[1-6]|tr|section|article|ul|ol|table|blockquote)\s*>`)
	reHTMLBr       = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHTMLTag      = regexp.MustCompile(`(?s)<[^>]+>`)
	reHTMLSpaces   = regexp.MustCompile(`[ \t\f\v]+`)
	reHTMLBlanks   = regexp.MustCompile(`\n[ \t]*\n[ \t]*(?:\n[ \t]*)+`)
)

// htmlToText strips an HTML document to readable text (drops script/style/nav/etc.).
func htmlToText(s string) string {
	s = reHTMLComment.ReplaceAllString(s, " ")
	s = reHTMLDrop.ReplaceAllString(s, " ")
	s = reHTMLBr.ReplaceAllString(s, "\n")
	s = reHTMLBlockEnd.ReplaceAllString(s, "\n")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reHTMLSpaces.ReplaceAllString(s, " ")
	s = reHTMLBlanks.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
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

const defaultMaxToolOutputChars = 8000

// capToolOutput truncates an oversized tool result (e.g. a curl/cat of a large page)
// before it enters the conversation, keeping the head and tail with a marker — so one
// tool call can't blow the context window.
func capToolOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	head := max * 2 / 3
	tail := max - head
	return s[:head] +
		fmt.Sprintf("\n\n…[truncated %d characters — tool output too large for the context window; read a specific file/section or use a narrower command]…\n\n", len(s)-max) +
		s[len(s)-tail:]
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
		{
			Name:        "fetch_url",
			Description: "Fetch a web page or text URL and return its readable text content (HTML is stripped to clean text). Use this instead of curl for reading websites or documentation.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "The http(s) URL to fetch."},
				},
				"required": []string{"url"},
			},
			Summarizable: true,
			Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
				url := strings.TrimSpace(argString(args, "url"))
				if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					return "", fmt.Errorf("url must start with http:// or https://")
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					return "", err
				}
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Tiramisu/1.0)")
				req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,*/*")
				resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
				if err != nil {
					return "", err
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
				if err != nil {
					return "", err
				}
				text := string(body)
				ct := strings.ToLower(resp.Header.Get("Content-Type"))
				sniff := text
				if len(sniff) > 256 {
					sniff = sniff[:256]
				}
				if strings.Contains(ct, "html") || strings.Contains(strings.ToLower(sniff), "<html") || strings.Contains(strings.ToLower(sniff), "<!doctype html") {
					text = htmlToText(text)
				}
				if strings.TrimSpace(text) == "" {
					return "(empty page)", nil
				}
				if resp.StatusCode >= 400 {
					return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, text), nil
				}
				return text, nil
			},
		},
	}
}
