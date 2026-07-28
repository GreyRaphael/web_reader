package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	goptyp "github.com/aymanbagabas/go-pty"
	"golang.org/x/net/websocket"
)

// resizeMessage is sent from the client to update the PTY size.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Handler returns an http.Handler that upgrades to a WebSocket and bridges
// it to a PTY running a shell in the given workspace directory.
func Handler(workspace string) http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		ctx, cancel := context.WithCancel(ws.Request().Context())
		defer cancel()

		shell, err := defaultShell()
		if err != nil {
			slog.Error("terminal: resolve shell", "error", err)
			_, _ = ws.Write([]byte("Terminal unavailable: " + err.Error() + "\r\n"))
			return
		}

		p, err := goptyp.New()
		if err != nil {
			slog.Error("terminal: open pty", "error", err)
			_, _ = ws.Write([]byte("Terminal unavailable: " + err.Error() + "\r\n"))
			return
		}
		defer p.Close()

		c := p.CommandContext(ctx, shell)
		c.Dir = workspace
		c.Env = append(os.Environ(), "TERM=xterm-256color")

		if err := c.Start(); err != nil {
			slog.Error("terminal: start command", "error", err)
			_, _ = ws.Write([]byte("Terminal unavailable: " + err.Error() + "\r\n"))
			return
		}

		// pty -> websocket
		go func() {
			buf := make([]byte, 4096)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				n, err := p.Read(buf)
				if n > 0 {
					if _, werr := ws.Write(buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					if err != io.EOF {
						slog.Debug("terminal: pty read ended", "error", err)
					}
					return
				}
			}
		}()

		// websocket -> pty (also handles resize control messages)
		readBuf := make([]byte, 4096)
		for {
			n, err := ws.Read(readBuf)
			if err != nil {
				cancel()
				break
			}
			if n == 0 {
				continue
			}

			// Try to parse as a JSON control message first.
			if isControlMessage(readBuf[:n]) {
				var msg resizeMessage
				if json.Unmarshal(readBuf[:n], &msg) == nil && msg.Type == "resize" {
					_ = p.Resize(int(msg.Cols), int(msg.Rows))
					continue
				}
			}

			// Otherwise treat as raw terminal input.
			if _, err := p.Write(readBuf[:n]); err != nil {
				break
			}
		}

		_ = c.Wait()
	})
}

// defaultShell returns a usable shell program path for the current platform.
func defaultShell() (string, error) {
	candidates := []string{os.Getenv("SHELL"), "/bin/bash", "/bin/sh"}
	if runtime.GOOS == "windows" {
		candidates = []string{os.Getenv("COMSPEC"), "powershell.exe", "cmd.exe"}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no usable shell found")
}

// isControlMessage returns true if the data looks like a JSON object starting
// with '{' and containing a "type" field. Regular terminal input never starts
// with '{' in a way that would match this heuristic reliably, but we do a
// stricter check: only if it parses as JSON with a type field.
func isControlMessage(data []byte) bool {
	if len(data) == 0 || data[0] != '{' {
		return false
	}
	var probe map[string]any
	return json.Unmarshal(data, &probe) == nil && probe["type"] != nil
}
