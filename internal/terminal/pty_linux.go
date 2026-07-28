//go:build linux

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// PTY holds the master file descriptor and the child process.
type PTY struct {
	Master *os.File
	Cmd    *exec.Cmd
}

// StartPTY creates a pseudo-terminal pair and starts the given command with
// its stdin/stdout/stderr connected to the slave end. The caller owns the
// returned Master file and must close it (and wait the process) when done.
func StartPTY(cmd *exec.Cmd) (*PTY, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open ptmx: %w", err)
	}

	// Unlock the slave PTY.
	var unlock int32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(),
		uintptr(unix.TIOCSPTLCK), uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		master.Close()
		return nil, fmt.Errorf("unlock pty: %v", errno)
	}

	// Get the PTY number.
	var ptn int32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(),
		uintptr(unix.TIOCGPTN), uintptr(unsafe.Pointer(&ptn))); errno != 0 {
		master.Close()
		return nil, fmt.Errorf("get pty number: %v", errno)
	}
	slaveName := fmt.Sprintf("/dev/pts/%d", ptn)

	slave, err := os.OpenFile(slaveName, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("open slave pty: %w", err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr.Ctty = 0 // stdin fd in the child process

	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		return nil, fmt.Errorf("start command: %w", err)
	}
	// Close slave in parent; child retains its copy.
	slave.Close()

	return &PTY{Master: master, Cmd: cmd}, nil
}

// SetSize updates the terminal window size.
func (p *PTY) SetSize(cols, rows uint16) error {
	ws := &unix.Winsize{Row: rows, Col: cols, Xpixel: 0, Ypixel: 0}
	return unix.IoctlSetWinsize(int(p.Master.Fd()), unix.TIOCSWINSZ, ws)
}

// Kill sends SIGKILL to the child process.
func (p *PTY) Kill() error {
	return p.Cmd.Process.Signal(unix.SIGKILL)
}
