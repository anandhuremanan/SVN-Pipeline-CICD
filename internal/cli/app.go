package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/config"
	"github.com/anandhuremanan/cicd-engine/internal/doctor"
	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// RunRequest encapsulates CLI parameters passed to the engine.
type RunRequest struct {
	PipelineFile    string `json:"pipeline_file"`
	PipelineContent string `json:"pipeline_content,omitempty"`
	RepositoryURL   string `json:"repository_url"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfigPath      string `json:"config_path"`
	KeepWorkspace   bool   `json:"keep_workspace"`
	Revision        string `json:"revision"`
	// OutputDir is a folder (local or network share) that receives the published
	// output under <OutputDir>/<repo>/Build-NNNN/<project>. Overrides YAML outputs.
	OutputDir string `json:"output_dir,omitempty"`
	// WorkspaceRoot overrides the directory where the checkout workspace is created.
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	// CleanWorkspace removes the workspace after the run regardless of the config file.
	CleanWorkspace bool `json:"clean_workspace,omitempty"`
	// DeferCleanup leaves the workspace untouched when the run ends; the caller removes it
	// (see the job's Workspace path). Lets a UI show the result before a slow cleanup.
	DeferCleanup bool `json:"-"`
	// ReuseDependencies keeps each Node project's node_modules between runs, in a ".deps" folder
	// inside the workspace root, so that repeat builds skip most of the install.
	ReuseDependencies bool `json:"reuse_dependencies,omitempty"`
	// CleanDependencies discards the kept node_modules of the projects in this run first, for a
	// build that installs everything from scratch.
	CleanDependencies bool `json:"clean_dependencies,omitempty"`
	// ProjectPaths limits the build to projects inside these repository folders. Empty builds everything.
	ProjectPaths []string `json:"project_paths,omitempty"`
	// BuildScript is the npm script used for Node projects that have no script set in YAML. Empty means "build".
	BuildScript string `json:"build_script,omitempty"`
	// ProjectScripts sets the npm script for individual ProjectPaths entries, keyed by the folder as written there.
	ProjectScripts map[string]string `json:"project_scripts,omitempty"`
	// OnFinish is invoked with the final job record once the run ends.
	OnFinish func(job models.Job) `json:"-"`
}

// Engine defines the pipeline execution entry point interface.
type Engine interface {
	// Run triggers pipeline discovery and build execution.
	Run(ctx context.Context, req RunRequest) error
	// GetHistory retrieves historical completed jobs.
	GetHistory() ([]models.Job, error)
	// GetStatus retrieves active or queued jobs.
	GetStatus() ([]models.Job, error)
}

// StubEngine is a stub pipeline engine used when engine is nil.
type StubEngine struct{}

// Run implements Engine for StubEngine.
func (s *StubEngine) Run(ctx context.Context, req RunRequest) error {
	return nil
}

// GetHistory implements Engine for StubEngine.
func (s *StubEngine) GetHistory() ([]models.Job, error) {
	return []models.Job{}, nil
}

// GetStatus implements Engine for StubEngine.
func (s *StubEngine) GetStatus() ([]models.Job, error) {
	return []models.Job{}, nil
}

// ServerRunner defines function signature for starting the web dashboard HTTP server.
type ServerRunner func(ctx context.Context, addr string) error

// App encapsulates CLI setup, argument parsing, and engine invocation.
type App struct {
	Out          io.Writer
	Err          io.Writer
	Engine       Engine
	ServerRunner ServerRunner
}

// NewApp creates a new CLI App instance with standard IO writers and engine.
func NewApp(out, err io.Writer, engine Engine) *App {
	if out == nil {
		out = os.Stdout
	}
	if err == nil {
		err = os.Stderr
	}
	if engine == nil {
		engine = &StubEngine{}
	}
	return &App{
		Out:    out,
		Err:    err,
		Engine: engine,
	}
}

// Run parses CLI arguments, builds RunRequest, and triggers engine execution.
func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) < 1 {
		a.printUsage()
		return fmt.Errorf("no command provided")
	}

	command := args[0]
	switch command {
	case "run":
		return a.handleRun(ctx, args[1:])
	case "server", "serve":
		return a.handleServer(ctx, args[1:])
	case "history":
		return a.handleHistory()
	case "status":
		return a.handleStatus()
	case "doctor":
		return a.handleDoctor(ctx, args[1:])
	case "-h", "--help", "help":
		a.printUsage()
		return nil
	default:
		a.printUsage()
		return fmt.Errorf("unknown command: %q", command)
	}
}

func (a *App) handleRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var req RunRequest

	fs.StringVar(&req.ConfigPath, "config", "", "Path to configuration file")
	fs.StringVar(&req.Username, "username", "", "SVN repository username")
	fs.StringVar(&req.Password, "password", "", "SVN repository password (development only)")
	fs.StringVar(&req.Revision, "revision", "HEAD", "SVN repository target revision")
	fs.BoolVar(&req.KeepWorkspace, "keep-workspace", false, "Keep workspace directory after execution")
	fs.StringVar(&req.OutputDir, "output", "", "Folder (local or network share) that receives each build under a unique build number")
	fs.StringVar(&req.WorkspaceRoot, "workspace", "", "Directory where the checkout workspace is created")
	fs.BoolVar(&req.CleanWorkspace, "clean-workspace", false, "Remove workspace directory after execution")
	fs.StringVar(&req.BuildScript, "script", "", "npm script used to build Node projects (default \"build\")")
	fs.BoolVar(&req.ReuseDependencies, "reuse-deps", false, "Keep node_modules between runs to speed up repeat builds")
	fs.BoolVar(&req.CleanDependencies, "clean-deps", false, "Discard kept node_modules for this run's projects and install from scratch")

	var projectPaths string
	fs.StringVar(&projectPaths, "project", "", "Comma-separated repository folders to build (default: every project found)")

	var flagArgs []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && (arg == "--config" || arg == "-config" ||
				arg == "--username" || arg == "-username" ||
				arg == "--password" || arg == "-password" ||
				arg == "--revision" || arg == "-revision" ||
				arg == "--output" || arg == "-output" ||
				arg == "--workspace" || arg == "-workspace" ||
				arg == "--script" || arg == "-script" ||
				arg == "--project" || arg == "-project") {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: cicd run <cicd.yaml|svn-url> [flags]")
	}

	req.ProjectPaths = SplitProjectPaths(projectPaths)

	targetArg := positional[0]
	if strings.HasSuffix(targetArg, ".yaml") || strings.HasSuffix(targetArg, ".yml") || fileExists(targetArg) {
		req.PipelineFile = targetArg
	} else {
		req.RepositoryURL = targetArg
	}

	if req.RepositoryURL == "" && req.PipelineFile == "" {
		return fmt.Errorf("target pipeline file or svn-url cannot be empty")
	}

	return a.Engine.Run(ctx, req)
}

func (a *App) handleServer(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var port int
	var host string

	fs.IntVar(&port, "port", 8080, "HTTP dashboard listening port")
	fs.StringVar(&host, "host", "0.0.0.0", "HTTP dashboard listening host")

	if err := fs.Parse(args); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Fprintf(a.Out, "Starting CI/CD Dashboard HTTP Server on http://%s\n", addr)

	if a.ServerRunner != nil {
		return a.ServerRunner(ctx, addr)
	}

	return nil
}

// SplitProjectPaths parses a comma- or semicolon-separated list of repository folders.
func SplitProjectPaths(list string) []string {
	var paths []string
	for _, p := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ';' }) {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (a *App) handleHistory() error {
	jobs, err := a.Engine.GetHistory()
	if err != nil {
		return fmt.Errorf("failed to retrieve build history: %w", err)
	}

	fmt.Fprintln(a.Out, "Build History")
	fmt.Fprintln(a.Out, "")

	if len(jobs) == 0 {
		fmt.Fprintln(a.Out, "No previous build records found.")
		return nil
	}

	for _, j := range jobs {
		fmt.Fprintf(a.Out, "Job ID:     %s\n", j.ID)
		fmt.Fprintf(a.Out, "Repository: %s\n", j.Repository)
		fmt.Fprintf(a.Out, "Revision:   %s\n", j.Revision)
		fmt.Fprintf(a.Out, "Status:     %s\n", j.Status)
		fmt.Fprintf(a.Out, "Started:    %s\n", j.StartedAt.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(a.Out, "Duration:   %v\n", j.Duration)
		fmt.Fprintf(a.Out, "Artifacts:  %d\n\n", len(j.Artifacts))
	}
	return nil
}

func (a *App) handleStatus() error {
	jobs, err := a.Engine.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to retrieve queue status: %w", err)
	}

	fmt.Fprintln(a.Out, "Currently Executing / Queued Jobs")
	fmt.Fprintln(a.Out, "")

	if len(jobs) == 0 {
		fmt.Fprintln(a.Out, "No jobs currently running or queued.")
		return nil
	}

	for _, j := range jobs {
		fmt.Fprintf(a.Out, "Job ID:     %s\n", j.ID)
		fmt.Fprintf(a.Out, "Repository: %s\n", j.Repository)
		fmt.Fprintf(a.Out, "Revision:   %s\n", j.Revision)
		fmt.Fprintf(a.Out, "Status:     %s\n\n", j.Status)
	}
	return nil
}

func (a *App) handleDoctor(ctx context.Context, args []string) error {
	configPath := ""
	for i := 0; i < len(args); i++ {
		if (args[i] == "--config" || args[i] == "-config") && i+1 < len(args) {
			configPath = args[i+1]
			break
		}
	}

	cfg, _ := config.Load(configPath)
	doc := doctor.NewDoctor(nil, nil)
	res, err := doc.RunDiagnostics(ctx, a.Out, cfg)
	if err != nil {
		return err
	}
	if !res.AllOK {
		return fmt.Errorf("doctor diagnostics found issues")
	}
	return nil
}

func (a *App) printUsage() {
	fmt.Fprintln(a.Out, "CI/CD Engine MVP CLI")
	fmt.Fprintln(a.Out, "Usage:")
	fmt.Fprintln(a.Out, "  cicd run <cicd.yaml|svn-url> [flags]")
	fmt.Fprintln(a.Out, "  cicd server [--port 8080] [--host 0.0.0.0]")
	fmt.Fprintln(a.Out, "  cicd history")
	fmt.Fprintln(a.Out, "  cicd status")
	fmt.Fprintln(a.Out, "  cicd doctor")
	fmt.Fprintln(a.Out, "")
	fmt.Fprintln(a.Out, "Flags:")
	fmt.Fprintln(a.Out, "  --config string       Path to configuration file")
	fmt.Fprintln(a.Out, "  --username string     SVN repository username")
	fmt.Fprintln(a.Out, "  --password string     SVN repository password (development only)")
	fmt.Fprintln(a.Out, "  --revision string     SVN target revision (default \"HEAD\")")
	fmt.Fprintln(a.Out, "  --keep-workspace      Keep workspace directory after run")
	fmt.Fprintln(a.Out, "  --output string       Folder or network share that receives each build under a unique build number")
	fmt.Fprintln(a.Out, "  --workspace string    Directory where the checkout workspace is created")
	fmt.Fprintln(a.Out, "  --clean-workspace     Remove workspace directory after run")
	fmt.Fprintln(a.Out, "  --project string      Comma-separated repository folders to build (default: every project found)")
	fmt.Fprintln(a.Out, "  --script string       npm script used to build Node projects (default \"build\")")
	fmt.Fprintln(a.Out, "  --reuse-deps          Keep node_modules between runs to speed up repeat builds")
	fmt.Fprintln(a.Out, "  --clean-deps          Discard kept node_modules for this run's projects first")
}
