package context

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/idursun/jjui/internal/askpass"
	"github.com/idursun/jjui/internal/ui/common"
)

type CommandRunner interface {
	RunCommandImmediate(args []string) ([]byte, error)
	RunCommandImmediateWithEnv(args []string, env []string) ([]byte, error)
	RunCommandStreaming(ctx context.Context, args []string) (*StreamingCommand, error)
	RunCommand(args []string, continuations ...tea.Cmd) tea.Cmd
	RunCommandWithInput(args []string, input string, continuations ...tea.Cmd) tea.Cmd
	RunInteractiveCommand(args []string, continuation tea.Cmd) tea.Cmd
}

type MainCommandRunner struct {
	Location  string
	Askpass   *askpass.Server
	idCounter atomic.Int64
}

// ViewSizeEnv returns terminal-size environment variables describing a jjui
// view that is w columns wide and h rows tall.
//
// Commands whose output is rendered inside a jjui view do not run in a
// pane-sized PTY: their stdout is a pipe, so width-sensitive tools fall back to
// a default width (jj resolves `$width` to 80, difftastic likewise) and lay out
// their output for the wrong size. Passing the view geometry through the
// conventional variables lets those tools wrap to the width we will actually
// render them at.
//
// Returns nil for non-positive sizes so callers can pass it straight to
// RunCommandImmediateWithEnv before the first window size is known.
func ViewSizeEnv(w, h int) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	return []string{
		"DFT_WIDTH=" + strconv.Itoa(w), // difftastic
		"COLUMNS=" + strconv.Itoa(w),
		"LINES=" + strconv.Itoa(h),
	}
}

func (a *MainCommandRunner) nextID() int { return int(a.idCounter.Add(1)) }

func (a *MainCommandRunner) RunCommandImmediateWithEnv(args []string, env []string) ([]byte, error) {
	c := exec.Command("jj", args...)
	c.Dir = a.Location
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	if output, err := c.Output(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, errors.New(string(exitError.Stderr))
		}
		return nil, err
	} else {
		return bytes.Trim(output, "\n"), nil
	}
}

func (a *MainCommandRunner) RunCommandImmediate(args []string) ([]byte, error) {
	return a.RunCommandImmediateWithEnv(args, nil)
}

func (a *MainCommandRunner) RunCommandStreaming(ctx context.Context, args []string) (*StreamingCommand, error) {
	c := exec.CommandContext(ctx, "jj", args...)
	c.Dir = a.Location
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err = c.Start(); err != nil {
		return nil, err
	}
	return &StreamingCommand{
		ReadCloser: pipe,
		ErrPipe:    errPipe,
		cmd:        c,
		ctx:        ctx,
	}, nil
}

func (a *MainCommandRunner) runCommandWithInput(args []string, input *string, continuations []tea.Cmd) tea.Cmd {
	id := a.nextID()
	command := "jj " + strings.Join(args, " ")
	commands := make([]tea.Cmd, 0)
	commands = append(commands,
		func() tea.Msg {
			started, cancel, env := a.Askpass.NewSubprocess(strings.Join(args, " "))
			defer cancel()
			if !slices.Contains(args, "--color") {
				args = append([]string{"--color", "always"}, args...)
			}
			c := exec.Command("jj", args...)
			c.Dir = a.Location
			c.Env = append(os.Environ(), env...)

			if input != nil {
				stdin, err := c.StdinPipe()
				if err != nil {
					return common.CommandCompletedMsg{
						ID:  id,
						Err: err,
					}
				}
				go func() {
					defer stdin.Close()
					io.WriteString(stdin, *input)
				}()
			}

			var output bytes.Buffer
			c.Stderr = &output
			if err := c.Start(); err != nil {
				return common.CommandCompletedMsg{
					ID:  id,
					Err: err,
				}
			}
			started(c.Process.Pid)

			err := c.Wait()
			if err != nil {
				var exitError *exec.ExitError
				if errors.As(err, &exitError) {
					err = errors.New(output.String())
				}
			}
			return common.CommandCompletedMsg{
				ID:     id,
				Output: output.String(),
				Err:    err,
			}
		})
	commands = append(commands, continuations...)
	return tea.Batch(
		func() tea.Msg {
			return common.CommandRunningMsg{ID: id, Command: command}
		},
		tea.Sequence(commands...),
	)
}

func (a *MainCommandRunner) RunCommandWithInput(args []string, input string, continuations ...tea.Cmd) tea.Cmd {
	return a.runCommandWithInput(args, &input, continuations)
}

func (a *MainCommandRunner) RunCommand(args []string, continuations ...tea.Cmd) tea.Cmd {
	return a.runCommandWithInput(args, nil, continuations)
}

func (a *MainCommandRunner) RunInteractiveCommand(args []string, continuation tea.Cmd) tea.Cmd {
	c := exec.Command("jj", args...)
	errBuffer := &bytes.Buffer{}
	c.Stderr = errBuffer
	c.Dir = a.Location
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return common.CommandCompletedMsg{Err: errors.New(errBuffer.String())}
		}
		if continuation != nil {
			return continuation()
		}
		return nil
	})
}

type StreamingCommand struct {
	io.ReadCloser
	ErrPipe io.ReadCloser
	cmd     *exec.Cmd
	ctx     context.Context
	once    sync.Once
}

func (c *StreamingCommand) Close() error {
	var err error
	c.once.Do(func() {
		log.Println("closing streaming command")
		pipeErr := c.ReadCloser.Close()

		if c.ctx.Err() != nil {
			log.Println("killing process due to context cancellation")
			if killErr := c.cmd.Process.Kill(); killErr != nil {
				err = killErr
				return
			}
		}

		log.Println("waiting for command to finish")
		err = c.cmd.Wait()
		if err != nil && (c.ctx.Err() != nil || errors.Is(err, os.ErrClosed)) {
			err = nil
		}

		if pipeErr != nil && err == nil {
			err = pipeErr
		}
	})
	return err
}

func (c *StreamingCommand) Wait() error {
	return c.cmd.Wait()
}
