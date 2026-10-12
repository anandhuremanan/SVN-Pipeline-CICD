package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/credentials"
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// Info contains metadata about a checked-out repository workspace.
type Info struct {
	// URL is the SVN repository remote URL.
	URL string `json:"url"`
	// Revision is the checked-out SVN revision.
	Revision string `json:"revision"`
	// TargetDir is the local absolute workspace path.
	TargetDir string `json:"target_dir"`
	// CheckedOutAt records when checkout or update completed.
	CheckedOutAt time.Time `json:"checked_out_at"`
}

// Service defines operations for repository checkout, update, and revision queries.
type Service interface {
	// CheckoutOrUpdate checks out or updates the working copy at targetDir.
	CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*Info, error)
	// GetRevision queries the current SVN revision of an existing workspace.
	GetRevision(ctx context.Context, targetDir string) (string, error)
}

// SVNService implements Service using native SVN process execution.
type SVNService struct {
	runner process.Runner
	logger logger.Logger
	// onProgress, when set, receives the number of files downloaded so far during a checkout or update.
	onProgress func(files int)
}

// SetProgress registers a callback that is told how many files a running checkout or update has
// downloaded. It is called at most once a second, and once more when the download ends.
func (s *SVNService) SetProgress(fn func(files int)) {
	s.onProgress = fn
}

// progressCounter counts the files svn reports while it downloads ("A    path", "U    path").
type progressCounter struct {
	report   func(files int)
	files    int
	lastSent time.Time
}

func (p *progressCounter) Stdout(line string) {
	if len(line) < 2 || line[1] != ' ' || !strings.ContainsRune("AUDGR", rune(line[0])) {
		return
	}
	p.files++
	if time.Since(p.lastSent) >= time.Second {
		p.lastSent = time.Now()
		p.report(p.files)
	}
}

func (p *progressCounter) Stderr(line string) {}

// progressHandlers returns the stream handler for a download, or none when nobody is listening.
func (s *SVNService) progressHandlers() (*progressCounter, []process.StreamHandler) {
	if s.onProgress == nil {
		return nil, nil
	}
	counter := &progressCounter{report: s.onProgress, lastSent: time.Now()}
	return counter, []process.StreamHandler{counter}
}

// NewSVNService creates a new SVNService with injected dependencies.
func NewSVNService(runner process.Runner, log logger.Logger) *SVNService {
	if log == nil {
		log = logger.NopLogger()
	}
	return &SVNService{
		runner: runner,
		logger: log,
	}
}

// CheckoutOrUpdate checks out the repository or updates an existing checkout directory.
func (s *SVNService) CheckoutOrUpdate(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) (*Info, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repository service: repoURL cannot be empty")
	}
	if targetDir == "" {
		return nil, fmt.Errorf("repository service: targetDir cannot be empty")
	}
	if revision == "" {
		revision = "HEAD"
	}

	svnDir := filepath.Join(targetDir, ".svn")
	_, err := os.Stat(svnDir)
	isWorkingCopy := err == nil

	var actionErr error
	if isWorkingCopy {
		s.logger.Info("updating existing SVN working copy", "url", repoURL, "dir", targetDir)
		actionErr = s.doUpdate(ctx, targetDir, revision, creds)
	} else {
		s.logger.Info("checking out SVN repository", "url", repoURL, "dir", targetDir)
		actionErr = s.doCheckout(ctx, repoURL, targetDir, revision, creds)
	}

	if actionErr != nil {
		return nil, actionErr
	}

	currentRev, err := s.GetRevision(ctx, targetDir)
	if err != nil {
		currentRev = revision
	}

	return &Info{
		URL:          repoURL,
		Revision:     currentRev,
		TargetDir:    targetDir,
		CheckedOutAt: time.Now(),
	}, nil
}

func (s *SVNService) doCheckout(ctx context.Context, repoURL, targetDir, revision string, creds credentials.Provider) error {
	args := []string{"checkout", repoURL, targetDir, "--non-interactive", "--no-auth-cache", "--trust-server-cert"}

	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}

	args, pass := s.appendCredArgs(args, creds)

	opts := models.ExecOptions{
		Command: "svn",
		Args:    args,
	}

	counter, handlers := s.progressHandlers()
	res, err := s.runner.Run(ctx, opts, handlers...)
	if err != nil || !res.Successful {
		errMsg := strings.TrimSpace(res.StdErr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(res.StdOut)
		}
		if errMsg == "" {
			errMsg = res.ErrorMessage
		}
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		cleanMsg := s.maskPassword(errMsg, pass)
		return fmt.Errorf("svn checkout failed: %s", cleanMsg)
	}
	if counter != nil {
		counter.report(counter.files)
	}
	return nil
}

func (s *SVNService) doUpdate(ctx context.Context, targetDir, revision string, creds credentials.Provider) error {
	args := []string{"update", targetDir, "--non-interactive", "--no-auth-cache", "--trust-server-cert"}

	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}

	args, pass := s.appendCredArgs(args, creds)

	opts := models.ExecOptions{
		Command: "svn",
		Args:    args,
		Dir:     targetDir,
	}

	counter, handlers := s.progressHandlers()
	res, err := s.runner.Run(ctx, opts, handlers...)
	if err != nil || !res.Successful {
		errMsg := strings.TrimSpace(res.StdErr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(res.StdOut)
		}
		if errMsg == "" {
			errMsg = res.ErrorMessage
		}
		if errMsg == "" && err != nil {
			errMsg = err.Error()
		}
		cleanMsg := s.maskPassword(errMsg, pass)
		return fmt.Errorf("svn update failed: %s", cleanMsg)
	}
	if counter != nil {
		counter.report(counter.files)
	}
	return nil
}

// CheckoutFolders downloads only the named folders of repoURL, plus the files in its top folder,
// instead of the whole tree. Folders are paths inside the repository such as "Client" or
// "Services/Api". When a name is not a folder in the repository (a typing mistake, or a project
// name), the whole tree is downloaded so the caller still sees everything that exists.
func (s *SVNService) CheckoutFolders(ctx context.Context, repoURL, targetDir, revision string, folders []string, creds credentials.Provider) (*Info, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repository service: repoURL cannot be empty")
	}
	if targetDir == "" {
		return nil, fmt.Errorf("repository service: targetDir cannot be empty")
	}

	counter, _ := s.progressHandlers()

	s.logger.Info("checking out selected SVN folders", "url", repoURL, "dir", targetDir, "folders", strings.Join(folders, ", "))
	args := []string{"checkout", pegURL(repoURL), targetDir, "--depth", "files"}
	if err := s.runSVN(ctx, "svn checkout", args, "", revision, creds, counter); err != nil {
		return nil, err
	}

	// Pin every folder to the revision just checked out, so a commit made meanwhile cannot mix in.
	currentRev, err := s.GetRevision(ctx, targetDir)
	if err != nil || currentRev == "UNKNOWN" {
		currentRev = revision
	}

	complete := true
	for _, folder := range folders {
		rel, ok := cleanRepoFolder(folder)
		if !ok {
			complete = false
			break
		}

		folderPath := filepath.Join(targetDir, rel)
		args := []string{"update", "--parents", "--set-depth", "infinity", folderPath}
		updateErr := s.runSVN(ctx, "svn update", args, targetDir, currentRev, creds, counter)
		if ctx.Err() != nil {
			return nil, updateErr
		}
		if info, statErr := os.Stat(folderPath); updateErr != nil || statErr != nil || !info.IsDir() {
			complete = false
			break
		}
	}

	if !complete {
		s.logger.Info("a requested folder is not in the repository; downloading everything", "url", repoURL)
		if err := s.deepenAll(ctx, targetDir, currentRev, creds, counter); err != nil {
			return nil, err
		}
	}

	if counter != nil {
		counter.report(counter.files)
	}

	return &Info{
		URL:          repoURL,
		Revision:     currentRev,
		TargetDir:    targetDir,
		CheckedOutAt: time.Now(),
	}, nil
}

// DeepenAll downloads everything that a CheckoutFolders working copy left out.
func (s *SVNService) DeepenAll(ctx context.Context, targetDir, revision string, creds credentials.Provider) error {
	counter, _ := s.progressHandlers()
	if err := s.deepenAll(ctx, targetDir, revision, creds, counter); err != nil {
		return err
	}
	if counter != nil {
		counter.report(counter.files)
	}
	return nil
}

func (s *SVNService) deepenAll(ctx context.Context, targetDir, revision string, creds credentials.Provider, counter *progressCounter) error {
	return s.runSVN(ctx, "svn update", []string{"update", "--set-depth", "infinity", targetDir}, targetDir, revision, creds, counter)
}

// cleanRepoFolder turns a folder typed by a user into a relative path inside the working copy.
// It reports false for the top folder and for anything that would leave the working copy.
func cleanRepoFolder(folder string) (string, bool) {
	folder = strings.Trim(strings.ReplaceAll(strings.TrimSpace(folder), "\\", "/"), "/")
	if folder == "" || folder == "." {
		return "", false
	}
	rel := filepath.Clean(filepath.FromSlash(folder))
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// runSVN runs one svn command with the standard unattended flags, revision and credentials.
func (s *SVNService) runSVN(ctx context.Context, action string, args []string, dir, revision string, creds credentials.Provider, counter *progressCounter) error {
	args = append(args, "--non-interactive", "--no-auth-cache", "--trust-server-cert")
	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}
	args, pass := s.appendCredArgs(args, creds)

	var handlers []process.StreamHandler
	if counter != nil {
		handlers = append(handlers, counter)
	}

	res, err := s.runner.Run(ctx, models.ExecOptions{Command: "svn", Args: args, Dir: dir}, handlers...)
	if err != nil || !res.Successful {
		return s.commandError(action, res, err, pass)
	}
	return nil
}

// GetRevision fetches revision info from svn info.
func (s *SVNService) GetRevision(ctx context.Context, targetDir string) (string, error) {
	opts := models.ExecOptions{
		Command: "svn",
		Args:    []string{"info", "--show-item", "revision", targetDir},
		Dir:     targetDir,
	}

	res, err := s.runner.Run(ctx, opts)
	if err != nil || !res.Successful {
		return "UNKNOWN", nil
	}

	rev := strings.TrimSpace(res.StdOut)
	if rev == "" {
		return "HEAD", nil
	}
	return rev, nil
}

// ListFiles returns every path below repoURL without checking it out. Paths are relative to
// repoURL, use forward slashes, and end with "/" when they are directories.
func (s *SVNService) ListFiles(ctx context.Context, repoURL, revision string, creds credentials.Provider) ([]string, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repository service: repoURL cannot be empty")
	}

	args := []string{"list", "--recursive", pegURL(repoURL), "--non-interactive", "--no-auth-cache", "--trust-server-cert"}
	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}
	args, pass := s.appendCredArgs(args, creds)

	res, err := s.runner.Run(ctx, models.ExecOptions{Command: "svn", Args: args})
	if err != nil || !res.Successful {
		return nil, s.commandError("svn list", res, err, pass)
	}

	var paths []string
	for _, line := range strings.Split(res.StdOut, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// ReadFile returns the content of a single file in the repository without checking it out.
func (s *SVNService) ReadFile(ctx context.Context, fileURL, revision string, creds credentials.Provider) ([]byte, error) {
	args := []string{"cat", pegURL(fileURL), "--non-interactive", "--no-auth-cache", "--trust-server-cert"}
	if revision != "" && revision != "HEAD" {
		args = append(args, "--revision", revision)
	}
	args, pass := s.appendCredArgs(args, creds)

	res, err := s.runner.Run(ctx, models.ExecOptions{Command: "svn", Args: args})
	if err != nil || !res.Successful {
		return nil, s.commandError("svn cat", res, err, pass)
	}
	return []byte(res.StdOut), nil
}

// pegURL keeps svn from reading an "@" inside a path as a peg revision.
func pegURL(url string) string {
	if strings.Contains(url, "@") {
		return url + "@"
	}
	return url
}

func (s *SVNService) commandError(action string, res models.ProcessResult, err error, pass string) error {
	errMsg := strings.TrimSpace(res.StdErr)
	if errMsg == "" {
		errMsg = res.ErrorMessage
	}
	if errMsg == "" && err != nil {
		errMsg = err.Error()
	}
	return fmt.Errorf("%s failed: %s", action, s.maskPassword(errMsg, pass))
}

func (s *SVNService) appendCredArgs(args []string, creds credentials.Provider) ([]string, string) {
	if creds == nil {
		return args, ""
	}
	user := creds.Username()
	pass, _ := creds.Password()

	if user != "" {
		args = append(args, "--username", user)
	}
	if pass != "" {
		args = append(args, "--password", pass)
	}
	return args, pass
}

func (s *SVNService) maskPassword(msg, password string) string {
	if password != "" && strings.Contains(msg, password) {
		return strings.ReplaceAll(msg, password, "********")
	}
	return msg
}
