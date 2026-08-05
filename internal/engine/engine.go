package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/builder"
	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/config"
	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/environment"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/manifest"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/pipeline"
	"github.com/anandhuremanan/cicd-engine/internal/planner"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/publisher"
	"github.com/anandhuremanan/cicd-engine/internal/queue"
	"github.com/anandhuremanan/cicd-engine/internal/reporter"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
	"github.com/anandhuremanan/cicd-engine/internal/retry"
	"github.com/anandhuremanan/cicd-engine/internal/sanitizer"
	"github.com/anandhuremanan/cicd-engine/internal/scanner"
	"github.com/anandhuremanan/cicd-engine/internal/scm"
	"github.com/anandhuremanan/cicd-engine/internal/scm/svn"
	"github.com/anandhuremanan/cicd-engine/internal/storage/builds"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
	"github.com/anandhuremanan/cicd-engine/internal/scriptvalidator"
	"github.com/anandhuremanan/cicd-engine/internal/workspace"
)

// Engine is the central orchestration engine for CI/CD pipeline runs.
type Engine struct {
	out          io.Writer
	processRun   process.Runner
	envValidator environment.Validator
	scmProvider  scm.Provider
	sanitizer    sanitizer.Sanitizer
	projScanner  scanner.Scanner
	bRegistry    builder.Registry
	bReporter    reporter.Reporter
	bPublisher   publisher.Publisher
	bPlanner     planner.Planner
	buildStore   builds.BuildStore
	logStore     logs.LogStore
	jobQueue     queue.Queue
}

// NewEngine creates a new Engine instance with injected dependencies.
func NewEngine(
	out io.Writer,
	runner process.Runner,
	envVal environment.Validator,
	scmProv scm.Provider,
	san sanitizer.Sanitizer,
	scan scanner.Scanner,
	reg builder.Registry,
	rep reporter.Reporter,
	bStore builds.BuildStore,
	lStore logs.LogStore,
	q queue.Queue,
) *Engine {
	if out == nil {
		out = os.Stdout
	}
	if runner == nil {
		runner = process.NewNativeRunner()
	}
	if bStore == nil {
		bStore, _ = builds.NewFileStore("")
	}
	if lStore == nil {
		lStore, _ = logs.NewFileLogStore("")
	}
	if q == nil {
		q = queue.NewMemoryQueue(100)
	}

	eng := &Engine{
		out:          out,
		processRun:   runner,
		envValidator: envVal,
		scmProvider:  scmProv,
		sanitizer:    san,
		projScanner:  scan,
		bRegistry:    reg,
		bReporter:    rep,
		bPublisher:   publisher.NewDefaultPublisher(nil),
		buildStore:   bStore,
		logStore:     lStore,
		jobQueue:     q,
	}

	eng.recoverStaleJobs()
	return eng
}

// SetPublisher allows injecting a custom Publisher implementation.
func (e *Engine) SetPublisher(pub publisher.Publisher) {
	if pub != nil {
		e.bPublisher = pub
	}
}

// SetPlanner allows injecting a custom Planner implementation.
func (e *Engine) SetPlanner(pl planner.Planner) {
	if pl != nil {
		e.bPlanner = pl
	}
}

// recoverStaleJobs inspects previous jobs and updates any stuck RUNNING job to INTERRUPTED.
func (e *Engine) recoverStaleJobs() {
	if e.buildStore == nil {
		return
	}
	jobs, err := e.buildStore.List()
	if err != nil {
		return
	}

	for _, j := range jobs {
		if j.Status == models.JobStatusRunning {
			j.Status = models.JobStatusInterrupted
			j.ErrorMessage = "job interrupted by previous process termination"
			j.FinishedAt = time.Now()
			_ = e.buildStore.Save(j)
		}
	}
}

// Run creates a Job and executes the pipeline flow with graceful shutdown handling.
func (e *Engine) Run(parentCtx context.Context, req cli.RunRequest) error {
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return fmt.Errorf("engine: failed to load configuration: %w", err)
	}

	var pipelineConfig *planner.PipelineConfig
	if req.PipelineFile != "" {
		fmt.Fprintln(e.out, "Loading Pipeline")
		fmt.Fprintf(e.out, "✓ %s\n\n", req.PipelineFile)

		loader := planner.NewDefaultLoader()
		pCfg, pErr := loader.Load(req.PipelineFile)
		if pErr != nil {
			return pErr
		}

		validator := planner.NewDefaultValidator()
		if vErr := validator.Validate(pCfg); vErr != nil {
			return vErr
		}

		pipelineConfig = pCfg
		req.RepositoryURL = pCfg.Repository.URL

		stratDisplay := "Fail Fast"
		if pCfg.Build.Strategy == planner.BuildStrategyContinue {
			stratDisplay = "Continue"
		}

		pubDisplay := "All Or Nothing"
		if pCfg.Deployment.Publish == planner.PublishPolicySuccessfulOnly {
			pubDisplay = "Successful Only"
		}

		fmt.Fprintf(e.out, "Repository: %s\n", path.Base(strings.TrimRight(pCfg.Repository.URL, "/\\")))
		if len(pCfg.Build.Targets) > 0 {
			fmt.Fprintf(e.out, "Targets:    %s\n", strings.Join(pCfg.Build.Targets, ", "))
		}
		fmt.Fprintf(e.out, "Strategy:   %s\n", stratDisplay)
		fmt.Fprintf(e.out, "Publish:    %s\n\n", pubDisplay)
	}

	shouldCleanup := true
	if req.KeepWorkspace {
		shouldCleanup = false
		cfg.KeepWorkspace = true
		cfg.Workspace.Cleanup = false
	} else if cfg.Workspace.Cleanup {
		shouldCleanup = true
	} else if cfg.KeepWorkspace {
		shouldCleanup = false
	} else {
		shouldCleanup = true
	}

	if req.Username != "" {
		cfg.Username = req.Username
	}

	log := logger.NewLogger(e.out, cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	rev := req.Revision
	if rev == "" {
		rev = "HEAD"
	}

	timestamp := time.Now().Format("20060102-150405")
	jobID := fmt.Sprintf("job-%s-%d", timestamp, time.Now().Nanosecond()%1000)

	job := models.Job{
		ID:                jobID,
		Repository:        req.RepositoryURL,
		Revision:          rev,
		Status:            models.JobStatusPending,
		StartedAt:         time.Now(),
		StageTimings:      make(map[string]time.Duration),
		DeploymentOutputs: make(map[string]string),
		WorkspaceRetained: !shouldCleanup,
	}

	pCtx := pipeline.NewPipelineContext(ctx, req.RepositoryURL, cfg.Username, req.Password)
	pCtx.KeepWorkspace = !shouldCleanup
	pCtx.Revision = rev

	var creds credentials.Provider
	if req.Password != "" || cfg.Username != "" {
		creds = credentials.NewStaticProvider(cfg.Username, req.Password)
	} else {
		creds = credentials.NewEnvironmentProvider("", "")
	}

	retryPolicy := retry.Policy{
		Enabled:        cfg.Retry.Enabled,
		Count:          cfg.Retry.Count,
		BackoffSeconds: cfg.Retry.BackoffSeconds * time.Second,
	}

	// 1. Environment Validation
	fmt.Fprintln(e.out, "Checking environment...")
	fmt.Fprintln(e.out, "")

	startStage := time.Now()
	envVal := e.envValidator
	if envVal == nil {
		envVal = environment.NewValidator(e.processRun, log)
	}

	envInfo, err := envVal.Validate(ctx)
	job.StageTimings["Environment Validation"] = time.Since(startStage)
	if err != nil {
		log.Warn("environment validation warning", "err", err)
	}

	for _, toolKey := range []string{"svn", "node", "npm", "dotnet"} {
		if tInfo, ok := envInfo.Tools[toolKey]; ok && tInfo.Installed {
			fmt.Fprintf(e.out, "✓ %s %s\n\n", tInfo.Name, tInfo.Version)
		} else if tInfo, ok := envInfo.Tools[toolKey]; ok {
			fmt.Fprintf(e.out, "✗ %s (not available: %s)\n\n", tInfo.Name, tInfo.ErrorMessage)
		}
	}

	// 2. Workspace Creation
	fmt.Fprintln(e.out, "Creating workspace...")
	startStage = time.Now()
	wsMgr, err := workspace.NewManager(cfg.WorkspaceRoot, !shouldCleanup, log)
	if err != nil {
		return fmt.Errorf("engine: workspace manager error: %w", err)
	}

	ws, err := wsMgr.CreateWorkspace("build")
	job.StageTimings["Workspace Creation"] = time.Since(startStage)
	if err != nil {
		return fmt.Errorf("engine: failed to create workspace: %w", err)
	}
	job.Workspace = ws.Path
	job.Status = models.JobStatusRunning
	_ = e.buildStore.Save(job)

	defer func() {
		if shouldCleanup {
			fmt.Fprintln(e.out, "\nCleaning up workspace...")
			if err := wsMgr.Cleanup(ws); err != nil {
				log.Warn("workspace cleanup error", "err", err)
				fmt.Fprintf(e.out, "✗ Workspace cleanup warning: %v\n", err)
			} else {
				fmt.Fprintln(e.out, "✓ Workspace removed.")
			}
		} else {
			divider := strings.Repeat("━", 50)
			fmt.Fprintln(e.out, "\n"+divider)
			fmt.Fprintln(e.out, "Workspace Retained")
			fmt.Fprintln(e.out, divider)
			fmt.Fprintln(e.out, "Location")
			fmt.Fprintln(e.out, ws.Path)
			if len(job.BuildResults) > 0 {
				fmt.Fprintln(e.out, "\nGenerated Outputs")
				for _, bRes := range job.BuildResults {
					fmt.Fprintf(e.out, "%s\n", bRes.Project.Name)
					if len(bRes.Artifacts) > 0 {
						for _, art := range bRes.Artifacts {
							relArt, _ := filepath.Rel(ws.Path, art.Path)
							if relArt == "" {
								relArt = art.Path
							}
							fmt.Fprintf(e.out, "  %s/\n", strings.ReplaceAll(relArt, "\\", "/"))
						}
					} else {
						fmt.Fprintln(e.out, "  dist/")
					}
				}
			}
			fmt.Fprintln(e.out, divider+"\n")
		}
	}()

	// 3. Repository Checkout via SCM Provider with Retry
	fmt.Fprintln(e.out, "\nChecking out repository...")
	startStage = time.Now()
	scmProv := e.scmProvider
	if scmProv == nil {
		scmProv = svn.NewProvider(e.processRun, log)
	}

	var repoInfo *repository.Info
	checkoutErr := retry.Execute(ctx, retryPolicy, log, "SVN Checkout", true, func() error {
		info, err := scmProv.CheckoutOrUpdate(ctx, req.RepositoryURL, ws.Path, pCtx.Revision, creds)
		if err != nil {
			return err
		}
		repoInfo = info
		return nil
	})

	job.StageTimings["Repository Checkout"] = time.Since(startStage)

	if checkoutErr != nil {
		if ctx.Err() != nil {
			job.Status = models.JobStatusInterrupted
		} else {
			job.Status = models.JobStatusFailed
		}
		job.FinishedAt = time.Now()
		job.Duration = time.Since(job.StartedAt)
		job.ErrorMessage = checkoutErr.Error()
		_ = e.buildStore.Save(job)
		_ = manifest.Generate(ws.Path, job)
		return fmt.Errorf("engine: checkout error: %w", checkoutErr)
	}
	job.Revision = repoInfo.Revision

	// 3.1 Auto-discover cicd.yaml from workspace root (when no explicit pipeline file was supplied).
	// Priority: explicit file > workspace root cicd.yaml > legacy auto-scan
	if pipelineConfig == nil {
		for _, candidate := range []string{"cicd.yaml", "cicd.yml"} {
			candidatePath := filepath.Join(ws.Path, candidate)
			if info, statErr := os.Stat(candidatePath); statErr == nil && !info.IsDir() {
				fmt.Fprintf(e.out, "\nPipeline file discovered in repository: %s\n\n", candidate)

				loader := planner.NewDefaultLoader()
				pCfg, loadErr := loader.Load(candidatePath)
				if loadErr != nil {
					log.Warn("failed to load discovered pipeline file, falling back to auto-scan", "file", candidatePath, "err", loadErr)
					break
				}

				validator := planner.NewDefaultValidator()
				if valErr := validator.Validate(pCfg); valErr != nil {
					log.Warn("discovered pipeline file failed validation, falling back to auto-scan", "file", candidatePath, "err", valErr)
					break
				}

				// Use the repository URL from YAML if it differs from what was passed
				if pCfg.Repository.URL != "" && pCfg.Repository.URL != req.RepositoryURL {
					log.Info("pipeline file overrides repository URL", "yaml_url", pCfg.Repository.URL)
				}

				pipelineConfig = pCfg

				stratDisplay := "Fail Fast"
				if pCfg.Build.Strategy == planner.BuildStrategyContinue {
					stratDisplay = "Continue"
				}
				pubDisplay := "All Or Nothing"
				if pCfg.Deployment.Publish == planner.PublishPolicySuccessfulOnly {
					pubDisplay = "Successful Only"
				}

				if len(pCfg.Build.Targets) > 0 {
					fmt.Fprintf(e.out, "Targets:    %s\n", strings.Join(pCfg.Build.Targets, ", "))
				}
				fmt.Fprintf(e.out, "Strategy:   %s\n", stratDisplay)
				fmt.Fprintf(e.out, "Publish:    %s\n\n", pubDisplay)
				break
			}
		}

		if pipelineConfig == nil {
			fmt.Fprintln(e.out, "No cicd.yaml found. Using auto-scan mode.")
		}
	}

	wsSanitizer := e.sanitizer
	if wsSanitizer == nil {
		var customRules []sanitizer.Rule
		if len(cfg.Cleanup.Rules) > 0 {
			for _, r := range cfg.Cleanup.Rules {
				customRules = append(customRules, sanitizer.Rule{
					Name:    r,
					Pattern: r,
					Type:    sanitizer.TypeAny,
				})
			}
		}
		wsSanitizer = sanitizer.NewSanitizer(customRules, log)
	}

	// 3.5 Workspace Sanitization
	if cfg.Cleanup.Enabled {
		fmt.Fprintln(e.out, "\nSanitizing workspace...")
		startStage = time.Now()

		sanRes, sanErr := wsSanitizer.Sanitize(ctx, ws.Path)
		job.StageTimings["Workspace Sanitization"] = time.Since(startStage)
		if sanErr != nil {
			log.Warn("workspace sanitization warning", "err", sanErr)
		} else {
			totalRemoved := len(sanRes.DirectoriesRemoved) + len(sanRes.FilesRemoved)
			if totalRemoved > 0 {
				for _, dir := range sanRes.DirectoriesRemoved {
					fmt.Fprintf(e.out, "✓ Removed %s/\n\n", strings.ReplaceAll(dir, "\\", "/"))
				}
				for _, f := range sanRes.FilesRemoved {
					fmt.Fprintf(e.out, "✓ Removed %s\n\n", strings.ReplaceAll(f, "\\", "/"))
				}
				fmt.Fprintln(e.out, "Workspace sanitized.")
			} else {
				fmt.Fprintln(e.out, "Workspace already clean.")
			}
		}
	}

	// 4. Repository Scan & Plan Resolution
	fmt.Fprintln(e.out, "\nScanning repository...")
	startStage = time.Now()
	projScanner := e.projScanner
	if projScanner == nil {
		projScanner = scanner.NewRepositoryScanner(log)
	}

	scannedProjects, err := projScanner.Scan(ctx, ws.Path)
	job.StageTimings["Repository Scan"] = time.Since(startStage)

	if err != nil {
		if ctx.Err() != nil {
			job.Status = models.JobStatusInterrupted
		} else {
			job.Status = models.JobStatusFailed
		}
		job.FinishedAt = time.Now()
		job.Duration = time.Since(job.StartedAt)
		job.ErrorMessage = err.Error()
		_ = e.buildStore.Save(job)
		_ = manifest.Generate(ws.Path, job)
		return fmt.Errorf("engine: repository scan error: %w", err)
	}

	var buildProjects []models.Project
	buildStrategy := planner.BuildStrategyFailFast
	publishPolicy := planner.PublishPolicyAllOrNothing
	replaceExisting := cfg.Deployment.ReplaceExisting
	targetOutputs := make(map[string]string)

	if pipelineConfig != nil {
		validator := planner.NewDefaultValidator()
		execPlan, rErr := validator.Resolve(ctx, pipelineConfig, scannedProjects, ws.Path)
		if rErr != nil {
			job.Status = models.JobStatusFailed
			job.FinishedAt = time.Now()
			job.Duration = time.Since(job.StartedAt)
			job.ErrorMessage = rErr.Error()
			_ = e.buildStore.Save(job)
			_ = manifest.Generate(ws.Path, job)
			fmt.Fprintln(e.out, rErr.Error())
			return rErr
		}

		buildProjects = execPlan.Projects
		buildStrategy = execPlan.BuildStrategy
		publishPolicy = execPlan.PublishPolicy
		replaceExisting = execPlan.ReplaceExisting
		targetOutputs = execPlan.TargetOutputs

		fmt.Fprintf(e.out, "Planner validated %d targets.\n\n", len(buildProjects))
	} else {
		// Legacy URL mode
		repoName := path.Base(strings.TrimRight(req.RepositoryURL, "/\\"))
		wsName := filepath.Base(ws.Path)
		for i := range scannedProjects {
			if scannedProjects[i].Name == "" || scannedProjects[i].Name == wsName || strings.HasPrefix(scannedProjects[i].Name, "build-") {
				if repoName != "" && repoName != "." && repoName != "/" {
					scannedProjects[i].Name = repoName
				}
			}
		}
		buildProjects = scannedProjects
		fmt.Fprintf(e.out, "Found %d project(s).\n\n", len(buildProjects))
	}

	if len(buildProjects) == 0 {
		fmt.Fprintln(e.out, "No supported projects found in repository.")
		job.Status = models.JobStatusSuccess
		job.FinishedAt = time.Now()
		job.Duration = time.Since(job.StartedAt)
		_ = e.buildStore.Save(job)
		_ = manifest.Generate(ws.Path, job)
		return nil
	}

	// 5. Create Build Plan
	startStage = time.Now()
	failFastBool := (buildStrategy == planner.BuildStrategyFailFast)
	plan := builder.NewBuildPlan(buildProjects, cfg.ParallelBuildCount, failFastBool)
	job.StageTimings["Build Plan"] = time.Since(startStage)

	// 5.5 Pre-build script validation: confirm npm scripts exist in package.json
	if svErr := scriptvalidator.ValidateAll(buildProjects); svErr != nil {
		job.Status = models.JobStatusFailed
		job.FinishedAt = time.Now()
		job.Duration = time.Since(job.StartedAt)
		job.ErrorMessage = svErr.Error()
		_ = e.buildStore.Save(job)
		_ = manifest.Generate(ws.Path, job)
		fmt.Fprintln(e.out, svErr.Error())
		return svErr
	}

	bReg := e.bRegistry
	if bReg == nil {
		bReg = builder.NewDefaultRegistry(e.processRun, log)
	}

	bRep := e.bReporter
	if bRep == nil {
		bRep = reporter.NewConsoleReporter(e.out)
	}

	fmt.Fprintln(e.out, "Executing build plan...")

	// 6. Execute Build Plan according to BuildStrategy
	startStage = time.Now()
	var buildResults []models.BuildResult
	var allArtifacts []models.Artifact
	hasFailure := false

	for i, project := range plan.Projects {
		if ctx.Err() != nil {
			hasFailure = true
			break
		}

		bld, err := bReg.BuilderFor(project)
		if err != nil {
			log.Error("failed to find builder", "project", project.Name, "err", err)
			bRes := models.BuildResult{
				Project:      project,
				Status:       models.BuildStatusFailed,
				ErrorMessage: err.Error(),
			}
			buildResults = append(buildResults, bRes)
			hasFailure = true
			bRep.ReportFailure(project.Name, "Builder Discovery", "BuilderFor", 1, 0, err.Error())
			if plan.FailFast {
				fmt.Fprintln(e.out, "Stopping execution.\nReason: Fail Fast policy.")
				// Mark remaining targets as NOT RUN
				for j := i + 1; j < len(plan.Projects); j++ {
					buildResults = append(buildResults, models.BuildResult{
						Project:      plan.Projects[j],
						Status:       models.BuildStatusFailed,
						ErrorMessage: "NOT RUN (skipped due to fail-fast policy)",
					})
				}
				break
			}
			continue
		}

		bRes, buildErr := bld.Build(ctx, project, bRep)

		for _, step := range bRes.Steps {
			_ = e.logStore.Append(job.ID, logs.LogEntry{
				Time:    time.Now(),
				JobID:   job.ID,
				Project: project.Name,
				Step:    step.Name,
				Level:   "INFO",
				Message: fmt.Sprintf("Step %s execution exit code %d", step.Name, step.ExitCode),
			})
		}

		allArtifacts = append(allArtifacts, bRes.Artifacts...)
		buildResults = append(buildResults, bRes)

		if buildErr != nil || bRes.Status == models.BuildStatusFailed {
			hasFailure = true
			if bRes.ErrorMessage == "" && buildErr != nil {
				bRes.ErrorMessage = buildErr.Error()
			}

			// Find the actual failed step to show real name, command, exit code, and output
			failedStepName := "Build"
			failedCommand := "compile project"
			failedExitCode := 1
			failedDuration := bRes.Duration
			var failedOutput strings.Builder

			for _, step := range bRes.Steps {
				if !step.Successful {
					failedStepName = step.Name
					failedCommand = step.Command
					failedExitCode = step.ExitCode
					failedDuration = step.Duration

					// Combine stdout + stderr for maximum diagnostics
					if step.StdOut != "" {
						failedOutput.WriteString(step.StdOut)
					}
					if step.StdErr != "" {
						if failedOutput.Len() > 0 {
							failedOutput.WriteString("\n")
						}
						failedOutput.WriteString(step.StdErr)
					}
					// Fallback to error message if no captured output
					if failedOutput.Len() == 0 && step.ErrorMessage != "" {
						failedOutput.WriteString(step.ErrorMessage)
					}
					break
				}
			}

			// Fallback: if no step-level output found, use build-level error message
			if failedOutput.Len() == 0 && bRes.ErrorMessage != "" {
				failedOutput.WriteString(bRes.ErrorMessage)
			}

			bRep.ReportFailure(project.Name, failedStepName, failedCommand, failedExitCode, failedDuration, failedOutput.String())

			if plan.FailFast {
				fmt.Fprintln(e.out, "Stopping execution.\nReason: Fail Fast policy.")
				for j := i + 1; j < len(plan.Projects); j++ {
					buildResults = append(buildResults, models.BuildResult{
						Project:      plan.Projects[j],
						Status:       models.BuildStatusFailed,
						ErrorMessage: "NOT RUN (skipped due to fail-fast policy)",
					})
				}
				break
			} else {
				fmt.Fprintln(e.out, "Continuing because build strategy is Continue.")
			}
		}
	}

	job.StageTimings["Build"] = time.Since(startStage)

	// 6.5 Publish Deployable Output according to PublishPolicy
	shouldPublish := true
	if publishPolicy == planner.PublishPolicyAllOrNothing && hasFailure {
		shouldPublish = false
		fmt.Fprintln(e.out, "Skipping deployable output publishing because build policy is All Or Nothing and build failures occurred.")
	}

	repoName := extractRepoName(req.RepositoryURL)

	if shouldPublish && len(buildResults) > 0 {
		fmt.Fprintln(e.out, "Publishing Deployable Output...")
		fmt.Fprintln(e.out, "")

		startStage = time.Now()
		pub := e.bPublisher
		if pub == nil {
			pub = publisher.NewDefaultPublisher(log)
		}

		defaultDeployRoot := cfg.Deployment.Directory
		if defaultDeployRoot == "" {
			defaultDeployRoot = filepath.Join(".", ".deployments")
		}

		pubSuccess := true
		for _, bRes := range buildResults {
			if bRes.Status == models.BuildStatusSuccess && len(bRes.Artifacts) > 0 {
				targetDir := targetOutputs[bRes.Project.Name]
				if targetDir == "" {
					targetDir = defaultDeployRoot
				}

				pubDir, pubErr := pub.Publish(ctx, repoName, bRes.Project, bRes, targetDir, replaceExisting)
				if pubErr != nil {
					log.Error("failed to publish deployable output", "project", bRes.Project.Name, "err", pubErr)
					pubSuccess = false
					hasFailure = true
					job.ErrorMessage = pubErr.Error()
					bRep.ReportFailure(bRes.Project.Name, "Publish Deployable Output", "copy build artifacts", 1, time.Since(startStage), pubErr.Error())
					break
				}
				job.DeploymentOutputs[bRes.Project.Name] = pubDir

				artRel := bRes.Artifacts[0].Name
				fmt.Fprintf(e.out, "✓ %s\n", bRes.Project.Name)
				fmt.Fprintf(e.out, "  %s/\n", artRel)
				fmt.Fprintln(e.out, "  ↓")
				fmt.Fprintf(e.out, "  %s\n\n", pubDir)
			}
		}

		job.StageTimings["Publish Deployable Output"] = time.Since(startStage)

		if pubSuccess {
			fmt.Fprintln(e.out, "Deployment output published successfully.")
		}

		// Post-publish workspace sanitization to save server disk space
		if wsSanitizer != nil {
			_, _ = wsSanitizer.Sanitize(ctx, ws.Path)
		}
	}

	// 7. Summary, History & Manifest Generation
	job.FinishedAt = time.Now()
	job.Duration = time.Since(job.StartedAt)
	job.BuildResults = buildResults
	job.Artifacts = allArtifacts

	if ctx.Err() != nil {
		job.Status = models.JobStatusInterrupted
		job.ErrorMessage = "job execution interrupted by user or system signal"
	} else if hasFailure {
		job.Status = models.JobStatusFailed
	} else {
		job.Status = models.JobStatusSuccess
	}

	_ = e.buildStore.Save(job)
	_ = manifest.Generate(ws.Path, job)

	bRep.ReportSummary(job)

	if job.Status == models.JobStatusInterrupted {
		return fmt.Errorf("engine: build execution interrupted")
	}

	if hasFailure {
		return fmt.Errorf("engine: one or more project builds failed")
	}

	fmt.Fprintln(e.out, "Build completed successfully.")
	return nil
}

func extractRepoName(rawURL string) string {
	cleaned := strings.TrimRight(rawURL, "/\\")
	parts := strings.Split(cleaned, "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if len(parts) >= 2 {
		prev := parts[len(parts)-2]
		prevLower := strings.ToLower(prev)
		if prevLower != "svn" && prevLower != "trunk" && prevLower != "branches" && !strings.Contains(prev, ":") && prevLower != "http:" && prevLower != "https:" {
			return prev
		}
	}
	return last
}

// GetHistory retrieves all completed jobs from build store.
func (e *Engine) GetHistory() ([]models.Job, error) {
	return e.buildStore.List()
}

// GetStatus retrieves all queued or active jobs.
func (e *Engine) GetStatus() ([]models.Job, error) {
	return e.jobQueue.List(), nil
}
