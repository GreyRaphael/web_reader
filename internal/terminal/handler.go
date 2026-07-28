package terminal

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/net/websocket"
)

// resizeMessage is sent from the client to update the PTY size.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Handler returns an http.Handler that upgrades to a WebSocket and bridges
// it to a PTY running $SHELL in the given workspace directory.
func Handler(workspace string) http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
		if _, err := exec.LookPath(shell); err != nil {
			shell = "/bin/sh"
		}

		cmd := exec.Command(shell)
		cmd.Dir = workspace
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")

		pty, err := StartPTY(cmd)
		if err != nil {
			slog.Error("terminal: start pty", "error", err)
			_, _ = ws.Write([]byte("Terminal unavailable: " + err.Error() + "\r\n"))
			return
		}
		defer pty.Master.Close()

		ctx, cancel := context.WithCancel(ws.Request().Context())
		defer cancel()

		// pty -> websocket
		go func() {
			buf := make([]byte, 4096)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				n, err := pty.Master.Read(buf)
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
				_ = pty.Kill()
				break
			}
			if n == 0 {
				continue
			}

			// Try to parse as a JSON control message first.
			if isControlMessage(readBuf[:n]) {
				var msg resizeMessage
				if json.Unmarshal(readBuf[:n], &msg) == nil && msg.Type == "resize" {
					_ = pty.SetSize(msg.Cols, msg.Rows)
					continue
				}
			}

			// Otherwise treat as raw terminal input.
			if _, err := pty.Master.Write(readBuf[:n]); err != nil {
				break
			}
		}

		_ = cmd.Wait()
	})
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
