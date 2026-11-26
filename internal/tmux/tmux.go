// Package tmux drives the tmux CLI to build the multi-pane layout used by
// `ssm session`.
//
// Sessions, windows and panes are always targeted by the unique IDs tmux
// assigns them ($N, @N, %N) rather than by index, so the commands are not
// affected by the user's base-index / pane-base-index settings or by being run
// from inside another tmux session.
package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// run executes tmux with the given arguments and returns its trimmed stdout.
// It is a variable so tests can replace it.
var run = func(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("tmux %s: %w: %s", args[0], err, msg)
		}
		return "", fmt.Errorf("tmux %s: %w", args[0], err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Session is a tmux session containing a single window.
type Session struct {
	ID       string // tmux session ID, e.g. "$3"
	Name     string // session name as tmux recorded it
	WindowID string // ID of the session's window, e.g. "@5"
}

// Pane is a single pane within a tmux window.
type Pane struct {
	ID string // tmux pane ID, e.g. "%7"
}

// exact makes a session target match only the session with exactly this name;
// without it tmux also accepts a name prefix (so "ssm-1" would match "ssm-10").
func exact(name string) string {
	return "=" + name
}

// HasSession reports whether a session with exactly this name exists.
func HasSession(name string) bool {
	_, err := run("has-session", "-t", exact(name))
	return err == nil
}

// KillSession kills the session with exactly this name, if there is one.
func KillSession(name string) error {
	if !HasSession(name) {
		return nil
	}

	_, err := run("kill-session", "-t", exact(name))
	return err
}

// NewSession creates a detached session with one window, replacing any
// existing session with the same name. It returns the session and the
// window's initial pane.
func NewSession(name, windowName string) (*Session, Pane, error) {
	if err := KillSession(name); err != nil {
		return nil, Pane{}, err
	}

	// The session name goes last because it is the only field that can
	// contain the separator.
	out, err := run("new-session", "-d", "-s", name, "-n", windowName,
		"-P", "-F", "#{session_id} #{window_id} #{pane_id} #{session_name}")
	if err != nil {
		return nil, Pane{}, err
	}

	fields := strings.SplitN(out, " ", 4)
	if len(fields) != 4 {
		return nil, Pane{}, fmt.Errorf("tmux new-session: unexpected output %q", out)
	}

	s := &Session{ID: fields[0], WindowID: fields[1], Name: fields[3]}
	return s, Pane{ID: fields[2]}, nil
}

// SetOption sets a tmux option on the session (or, for a window option, on
// its window).
func (s *Session) SetOption(option, value string) error {
	_, err := run("set-option", "-t", s.ID, option, value)
	return err
}

// Tile arranges the panes of the session's window in a tiled layout.
func (s *Session) Tile() error {
	_, err := run("select-layout", "-t", s.WindowID, "tiled")
	return err
}

// Attach attaches the current terminal to the session and blocks until the
// client detaches or the session ends.
func (s *Session) Attach() error {
	cmd := exec.Command("tmux", "attach-session", "-t", s.ID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Split splits the pane vertically and returns the new pane, which is placed
// below it.
func (p Pane) Split() (Pane, error) {
	out, err := run("split-window", "-v", "-t", p.ID, "-P", "-F", "#{pane_id}")
	if err != nil {
		return Pane{}, err
	}

	if out == "" {
		return Pane{}, fmt.Errorf("tmux split-window: no pane ID returned")
	}

	return Pane{ID: out}, nil
}

// SetTitle sets the pane's title, shown in the pane border.
func (p Pane) SetTitle(title string) error {
	_, err := run("select-pane", "-t", p.ID, "-T", title)
	return err
}

// SendLine types line into the pane exactly as given, then presses Enter.
func (p Pane) SendLine(line string) error {
	if _, err := run("send-keys", "-t", p.ID, "-l", line); err != nil {
		return err
	}

	_, err := run("send-keys", "-t", p.ID, "Enter")
	return err
}

// Kill closes the pane.
func (p Pane) Kill() error {
	_, err := run("kill-pane", "-t", p.ID)
	return err
}
