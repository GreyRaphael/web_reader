//go:build !linux

package terminal

import (
	"errors"
	"os/exec"
)

type PTY struct{}

func StartPTY(cmd *exec.Cmd) (*PTY, error) {
	return nil, errors.New("terminal not supported on this platform")
}

func (p *PTY) SetSize(cols, rows uint16) error { return nil }

func (p *PTY) Kill() error { return nil }
