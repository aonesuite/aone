package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process"
	"github.com/aonesuite/aone/packages/go/sandbox/internal/envdapi/process/processconnect"
)

// CommandResult contains the final observable output of a completed process.
// Stdout and Stderr contain accumulated stream data, while Error contains the
// process-level error text reported by envd when the command fails to start or
// exits abnormally.
type CommandResult struct {
	// ExitCode is the numeric process exit status returned by the sandbox.
	ExitCode int
	// Stdout is the complete standard-output stream captured by the SDK.
	Stdout string
	// Stderr is the complete standard-error stream captured by the SDK.
	Stderr string
	// Error is an envd-provided error message, when one is available.
	Error string
}

// CommandExitError is returned by WaitOK when a command exits with a non-zero
// status. It carries the full command result for diagnostics.
type CommandExitError struct {
	Result *CommandResult
}

func (e *CommandExitError) Error() string {
	if e.Result == nil {
		return "command exited with non-zero status"
	}
	if e.Result.Error != "" {
		return e.Result.Error
	}
	return fmt.Sprintf("command exited with status %d", e.Result.ExitCode)
}

// CommandHandle represents a process that has been started inside a sandbox.
// It can be used to wait for completion, wait only until the PID is known, or
// send a termination signal through Commands.Kill.
type CommandHandle struct {
	pid uint32

	commands *Commands
	cancel   context.CancelFunc
	done     chan struct{}
	pidCh    chan struct{}
	result   *CommandResult
	stdout   []byte
	stderr   []byte

	mu        sync.Mutex
	onStdout  func(data []byte)
	onStderr  func(data []byte)
	onPtyData func(data []byte)
}

// PID returns the process identifier assigned inside the sandbox. The value is
// populated after the start event arrives; call WaitPID when the caller must
// block until the PID is available.
func (h *CommandHandle) PID() uint32 {
	return h.pid
}

// Wait blocks until the command stream ends and returns the final result. The
// method does not kill the process; cancellation is driven by the context passed
// to Start, by WithTimeout, or by calling Kill.
func (h *CommandHandle) Wait() (*CommandResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result == nil {
		return nil, fmt.Errorf("command terminated without result")
	}
	return h.result, nil
}

// WaitOK waits for completion and returns CommandExitError for non-zero exit
// codes, matching JS SDK wait semantics while preserving Wait.
func (h *CommandHandle) WaitOK() (*CommandResult, error) {
	result, err := h.Wait()
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, &CommandExitError{Result: result}
	}
	return result, nil
}

// Disconnect stops receiving process events without killing the process.
func (h *CommandHandle) Disconnect() {
	h.cancel()
}

// Stdout returns stdout accumulated so far.
func (h *CommandHandle) Stdout() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return string(h.stdout)
}

// Stderr returns stderr accumulated so far.
func (h *CommandHandle) Stderr() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return string(h.stderr)
}

// ExitCode returns the process exit code when the command has completed.
func (h *CommandHandle) ExitCode() *int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result == nil {
		return nil
	}
	code := h.result.ExitCode
	return &code
}

// ErrorMessage returns the process error message when present.
func (h *CommandHandle) ErrorMessage() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result == nil {
		return ""
	}
	return h.result.Error
}

// Kill asks the sandbox process service to terminate the command represented by
// this handle.
func (h *CommandHandle) Kill(ctx context.Context) error {
	return h.commands.Kill(ctx, h.pid)
}

// WaitPID blocks until the sandbox reports the process PID. This is useful for
// long-running commands that need to be tracked or killed before they complete.
func (h *CommandHandle) WaitPID(ctx context.Context) (uint32, error) {
	select {
	case <-h.pidCh:
		return h.pid, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// ProcessInfo describes a process currently known to the sandbox process
// service. Tags, environment variables, and working directory are present only
// when they were supplied at process creation time or reported by envd.
type ProcessInfo struct {
	// PID is the process identifier inside the sandbox.
	PID uint32
	// Tag is an optional caller-defined label that can be used to identify a
	// process across list/connect operations.
	Tag *string
	// Cmd is the executable path or shell command used to start the process.
	Cmd string
	// Args are the command arguments passed to the process.
	Args []string
	// Envs contains the environment variables supplied at process start.
	Envs map[string]string
	// Cwd is the process working directory when one is known.
	Cwd *string
}

// CommandOption customizes command creation. Options are applied in order, so
// later options can override values set by earlier options.
type CommandOption func(*commandOpts)

type commandOpts struct {
	envs      map[string]string
	cwd       string
	user      string
	tag       string
	onStdout  func(data []byte)
	onStderr  func(data []byte)
	onPtyData func(data []byte)
	timeout   time.Duration
	stdin     bool
}

// WithEnvs sets environment variables for the process. The map is sent as-is;
// callers should avoid mutating it after passing it to the SDK.
func WithEnvs(envs map[string]string) CommandOption {
	return func(o *commandOpts) { o.envs = envs }
}

// WithCwd sets the working directory for the process inside the sandbox.
func WithCwd(cwd string) CommandOption {
	return func(o *commandOpts) { o.cwd = cwd }
}

// WithCommandUser runs the process as the named sandbox user. When omitted, the
// SDK uses DefaultUser.
func WithCommandUser(user string) CommandOption {
	return func(o *commandOpts) { o.user = user }
}

// WithTag attaches a caller-defined label to the process so it can be found
// later through process listing or connected to by PID-aware workflows.
func WithTag(tag string) CommandOption {
	return func(o *commandOpts) { o.tag = tag }
}

// WithOnStdout registers a callback for stdout chunks as they arrive. The SDK
// still accumulates stdout into CommandResult.Stdout.
func WithOnStdout(fn func(data []byte)) CommandOption {
	return func(o *commandOpts) { o.onStdout = fn }
}

// WithOnStderr registers a callback for stderr chunks as they arrive. The SDK
// still accumulates stderr into CommandResult.Stderr.
func WithOnStderr(fn func(data []byte)) CommandOption {
	return func(o *commandOpts) { o.onStderr = fn }
}

// WithOnPtyData registers a callback for raw PTY output. It is primarily used
// by interactive terminals where stdout/stderr are merged by the pseudo-terminal.
func WithOnPtyData(fn func(data []byte)) CommandOption {
	return func(o *commandOpts) { o.onPtyData = fn }
}

// WithTimeout bounds the command lifetime. When the duration expires, the
// command context is canceled and envd closes the command stream.
func WithTimeout(timeout time.Duration) CommandOption {
	return func(o *commandOpts) { o.timeout = timeout }
}

// WithStdin keeps stdin open for the process so callers can stream input later.
func WithStdin() CommandOption {
	return func(o *commandOpts) { o.stdin = true }
}

func applyCommandOpts(opts []CommandOption) *commandOpts {
	o := &commandOpts{user: DefaultUser}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// Commands provides process execution helpers for a single sandbox. Use
// Sandbox.Commands to obtain an instance bound to the sandbox's envd endpoint.
type Commands struct {
	sandbox *Sandbox
	rpc     processconnect.ProcessClient
}

func newCommands(s *Sandbox, rpc processconnect.ProcessClient) *Commands {
	return &Commands{sandbox: s, rpc: rpc}
}

func pidSelector(pid uint32) *process.ProcessSelector {
	return &process.ProcessSelector{Selector: &process.ProcessSelector_Pid{Pid: pid}}
}

// Run starts a shell command and waits until it completes. The command is
// executed through "/bin/bash -l -c" so login-shell initialization is applied.
func (c *Commands) Run(ctx context.Context, cmd string, opts ...CommandOption) (*CommandResult, error) {
	handle, err := c.Start(ctx, cmd, opts...)
	if err != nil {
		return nil, err
	}
	return handle.Wait()
}

// Start starts a shell command and returns immediately with a handle. The caller
// can use the returned handle to wait for the PID, stream output through
// callbacks, wait for completion, or kill the process.
func (c *Commands) Start(ctx context.Context, cmd string, opts ...CommandOption) (*CommandHandle, error) {
	o := applyCommandOpts(opts)

	cmdCtx := ctx
	var cmdCancel context.CancelFunc
	if o.timeout > 0 {
		cmdCtx, cmdCancel = context.WithTimeout(ctx, o.timeout)
	} else {
		cmdCtx, cmdCancel = context.WithCancel(ctx)
	}

	startReq := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-l", "-c", cmd},
			Envs: o.envs,
		},
	}
	if o.cwd != "" {
		startReq.Process.Cwd = &o.cwd
	}
	if o.tag != "" {
		startReq.Tag = &o.tag
	}
	startReq.Stdin = &o.stdin

	req := connect.NewRequest(startReq)
	c.sandbox.setEnvdAuth(req, o.user)

	stream, err := c.rpc.Start(cmdCtx, req)
	if err != nil {
		cmdCancel()
		return nil, fmt.Errorf("start command: %w", err)
	}

	handle := &CommandHandle{
		commands: c,
		cancel:   cmdCancel,
		done:     make(chan struct{}),
		pidCh:    make(chan struct{}),
		onStdout: o.onStdout,
		onStderr: o.onStderr,
	}

	go processEventStream(stream, handle)

	return handle, nil
}

type eventMessage interface {
	GetEvent() *process.ProcessEvent
}

type streamReceiver[T eventMessage] interface {
	Receive() bool
	Msg() T
	Err() error
}

func processEventStream[T eventMessage](stream streamReceiver[T], handle *CommandHandle) {
	defer close(handle.done)

	var stdout, stderr []byte
	for stream.Receive() {
		event := stream.Msg().GetEvent()
		if event == nil {
			continue
		}
		switch ev := event.Event.(type) {
		case *process.ProcessEvent_Start:
			handle.pid = ev.Start.Pid
			close(handle.pidCh)
		case *process.ProcessEvent_Data:
			if data := ev.Data.GetStdout(); len(data) > 0 {
				stdout = append(stdout, data...)
				handle.mu.Lock()
				handle.stdout = append(handle.stdout, data...)
				fn := handle.onStdout
				handle.mu.Unlock()
				if fn != nil {
					fn(data)
				}
			}
			if data := ev.Data.GetStderr(); len(data) > 0 {
				stderr = append(stderr, data...)
				handle.mu.Lock()
				handle.stderr = append(handle.stderr, data...)
				fn := handle.onStderr
				handle.mu.Unlock()
				if fn != nil {
					fn(data)
				}
			}
			if data := ev.Data.GetPty(); len(data) > 0 {
				handle.mu.Lock()
				fn := handle.onPtyData
				handle.mu.Unlock()
				if fn != nil {
					fn(data)
				}
			}
		case *process.ProcessEvent_End:
			result := &CommandResult{
				ExitCode: int(ev.End.ExitCode),
				Stdout:   string(stdout),
				Stderr:   string(stderr),
			}
			if ev.End.Error != nil {
				result.Error = *ev.End.Error
			}
			handle.mu.Lock()
			handle.result = result
			handle.mu.Unlock()
		}
	}

	if handle.result == nil {
		errMsg := ""
		if err := stream.Err(); err != nil {
			errMsg = err.Error()
		}
		result := &CommandResult{
			ExitCode: -1,
			Stdout:   string(stdout),
			Stderr:   string(stderr),
			Error:    errMsg,
		}
		handle.mu.Lock()
		handle.result = result
		handle.mu.Unlock()
	}
}

// Connect attaches to an existing process stream by PID. It is intended for
// processes that are still running and whose output stream is available in envd.
func (c *Commands) Connect(ctx context.Context, pid uint32) (*CommandHandle, error) {
	connectCtx, connectCancel := context.WithCancel(ctx)

	req := connect.NewRequest(&process.ConnectRequest{
		Process: pidSelector(pid),
	})
	c.sandbox.setEnvdAuth(req, DefaultUser)

	stream, err := c.rpc.Connect(connectCtx, req)
	if err != nil {
		connectCancel()
		return nil, fmt.Errorf("connect to process: %w", err)
	}

	pidCh := make(chan struct{})
	close(pidCh)

	handle := &CommandHandle{
		pid:      pid,
		commands: c,
		cancel:   connectCancel,
		done:     make(chan struct{}),
		pidCh:    pidCh,
	}

	go processEventStream(stream, handle)

	return handle, nil
}

// List returns the processes currently tracked by the sandbox process service.
// The result is a point-in-time snapshot and may change immediately after the
// call returns as processes start or exit.
func (c *Commands) List(ctx context.Context) ([]ProcessInfo, error) {
	req := connect.NewRequest(&process.ListRequest{})
	c.sandbox.setEnvdAuth(req, DefaultUser)

	resp, err := c.rpc.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	var infos []ProcessInfo
	for _, p := range resp.Msg.Processes {
		info := ProcessInfo{
			PID: p.Pid,
			Tag: p.Tag,
		}
		if p.Config != nil {
			info.Cmd = p.Config.Cmd
			info.Args = p.Config.Args
			info.Envs = p.Config.Envs
			info.Cwd = p.Config.Cwd
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// SendStdin writes bytes to a process stdin stream. The process must have been
// started with WithStdin; otherwise envd may reject the write or the process may
// have no open stdin reader.
func (c *Commands) SendStdin(ctx context.Context, pid uint32, data []byte) error {
	req := connect.NewRequest(&process.SendInputRequest{
		Process: pidSelector(pid),
		Input: &process.ProcessInput{
			Input: &process.ProcessInput_Stdin{Stdin: data},
		},
	})
	c.sandbox.setEnvdAuth(req, DefaultUser)

	_, err := c.rpc.SendInput(ctx, req)
	if err != nil {
		return fmt.Errorf("send stdin: %w", err)
	}
	return nil
}

// CloseStdin closes the stdin stream for a process. This is useful for commands
// that wait for EOF before exiting, such as interpreters or tools reading from
// standard input.
func (c *Commands) CloseStdin(ctx context.Context, pid uint32) error {
	req := connect.NewRequest(&process.CloseStdinRequest{
		Process: pidSelector(pid),
	})
	c.sandbox.setEnvdAuth(req, DefaultUser)

	_, err := c.rpc.CloseStdin(ctx, req)
	if err != nil {
		return fmt.Errorf("close stdin: %w", err)
	}
	return nil
}

// Kill sends SIGKILL to a process inside the sandbox. The operation is
// intentionally forceful and does not give the process a graceful shutdown hook.
func (c *Commands) Kill(ctx context.Context, pid uint32) error {
	req := connect.NewRequest(&process.SendSignalRequest{
		Process: pidSelector(pid),
		Signal:  process.Signal_SIGNAL_SIGKILL,
	})
	c.sandbox.setEnvdAuth(req, DefaultUser)

	_, err := c.rpc.SendSignal(ctx, req)
	if err != nil {
		return fmt.Errorf("kill process: %w", err)
	}
	return nil
}
