package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// StreamHandler provides real-time line-by-line output callbacks during process execution.
type StreamHandler interface {
	Stdout(line string)
	Stderr(line string)
}

// Runner interface abstracts process execution to allow mocking in tests.
type Runner interface {
	// Run executes an external command natively with context support.
	Run(ctx context.Context, opts models.ExecOptions, handler ...StreamHandler) (models.ProcessResult, error)
}

// NativeRunner implements Runner using Go's native os/exec package without shell invocation.
type NativeRunner struct {
	logger logger.Logger
}

// NewNativeRunner creates a new NativeRunner instance with an optional logger.
func NewNativeRunner(log ...logger.Logger) *NativeRunner {
	var l logger.Logger
	if len(log) > 0 && log[0] != nil {
		l = log[0]
	}
	return &NativeRunner{logger: l}
}

// Run executes the specified command directly without invoking cmd /C, powershell, or bash.
// Resolves executable names for Windows compatibility (e.g., npm -> npm.cmd).
func (r *NativeRunner) Run(ctx context.Context, opts models.ExecOptions, handlers ...StreamHandler) (models.ProcessResult, error) {
	if opts.Command == "" {
		return models.ProcessResult{}, fmt.Errorf("process runner: command cannot be empty")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	resolvedCmd := ResolveExecutable(opts.Command)
	if r.logger != nil && resolvedCmd != opts.Command {
		r.logger.Debug("resolved command executable", "requested", opts.Command, "resolved", resolvedCmd)
	}

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, resolvedCmd, opts.Args...)

	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	var handler StreamHandler
	if len(handlers) > 0 {
		handler = handlers[0]
	}

	outStream := &streamingWriter{callback: func(line string) {
		if handler != nil {
			handler.Stdout(line)
		}
	}}

	errStream := &streamingWriter{callback: func(line string) {
		if handler != nil {
			handler.Stderr(line)
		}
	}}

	cmd.Stdout = outStream
	cmd.Stderr = errStream

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	outStream.Flush()
	errStream.Flush()

	stdoutStr := outStream.String()
	stderrStr := errStream.String()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := models.NewProcessResult(
		opts.Command,
		exitCode,
		stdoutStr,
		stderrStr,
		duration,
		err,
	)

	return result, err
}

type streamingWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	lineBuf  bytes.Buffer
	callback func(string)
}

func (w *streamingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.buf.Write(p)
	if w.callback != nil {
		for _, b := range p {
			if b == '\n' {
				line := w.lineBuf.String()
				w.lineBuf.Reset()
				w.callback(strings.TrimRight(line, "\r"))
			} else {
				w.lineBuf.WriteByte(b)
			}
		}
	}
	return n, err
}

func (w *streamingWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.callback != nil && w.lineBuf.Len() > 0 {
		line := w.lineBuf.String()
		w.lineBuf.Reset()
		w.callback(strings.TrimRight(line, "\r"))
	}
}

func (w *streamingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
