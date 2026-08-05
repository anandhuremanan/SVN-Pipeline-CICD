package models

import "time"

// ExecOptions defines execution parameters for running an external process.
type ExecOptions struct {
	// Command is the binary name or path to execute (e.g., "svn", "npm", "dotnet").
	Command string `json:"command"`
	// Args contains command line arguments.
	Args []string `json:"args"`
	// Dir specifies the working directory for process execution.
	Dir string `json:"dir"`
	// Env specifies additional environment variables in KEY=VALUE format.
	Env []string `json:"env,omitempty"`
	// Timeout specifies maximum duration before killing the process (0 for no timeout).
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ProcessResult captures the output, execution metrics, and exit status of a completed process.
type ProcessResult struct {
	// Command is the executed binary name.
	Command string `json:"command"`
	// ExitCode is the process exit status code (0 for success).
	ExitCode int `json:"exit_code"`
	// Successful indicates whether the process completed successfully with exit code 0.
	Successful bool `json:"successful"`
	// StdOut contains captured standard output.
	StdOut string `json:"stdout"`
	// StdErr contains captured standard error output.
	StdErr string `json:"stderr"`
	// Duration measures the runtime execution duration of the command.
	Duration time.Duration `json:"duration"`
	// ErrorMessage contains the error message string if execution failed.
	ErrorMessage string `json:"error_message,omitempty"`
}

// NewProcessResult constructs a ProcessResult, automatically setting Successful based on exit code and error.
func NewProcessResult(cmd string, exitCode int, stdout, stderr string, duration time.Duration, err error) ProcessResult {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return ProcessResult{
		Command:      cmd,
		ExitCode:     exitCode,
		Successful:   exitCode == 0 && err == nil,
		StdOut:       stdout,
		StdErr:       stderr,
		Duration:     duration,
		ErrorMessage: errMsg,
	}
}
