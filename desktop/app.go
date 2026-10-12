package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/engine"
	"github.com/anandhuremanan/cicd-engine/internal/environment"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/netshare"
	"github.com/anandhuremanan/cicd-engine/internal/process"
	"github.com/anandhuremanan/cicd-engine/internal/publisher"
	"github.com/anandhuremanan/cicd-engine/internal/repository"
	"github.com/anandhuremanan/cicd-engine/internal/scanner"
	"github.com/anandhuremanan/cicd-engine/internal/scriptvalidator"
	"github.com/anandhuremanan/cicd-engine/internal/storage/logs"
)

const appDirName = "SvnBuilder"

// Events emitted to the frontend while a build runs.
const (
	eventStage = "build:stage"
	eventLog   = "build:log"
	eventDone  = "build:done"
)

// Build stages shown to the user, in order.
const (
	stageTools    = "tools"
	stageDownload = "download"
	stageBuild    = "build"
	stageSave     = "save"
)

// Settings holds the form values remembered between sessions. The password is never stored.
type Settings struct {
	RepositoryURL string          `json:"repositoryUrl"`
	Username      string          `json:"username"`
	OutputDir     string          `json:"outputDir"`
	ShareUsername string          `json:"shareUsername"`
	Projects      []ProjectTarget `json:"projects"`
	// StorageDir is the working folder chosen in Settings for downloads, builds and history.
	// Empty means automatic: the Windows temp folder and the user settings folder.
	StorageDir string `json:"storageDir"`
	// Theme is "light", "dark" or "system" (follow Windows). Empty means "system".
	Theme string `json:"theme"`
	// PackageKeepDays is how many days saved packages are kept after a project's last build.
	// Zero means not chosen yet (the default applies); -1 means until cleared by hand.
	PackageKeepDays int `json:"packageKeepDays"`
}

// ProjectTarget is one row of the "Folders to build" list.
type ProjectTarget struct {
	// Folder is a folder inside the repository, e.g. "Client".
	Folder string `json:"folder"`
	// Script is the npm script for a website project in that folder. Empty means "build".
	Script string `json:"script"`
}

// BuildRequest is the form submitted from the UI.
type BuildRequest struct {
	RepositoryURL string `json:"repositoryUrl"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	OutputDir     string `json:"outputDir"`
	Revision      string `json:"revision"`
	// ShareUsername and SharePassword sign in to a network output folder that needs its own account.
	// Both empty means the folder is used with the account the user is logged on to Windows with.
	ShareUsername string `json:"shareUsername"`
	SharePassword string `json:"sharePassword"`
	// Projects lists the repository folders to build, in build order. Empty builds every project found.
	Projects []ProjectTarget `json:"projects"`
	// CleanBuild throws away the packages kept from earlier builds and installs everything again.
	CleanBuild bool `json:"cleanBuild"`
}

// StartResult reports whether a build was started.
type StartResult struct {
	Started bool `json:"started"`
	// NeedsSignIn is set when the output folder is a network folder that asked for an account.
	NeedsSignIn bool `json:"needsSignIn"`
	// Message explains why the build did not start.
	Message string `json:"message"`
}

// StageUpdate tells the UI which stage is active and what it is doing.
type StageUpdate struct {
	Stage  string `json:"stage"`
	Detail string `json:"detail"`
}

// ProjectOutcome summarises one project of a finished build.
type ProjectOutcome struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Success bool   `json:"success"`
}

// BuildOutcome is the final result sent to the UI.
type BuildOutcome struct {
	// Status is "success", "failed", "cancelled" or "nothing".
	Status      string           `json:"status"`
	BuildNumber int              `json:"buildNumber"`
	BuildName   string           `json:"buildName"`
	BuildDir    string           `json:"buildDir"`
	Revision    string           `json:"revision"`
	FailedStage string           `json:"failedStage"`
	Message     string           `json:"message"`
	Details     string           `json:"details"`
	Projects    []ProjectOutcome `json:"projects"`
}

// ToolStatus reports whether a build tool is installed on this PC.
type ToolStatus struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
}

// App is the Wails-bound backend of the desktop build tool.
type App struct {
	ctx context.Context
	// configDir is the per-user settings folder (%APPDATA%\SvnBuilder).
	configDir string

	mu       sync.Mutex
	settings Settings
	engine   *engine.Engine
	// workspaceDir is where projects are downloaded and built; dataDir holds the build history and logs.
	workspaceDir string
	dataDir      string
	running      bool
	cancel       context.CancelFunc
	// done is closed when the running build has stopped.
	done chan struct{}
	// cleanup tracks temporary downloads being deleted in the background.
	cleanup sync.WaitGroup
	stage   string
}

// NewApp loads the saved settings and wires the build engine to the working folder they name.
func NewApp() (*App, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot locate user settings folder: %w", err)
	}

	app := &App{configDir: filepath.Join(configDir, appDirName)}
	app.settings = app.loadSettings()
	return app, nil
}

// instanceID identifies this app for the single-instance lock. It includes the settings folder,
// so copies pointed at different settings folders do not block each other.
func (a *App) instanceID() string {
	sum := sha256.Sum256([]byte(strings.ToLower(a.configDir)))
	return "svn-builder-" + hex.EncodeToString(sum[:8])
}

// secondInstance runs in the first copy when the app is started again.
func (a *App) secondInstance(data options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// startup creates the build engine once the window exists. Doing it here and not in NewApp
// matters: `wails build` runs the program briefly to generate bindings, and creating an engine
// marks unfinished build records as interrupted, which must not happen to a build that another
// open copy of the app is still running.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.configureStorage(a.settings.StorageDir); err != nil {
		// The chosen drive may be unplugged or gone; fall back to the automatic locations.
		a.settings.StorageDir = ""
		if err := a.configureStorage(""); err != nil {
			runtime.LogErrorf(ctx, "cannot prepare the working folder: %v", err)
		}
	}

	// Delete temporary downloads that an earlier session left behind, in the working folder in
	// use now and in the automatic one, which may hold leftovers from before a drive was chosen.
	roots := []string{a.workspaceDir}
	if auto := automaticWorkspaceDir(); auto != a.workspaceDir {
		roots = append(roots, auto)
	}
	a.cleanup.Add(1)
	go func() {
		defer a.cleanup.Done()
		for _, root := range roots {
			purgeWorkspaces(root)
		}
	}()

	// Drop saved packages of projects that have not been built for a while.
	a.prunePackages()
}

// SetTheme remembers the colour theme and makes the window frame match it.
func (a *App) SetTheme(theme string) error {
	if theme != "light" && theme != "dark" {
		theme = "system"
	}
	a.applyWindowTheme(theme)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.Theme = theme
	return a.saveSettings()
}

// applyWindowTheme switches the title bar; the page switches its own colours.
func (a *App) applyWindowTheme(theme string) {
	if a.ctx == nil {
		return
	}
	switch theme {
	case "light":
		runtime.WindowSetLightTheme(a.ctx)
	case "dark":
		runtime.WindowSetDarkTheme(a.ctx)
	default:
		runtime.WindowSetSystemDefaultTheme(a.ctx)
	}
}

// domReady applies the saved theme and shrinks the window on screens too short for its fixed height.
func (a *App) domReady(ctx context.Context) {
	a.mu.Lock()
	theme := a.settings.Theme
	a.mu.Unlock()
	a.applyWindowTheme(theme)

	screens, err := runtime.ScreenGetAll(ctx)
	if err != nil {
		return
	}
	for _, screen := range screens {
		// Leave room for the taskbar.
		if available := screen.Size.Height - 72; screen.IsCurrent && available > 0 && available < windowHeight {
			runtime.WindowSetSize(ctx, windowWidth, available)
			runtime.WindowCenter(ctx)
		}
	}
}

// beforeClose stops a running build so no tool keeps running after the window is gone.
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()

	a.CancelBuild()

	// Give the build a moment to stop its tools and the temporary download a moment to be removed
	// before the process exits. Whatever is not deleted in time is finished on the next start.
	settled := make(chan struct{})
	go func() {
		if done != nil {
			<-done
		}
		a.cleanup.Wait()
		close(settled)
	}()
	select {
	case <-settled:
	case <-time.After(10 * time.Second):
	}
	return false
}

func (a *App) emit(event string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data)
	}
}

func (a *App) setStage(stage, detail string) {
	a.mu.Lock()
	a.stage = stage
	a.mu.Unlock()
	a.emit(eventStage, StageUpdate{Stage: stage, Detail: detail})
}

// LoadSettings returns the values used for the previous build.
func (a *App) LoadSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// ChooseFolder opens the folder picker and returns the chosen path ("" when cancelled).
func (a *App) ChooseFolder(current, title string) (string, error) {
	opts := runtime.OpenDialogOptions{Title: title}
	if info, err := os.Stat(current); err == nil && info.IsDir() {
		opts.DefaultDirectory = current
	}
	return runtime.OpenDirectoryDialog(a.ctx, opts)
}

// OpenFolder shows a folder in File Explorer.
func (a *App) OpenFolder(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("The folder %s can't be opened. Check that the server is reachable.", path)
	}
	return exec.Command("explorer.exe", filepath.Clean(path)).Start()
}

// CheckTools reports which build tools are installed on this PC.
func (a *App) CheckTools() []ToolStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, _ := environment.NewValidator(process.NewNativeRunner(), nil).Validate(ctx)

	names := map[string]string{"svn": "Subversion (SVN)", "node": "Node.js", "npm": "npm", "dotnet": ".NET SDK"}
	var tools []ToolStatus
	for _, key := range []string{"svn", "node", "npm", "dotnet"} {
		t := info.Tools[key]
		tools = append(tools, ToolStatus{Key: key, Name: names[key], Installed: t.Installed, Version: t.Version})
	}
	return tools
}

// RemoteFolder is a buildable folder found in the SVN repository.
type RemoteFolder struct {
	// Folder is the path inside the repository; "." is the top folder.
	Folder string `json:"folder"`
	// Type is "react", "next" or "dotnet".
	Type string `json:"type"`
}

// maxRemoteFileReads caps how many package.json files are fetched while looking for projects.
const maxRemoteFileReads = 40

// FetchFolders asks the SVN server which folders of the project can be built, without
// downloading the project. Only the address, credentials and revision of req are used.
func (a *App) FetchFolders(req BuildRequest) ([]RemoteFolder, error) {
	repoURL := strings.TrimRight(strings.TrimSpace(req.RepositoryURL), "/")
	revision := strings.TrimPrefix(strings.TrimSpace(req.Revision), "r")

	if repoURL == "" {
		return nil, fmt.Errorf("Enter the SVN address of the project first.")
	}
	if !strings.Contains(repoURL, "://") {
		return nil, fmt.Errorf("The SVN address should look like https://server/repos/Project.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	svn := repository.NewSVNService(process.NewNativeRunner(), nil)
	creds := credentials.NewStaticProvider(strings.TrimSpace(req.Username), req.Password)

	paths, err := svn.ListFiles(ctx, repoURL, revision, creds)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("The SVN server took too long to answer. Type the folder names instead.")
		}
		return nil, fmt.Errorf("%s", friendlyFailure(err.Error(), stageDownload, nil))
	}

	reads := 0
	projects := scanner.DetectRemote(paths, func(filePath string) ([]byte, error) {
		if reads++; reads > maxRemoteFileReads {
			return nil, fmt.Errorf("too many package.json files to inspect")
		}
		return svn.ReadFile(ctx, repoURL+"/"+filePath, revision, creds)
	})

	folders := []RemoteFolder{}
	for _, p := range projects {
		folder := p.Path
		if folder == "" {
			folder = "."
		}
		folders = append(folders, RemoteFolder{Folder: folder, Type: string(p.Type)})
	}
	return folders, nil
}

// StartBuild validates the form and starts the build in the background.
// Progress arrives through the build:stage, build:log and build:done events.
func (a *App) StartBuild(req BuildRequest) StartResult {
	refuse := func(format string, args ...interface{}) StartResult {
		return StartResult{Message: fmt.Sprintf(format, args...)}
	}

	req.RepositoryURL = strings.TrimSpace(req.RepositoryURL)
	req.Username = strings.TrimSpace(req.Username)
	req.OutputDir = strings.TrimSpace(req.OutputDir)
	req.ShareUsername = strings.TrimSpace(req.ShareUsername)
	req.Revision = strings.TrimPrefix(strings.TrimSpace(req.Revision), "r")

	// Rows without a folder are ignored; a script on its own has nothing to apply to.
	projects := []ProjectTarget{}
	for _, p := range req.Projects {
		p.Folder = strings.TrimSpace(p.Folder)
		p.Script = strings.TrimSpace(p.Script)
		if p.Folder != "" {
			projects = append(projects, p)
		}
	}
	req.Projects = projects

	if req.RepositoryURL == "" {
		return refuse("Enter the SVN address of the project.")
	}
	if !strings.Contains(req.RepositoryURL, "://") {
		return refuse("The SVN address should look like https://server/repos/Project.")
	}
	if req.OutputDir == "" {
		return refuse("Choose the folder where the build should be saved.")
	}
	if result := prepareOutputDir(req); result.Message != "" {
		return result
	}

	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return refuse("A build is already running.")
	}
	if a.engine == nil {
		a.mu.Unlock()
		return refuse("The working folder can't be used. Open Settings (the gear at the top) and choose another drive.")
	}
	workspaceDir := a.workspaceDir
	if free, _, ok := diskSpace(workspaceDir); ok && free < minWorkspaceBytes {
		a.mu.Unlock()
		return refuse("Drive %s has only %s free, which is not enough to download and build a project. Open Settings (the gear at the top) and choose another drive.", driveName(workspaceDir), formatBytes(free))
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.running = true
	a.cancel = cancel
	a.done = make(chan struct{})
	a.stage = ""

	a.settings.RepositoryURL = req.RepositoryURL
	a.settings.Username = req.Username
	a.settings.OutputDir = req.OutputDir
	a.settings.ShareUsername = req.ShareUsername
	a.settings.Projects = req.Projects
	_ = a.saveSettings()
	a.mu.Unlock()

	go a.runBuild(ctx, req, workspaceDir)
	return StartResult{Started: true}
}

// prepareOutputDir signs in to a network output folder when an account was given, then checks
// that the folder can be written to, so a problem shows up now and not after a long build.
// A result with an empty Message means the folder is ready.
func prepareOutputDir(req BuildRequest) StartResult {
	_, isNetwork := netshare.ShareRoot(req.OutputDir)
	hasAccount := req.ShareUsername != ""

	if hasAccount && !isNetwork {
		return StartResult{Message: "A separate sign-in only works for network folders such as \\\\SERVER\\Builds. Turn it off or choose a network folder."}
	}

	var conflict bool
	if hasAccount {
		err := netshare.Connect(req.OutputDir, req.ShareUsername, req.SharePassword)

		// Windows allows one account per server at a time. A connection it made earlier with the
		// Windows login (for example by opening the folder in File Explorer) blocks the sign-in;
		// when that connection cannot save here anyway, drop it and sign in again.
		if errors.Is(err, netshare.ErrConflict) && writeProbe(req.OutputDir) != nil {
			netshare.Disconnect(req.OutputDir)
			err = netshare.Connect(req.OutputDir, req.ShareUsername, req.SharePassword)
		}

		switch {
		case err == nil:
		case errors.Is(err, netshare.ErrLogonFailed):
			return StartResult{NeedsSignIn: true, Message: "The server did not accept that user name or password for the folder."}
		case errors.Is(err, netshare.ErrNotFound):
			return StartResult{Message: fmt.Sprintf("The folder %s can't be found. Check the server name, the folder name and your network or VPN connection.", req.OutputDir)}
		case errors.Is(err, netshare.ErrAccessDenied):
			return StartResult{NeedsSignIn: true, Message: "That account is not allowed to use the folder. Ask the server's administrator for access."}
		case errors.Is(err, netshare.ErrConflict):
			// Windows already has a connection to this server; it is fine if that one can write.
			conflict = true
		default:
			return StartResult{Message: fmt.Sprintf("The folder %s can't be reached: %v", req.OutputDir, err)}
		}
	}

	err := writeProbe(req.OutputDir)
	if err == nil {
		return StartResult{}
	}

	switch {
	case conflict:
		return StartResult{NeedsSignIn: true, Message: "Windows is connected to this server with a different account through something else, such as a mapped drive, and that account can't save here. Disconnect that drive or restart the PC, then try again."}
	case isNetwork && isAccessError(err) && hasAccount:
		return StartResult{NeedsSignIn: true, Message: "That account can open the folder but is not allowed to save files in it."}
	case isNetwork && isAccessError(err):
		return StartResult{NeedsSignIn: true, Message: "This folder needs a sign-in. Enter the user name and password for the server."}
	case isAccessError(err):
		return StartResult{Message: fmt.Sprintf("You don't have permission to save files in %s.", req.OutputDir)}
	}
	return StartResult{Message: fmt.Sprintf("The folder %s can't be reached. Check the path and your network or VPN connection.", req.OutputDir)}
}

// writeProbe checks that dir exists, or can be created, and that files can be added to it.
func writeProbe(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	probe, err := os.MkdirTemp(dir, ".svnbuilder-check-")
	if err != nil {
		return err
	}
	return os.Remove(probe)
}

// isAccessError reports whether err means "not signed in" or "not allowed" as opposed to "not found".
func isAccessError(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case 5, 86, 1219, 1326, 1327, 1330, 1331, 1385, 1907, 1909, 2202:
			return true
		}
	}
	return false
}

// CancelBuild stops the running build, if any.
func (a *App) CancelBuild() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) runBuild(ctx context.Context, req BuildRequest, workspaceDir string) {
	var job models.Job

	var folders []string
	scripts := make(map[string]string)
	for _, p := range req.Projects {
		folders = append(folders, p.Folder)
		scripts[p.Folder] = p.Script
	}

	runErr := a.engine.Run(ctx, cli.RunRequest{
		RepositoryURL:  req.RepositoryURL,
		Username:       req.Username,
		Password:       req.Password,
		Revision:       req.Revision,
		OutputDir:      req.OutputDir,
		WorkspaceRoot:  workspaceDir,
		CleanWorkspace: true,
		DeferCleanup:   true,
		// Keeping installed packages between builds turns a minutes-long install into seconds.
		ReuseDependencies: true,
		CleanDependencies: req.CleanBuild,
		ProjectPaths:      folders,
		ProjectScripts:    scripts,
		OnFinish:          func(j models.Job) { job = j },
	})

	cancelled := ctx.Err() != nil

	a.mu.Lock()
	a.running = false
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.done != nil {
		close(a.done)
		a.done = nil
	}
	failedStage := a.stage
	a.mu.Unlock()

	// Show the result first. The temporary download can hold tens of thousands of dependency
	// files and take minutes to delete, which nobody should have to wait for.
	a.emit(eventDone, buildOutcome(job, runErr, cancelled, failedStage))

	if job.Workspace != "" {
		a.cleanup.Add(1)
		go func() {
			defer a.cleanup.Done()
			discardWorkspace(workspaceDir, job.Workspace)
		}()
	}

	a.mu.Lock()
	a.prunePackages()
	a.mu.Unlock()
}

func buildOutcome(job models.Job, runErr error, cancelled bool, failedStage string) BuildOutcome {
	outcome := BuildOutcome{
		Revision: job.Revision,
		Projects: []ProjectOutcome{},
	}

	var failed []string
	for _, bRes := range job.BuildResults {
		ok := bRes.Status == models.BuildStatusSuccess
		if !ok && !strings.HasPrefix(bRes.ErrorMessage, "NOT RUN") {
			failed = append(failed, bRes.Project.Name)
		}
		outcome.Projects = append(outcome.Projects, ProjectOutcome{
			Name:    bRes.Project.Name,
			Type:    string(bRes.Project.Type),
			Success: ok,
		})
	}

	switch {
	case cancelled:
		outcome.Status = "cancelled"
		outcome.Message = "The build was stopped. Nothing was saved to the shared folder."

	case runErr != nil:
		outcome.Status = "failed"
		outcome.FailedStage = failedStage
		outcome.Details = runErr.Error()
		if job.ErrorMessage != "" {
			outcome.Details = job.ErrorMessage
		}
		outcome.Message = friendlyFailure(outcome.Details, failedStage, failed)

		var filterErr *engine.ProjectFilterError
		var scriptErr *scriptvalidator.ValidationError
		if errors.As(runErr, &filterErr) {
			outcome.Message = filterErr.Error() + " Check the \"Folders to build\" list."
		} else if errors.As(runErr, &scriptErr) {
			outcome.Message = fmt.Sprintf("%s has no build script called %q. Scripts it does have: %s. Check the build script next to its folder.",
				scriptErr.Project, scriptErr.ConfiguredScript, strings.Join(scriptErr.AvailableScripts, ", "))
		}

	case job.BuildNumber > 0:
		outcome.Status = "success"
		outcome.BuildNumber = job.BuildNumber
		outcome.BuildName = publisher.BuildDirName(job.BuildNumber)
		outcome.BuildDir = job.BuildDir

	default:
		outcome.Status = "nothing"
		outcome.Message = "No React, Next.js or .NET project was found at this SVN address, so there was nothing to build."
		if len(job.BuildResults) > 0 {
			outcome.Message = "The projects were built but produced no output files, so nothing was saved."
		}
	}

	return outcome
}

// friendlyFailure turns an engine error into a sentence a non-technical user can act on.
func friendlyFailure(details, stage string, failedProjects []string) string {
	lower := strings.ToLower(details)

	switch {
	case strings.Contains(lower, "executable file not found") && strings.Contains(lower, "svn"):
		return "Subversion (SVN) is not installed on this PC. Ask IT to install the SVN command-line tools."
	case strings.Contains(lower, "e170001") || strings.Contains(lower, "e215004") ||
		strings.Contains(lower, "authentication failed") || strings.Contains(lower, "authorization failed"):
		return "SVN did not accept the user name or password. Check them and try again."
	case strings.Contains(lower, "e170013") || strings.Contains(lower, "e731001") || strings.Contains(lower, "e670002") ||
		strings.Contains(lower, "unable to connect"):
		return "The SVN server can't be reached. Check the address and your network or VPN connection."
	case strings.Contains(lower, "e170000") || strings.Contains(lower, "e200009") || strings.Contains(lower, "doesn't exist"):
		return "Nothing was found at that SVN address. Check the address for typing mistakes."
	}

	switch stage {
	case stageDownload:
		return "The project could not be downloaded from SVN."
	case stageSave:
		return "The build worked, but it could not be saved to the shared folder. Check that you can open the folder and have permission to add files to it."
	case stageBuild:
		if len(failedProjects) > 0 {
			return fmt.Sprintf("%s could not be built. The code in SVN has an error that a developer needs to fix.", strings.Join(failedProjects, ", "))
		}
		return "The project could not be built."
	}
	return "The build could not be completed."
}

// stageLogStore forwards engine log entries to the UI as stage updates.
type stageLogStore struct {
	inner logs.LogStore
	app   *App
}

func (s *stageLogStore) Append(jobID string, entry logs.LogEntry) error {
	if stage, detail := stageFor(entry); stage != "" {
		s.app.setStage(stage, detail)
	}
	return s.inner.Append(jobID, entry)
}

func (s *stageLogStore) Read(jobID string) ([]logs.LogEntry, error) {
	return s.inner.Read(jobID)
}

func stageFor(entry logs.LogEntry) (string, string) {
	switch entry.Step {
	case "Environment Validation":
		return stageTools, ""
	case "Workspace Creation":
		return stageDownload, ""
	case "Repository Checkout":
		// Progress entries read "Downloaded 1240 files"; the first entry names the address instead.
		if strings.HasPrefix(entry.Message, "Downloaded ") {
			return stageDownload, entry.Message
		}
		return stageDownload, "Connecting to the SVN server"
	case "Publish Deployable Output":
		return stageSave, ""
	case "End Event":
		return "", ""
	case "Repository Scan":
		return stageBuild, "Looking for projects"
	}
	if entry.Project != "" {
		if entry.Step != "" {
			// Progress summaries such as "Downloaded 312 packages" are worth showing as they change.
			if strings.HasPrefix(entry.Message, "Downloaded ") {
				return stageBuild, entry.Project + " · " + entry.Step + " · " + entry.Message
			}
			return stageBuild, entry.Project + " · " + entry.Step
		}
		return stageBuild, entry.Project
	}
	if entry.Step == "Build" {
		return stageBuild, ""
	}
	return "", ""
}

// lineWriter splits the engine's console output into lines for the details view.
type lineWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	emit func(line string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	for {
		idx := bytes.IndexByte(w.buf.Bytes(), '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf.Next(idx+1)), "\r\n")
		w.emit(line)
	}
	return len(p), nil
}
