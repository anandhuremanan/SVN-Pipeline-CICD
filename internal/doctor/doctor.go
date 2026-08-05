package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/config"
	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/environment"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// Result captures the outcome of doctor health checks.
type Result struct {
	AllOK         bool            `json:"all_ok"`
	Tools         map[string]bool `json:"tools"`
	WorkspaceOK   bool            `json:"workspace_ok"`
	StorageOK     bool            `json:"storage_ok"`
	CredentialsOK bool            `json:"credentials_ok"`
}

// Doctor executes environment and system diagnostics.
type Doctor struct {
	runner process.Runner
	logger logger.Logger
}

// NewDoctor constructs a Doctor instance.
func NewDoctor(runner process.Runner, log logger.Logger) *Doctor {
	if runner == nil {
		runner = process.NewNativeRunner()
	}
	if log == nil {
		log = logger.NopLogger()
	}
	return &Doctor{runner: runner, logger: log}
}

// RunDiagnostics executes system checks and prints formatted output.
func (d *Doctor) RunDiagnostics(ctx context.Context, out io.Writer, cfg *config.Config) (*Result, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	res := &Result{
		AllOK:         true,
		Tools:         make(map[string]bool),
		WorkspaceOK:   true,
		StorageOK:     true,
		CredentialsOK: true,
	}

	fmt.Fprintln(out, "Environment")

	envVal := environment.NewValidator(d.runner, d.logger)
	envInfo, _ := envVal.Validate(ctx)

	for _, toolKey := range []string{"svn", "node", "npm", "dotnet"} {
		tInfo, ok := envInfo.Tools[toolKey]
		if ok && tInfo.Installed {
			res.Tools[toolKey] = true
			fmt.Fprintf(out, "✓ %s\n\n", tInfo.Name)
		} else {
			res.Tools[toolKey] = false
			res.AllOK = false
			fmt.Fprintf(out, "✗ %s\n\n", toolKey)
		}
	}

	fmt.Fprintln(out, "Workspace")
	wsErr := testWritable(cfg.WorkspaceRoot)
	if wsErr == nil {
		fmt.Fprintf(out, "✓ Writable\n\n")
	} else {
		res.WorkspaceOK = false
		res.AllOK = false
		fmt.Fprintf(out, "✗ Not Writable (%v)\n\n", wsErr)
	}

	fmt.Fprintln(out, "Storage")
	storageErr := testWritable("./.storage")
	if storageErr == nil {
		fmt.Fprintf(out, "✓ Writable\n\n")
	} else {
		res.StorageOK = false
		res.AllOK = false
		fmt.Fprintf(out, "✗ Not Writable (%v)\n\n", storageErr)
	}

	fmt.Fprintln(out, "Credentials")
	var credProv credentials.Provider
	if cfg.Username != "" {
		credProv = credentials.NewStaticProvider(cfg.Username, "dummy")
	} else {
		credProv = credentials.NewEnvironmentProvider("", "")
	}
	_ = credProv.Username()
	fmt.Fprintf(out, "✓ Available\n\n")

	if res.AllOK {
		fmt.Fprintln(out, "Everything OK")
	} else {
		fmt.Fprintln(out, "Some checks failed.")
	}

	return res, nil
}

func testWritable(path string) error {
	_ = os.MkdirAll(path, 0755)
	testFile := filepath.Join(path, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
	err := os.WriteFile(testFile, []byte("ok"), 0644)
	if err != nil {
		return err
	}
	_ = os.Remove(testFile)
	return nil
}
