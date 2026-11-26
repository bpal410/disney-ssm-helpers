package tmux

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// fakeRun replaces run for the duration of a test. respond returns the output
// and error for each command; every call is recorded.
func fakeRun(t *testing.T, respond func(args []string) (string, error)) *[][]string {
	t.Helper()

	calls := &[][]string{}
	orig := run
	run = func(args ...string) (string, error) {
		*calls = append(*calls, args)
		return respond(args)
	}
	t.Cleanup(func() { run = orig })

	return calls
}

func TestNewSession(t *testing.T) {
	noSession := errors.New("can't find session")

	tests := []struct {
		name      string
		exists    bool
		wantCalls [][]string
	}{
		{
			name:   "no existing session",
			exists: false,
			wantCalls: [][]string{
				{"has-session", "-t", "=ssm"},
				{"new-session", "-d", "-s", "ssm", "-n", "win", "-P", "-F", "#{session_id} #{window_id} #{pane_id} #{session_name}"},
			},
		},
		{
			name:   "replaces existing session",
			exists: true,
			wantCalls: [][]string{
				{"has-session", "-t", "=ssm"},
				{"kill-session", "-t", "=ssm"},
				{"new-session", "-d", "-s", "ssm", "-n", "win", "-P", "-F", "#{session_id} #{window_id} #{pane_id} #{session_name}"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := fakeRun(t, func(args []string) (string, error) {
				switch args[0] {
				case "has-session":
					if !tt.exists {
						return "", noSession
					}
				case "new-session":
					return "$1 @2 %3 ssm", nil
				}
				return "", nil
			})

			s, p, err := NewSession("ssm", "win")
			if err != nil {
				t.Fatalf("NewSession: %v", err)
			}

			if want := (&Session{ID: "$1", Name: "ssm", WindowID: "@2"}); !reflect.DeepEqual(s, want) {
				t.Errorf("session = %+v, want %+v", s, want)
			}
			if p.ID != "%3" {
				t.Errorf("pane ID = %q, want %%3", p.ID)
			}
			if !reflect.DeepEqual(*calls, tt.wantCalls) {
				t.Errorf("calls = %q\nwant  %q", *calls, tt.wantCalls)
			}
		})
	}
}

func TestNewSessionErrors(t *testing.T) {
	t.Run("new-session fails", func(t *testing.T) {
		fakeRun(t, func(args []string) (string, error) {
			if args[0] == "new-session" {
				return "", errors.New("boom")
			}
			return "", errors.New("no session")
		})

		if _, _, err := NewSession("ssm", "win"); err == nil {
			t.Error("expected an error")
		}
	})

	t.Run("unexpected output", func(t *testing.T) {
		fakeRun(t, func(args []string) (string, error) {
			if args[0] == "new-session" {
				return "garbage", nil
			}
			return "", errors.New("no session")
		})

		if _, _, err := NewSession("ssm", "win"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestSessionAndPaneCommands(t *testing.T) {
	calls := fakeRun(t, func(args []string) (string, error) {
		if args[0] == "split-window" {
			return "%9", nil
		}
		return "", nil
	})

	s := &Session{ID: "$1", Name: "ssm", WindowID: "@2"}
	p := Pane{ID: "%3"}

	newPane, err := p.Split()
	if err != nil || newPane.ID != "%9" {
		t.Fatalf("Split() = %+v, %v; want %%9", newPane, err)
	}

	for _, f := range []func() error{
		func() error { return s.SetOption("mouse", "on") },
		s.Tile,
		func() error { return p.SetTitle("i-0123") },
		func() error { return p.SendLine(`echo "Enter" 'C-c'`) },
		p.Kill,
	} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}

	want := [][]string{
		{"split-window", "-v", "-t", "%3", "-P", "-F", "#{pane_id}"},
		{"set-option", "-t", "$1", "mouse", "on"},
		{"select-layout", "-t", "@2", "tiled"},
		{"select-pane", "-t", "%3", "-T", "i-0123"},
		{"send-keys", "-t", "%3", "-l", `echo "Enter" 'C-c'`},
		{"send-keys", "-t", "%3", "Enter"},
		{"kill-pane", "-t", "%3"},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls = %q\nwant  %q", *calls, want)
	}
}

func TestSplitNoPaneID(t *testing.T) {
	fakeRun(t, func(args []string) (string, error) { return "", nil })

	if _, err := (Pane{ID: "%1"}).Split(); err == nil {
		t.Error("expected an error")
	}
}

// TestWithTmux runs against a real tmux server on a private socket. It is
// skipped when tmux is not installed.
func TestWithTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	// Private server: socket under a temp dir, and not nested in any tmux
	// session the tests happen to be run from.
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("TMUX", "")
	os.Unsetenv("TMUX")
	t.Cleanup(func() { run("kill-server") })

	// A session whose name has ours as a prefix must survive.
	if _, _, err := NewSession("ssm-test-10", "other"); err != nil {
		t.Fatalf("NewSession(ssm-test-10): %v", err)
	}

	// Creating the session twice must replace it, not fail or duplicate it.
	if _, _, err := NewSession("ssm-test-1", "ssm"); err != nil {
		t.Fatalf("first NewSession: %v", err)
	}
	s, first, err := NewSession("ssm-test-1", "ssm")
	if err != nil {
		t.Fatalf("second NewSession: %v", err)
	}

	sessions, err := run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(sessions); !reflect.DeepEqual(got, []string{"ssm-test-1", "ssm-test-10"}) {
		t.Errorf("sessions = %q, want [ssm-test-1 ssm-test-10]", got)
	}

	if err := s.SetOption("pane-border-status", "top"); err != nil {
		t.Fatal(err)
	}

	last := first
	for _, title := range []string{"i-aaa", "i-bbb", "i-ccc"} {
		if last, err = last.Split(); err != nil {
			t.Fatalf("Split: %v", err)
		}
		if err := last.SetTitle(title); err != nil {
			t.Fatal(err)
		}
		if err := s.Tile(); err != nil {
			t.Fatal(err)
		}
	}

	if err := last.SendLine("true"); err != nil {
		t.Fatal(err)
	}
	if err := first.Kill(); err != nil {
		t.Fatal(err)
	}

	titles, err := run("list-panes", "-t", s.WindowID, "-F", "#{pane_title}")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Split(titles, "\n"); !reflect.DeepEqual(got, []string{"i-aaa", "i-bbb", "i-ccc"}) {
		t.Errorf("pane titles = %q, want [i-aaa i-bbb i-ccc]", got)
	}
}
