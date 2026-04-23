package sandbox

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process/processconnect"
)

// PtySize describes the terminal dimensions for a pseudo-terminal session.
type PtySize struct {
	// Cols is the terminal width in character cells.
	Cols uint32
	// Rows is the terminal height in character cells.
	Rows uint32
}

// Pty provides interactive pseudo-terminal operations for a sandbox. It shares
// the same process service as Commands but sends and receives PTY data instead
// of separate stdout/stderr streams.
type Pty struct {
	sandbox *Sandbox
	rpc     processconnect.ProcessClient
}

func newPty(s *Sandbox, rpc processconnect.ProcessClient) *Pty {
	return &Pty{sandbox: s, rpc: rpc}
}

// Create starts an interactive login shell attached to a new pseudo-terminal.
// Output is delivered through WithOnPtyData, or WithOnStdout when no PTY callback
// is provided.
func (p *Pty) Create(ctx context.Context, size PtySize, opts ...CommandOption) (*CommandHandle, error) {
	o := applyCommandOpts(opts)

	ptyCtx, ptyCancel := context.WithCancel(ctx)

	envs := map[string]string{
		"TERM":   "xterm",
		"LANG":   "C.UTF-8",
		"LC_ALL": "C.UTF-8",
	}
	for k, v := range o.envs {
		envs[k] = v
	}

	startReq := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-i", "-l"},
			Envs: envs,
		},
		Pty: &process.PTY{
			Size: &process.PTY_Size{
				Cols: size.Cols,
				Rows: size.Rows,
			},
		},
	}
	if o.cwd != "" {
		startReq.Process.Cwd = &o.cwd
	}
	if o.tag != "" {
		startReq.Tag = &o.tag
	}

	req := connect.NewRequest(startReq)
	p.sandbox.setEnvdAuth(req, o.user)

	stream, err := p.rpc.Start(ptyCtx, req)
	if err != nil {
		ptyCancel()
		return nil, fmt.Errorf("create pty: %w", err)
	}

	commands := &Commands{sandbox: p.sandbox, rpc: p.rpc}

	ptyDataFn := o.onPtyData
	if ptyDataFn == nil {
		ptyDataFn = o.onStdout
	}

	handle := &CommandHandle{
		commands:  commands,
		cancel:    ptyCancel,
		done:      make(chan struct{}),
		pidCh:     make(chan struct{}),
		onPtyData: ptyDataFn,
	}

	go processEventStream(stream, handle)

	return handle, nil
}

// Connect attaches to an existing PTY process by PID.
func (p *Pty) Connect(ctx context.Context, pid uint32) (*CommandHandle, error) {
	commands := &Commands{sandbox: p.sandbox, rpc: p.rpc}
	return commands.Connect(ctx, pid)
}

// SendInput writes raw bytes to the PTY input stream for pid. Callers should
// include terminal control sequences exactly as they should be received.
func (p *Pty) SendInput(ctx context.Context, pid uint32, data []byte) error {
	req := connect.NewRequest(&process.SendInputRequest{
		Process: pidSelector(pid),
		Input: &process.ProcessInput{
			Input: &process.ProcessInput_Pty{Pty: data},
		},
	})
	p.sandbox.setEnvdAuth(req, DefaultUser)

	_, err := p.rpc.SendInput(ctx, req)
	if err != nil {
		return fmt.Errorf("send pty input: %w", err)
	}
	return nil
}

// Resize updates the PTY dimensions for pid so full-screen terminal programs can
// redraw against the current client window size.
func (p *Pty) Resize(ctx context.Context, pid uint32, size PtySize) error {
	req := connect.NewRequest(&process.UpdateRequest{
		Process: pidSelector(pid),
		Pty: &process.PTY{
			Size: &process.PTY_Size{
				Cols: size.Cols,
				Rows: size.Rows,
			},
		},
	})
	p.sandbox.setEnvdAuth(req, DefaultUser)

	_, err := p.rpc.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	return nil
}

// Kill sends SIGKILL to a PTY process. It is equivalent to Commands.Kill but is
// provided here for callers that work entirely through the PTY helper.
func (p *Pty) Kill(ctx context.Context, pid uint32) error {
	commands := &Commands{sandbox: p.sandbox, rpc: p.rpc}
	return commands.Kill(ctx, pid)
}
