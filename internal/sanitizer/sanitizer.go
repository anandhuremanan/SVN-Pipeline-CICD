package sanitizer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anandhuremanan/cicd-engine/internal/logger"
)

// TargetType indicates whether a cleanup rule targets directories, files, or any entry.
type TargetType int

const (
	// TypeDirectory targets directories for removal.
	TypeDirectory TargetType = iota
	// TypeFile targets individual files for removal.
	TypeFile
	// TypeAny targets either directories or files.
	TypeAny
)

// Rule defines a single pattern matching rule for workspace sanitization.
type Rule struct {
	Name    string     `json:"name"`
	Pattern string     `json:"pattern"`
	Type    TargetType `json:"type"`
}

// Result summarizes the directories/files removed and disk space freed by sanitization.
type Result struct {
	DirectoriesRemoved []string `json:"directories_removed"`
	FilesRemoved       []string `json:"files_removed"`
	BytesFreed         int64    `json:"bytes_freed"`
}

// Sanitizer defines operations for cleaning generated files from build workspaces.
type Sanitizer interface {
	// Sanitize recursively walks workspace and removes matching generated artifacts.
	Sanitize(ctx context.Context, workspace string) (*Result, error)
}

// DefaultSanitizer implements Sanitizer with safety checks and configurable rules.
type DefaultSanitizer struct {
	rules  []Rule
	logger logger.Logger
}

// DefaultRules returns standard cleanup rules for Node, React, Next, .NET, and temp files.
func DefaultRules() []Rule {
	return []Rule{
		{Name: "Node Modules", Pattern: "node_modules", Type: TypeDirectory},
		{Name: "Dist Output", Pattern: "dist", Type: TypeDirectory},
		{Name: "Coverage", Pattern: "coverage", Type: TypeDirectory},
		{Name: "NPM Cache", Pattern: ".npm", Type: TypeDirectory},
		{Name: "Turbo Cache", Pattern: ".turbo", Type: TypeDirectory},
		{Name: "Next Output", Pattern: ".next", Type: TypeDirectory},
		{Name: "Dotnet Bin", Pattern: "bin", Type: TypeDirectory},
		{Name: "Dotnet Obj", Pattern: "obj", Type: TypeDirectory},
		{Name: "Test Results", Pattern: "TestResults", Type: TypeDirectory},
		{Name: "Cache", Pattern: ".cache", Type: TypeDirectory},
		{Name: "Temp", Pattern: ".tmp", Type: TypeDirectory},
	}
}

// ProtectedNames lists directory names that MUST NEVER be deleted.
var ProtectedNames = map[string]bool{
	".git":    true,
	".svn":    true,
	".vscode": true,
	".idea":   true,
}

// NewSanitizer constructs a DefaultSanitizer instance.
func NewSanitizer(customRules []Rule, log logger.Logger) *DefaultSanitizer {
	if log == nil {
		log = logger.NopLogger()
	}
	rules := customRules
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	return &DefaultSanitizer{
		rules:  rules,
		logger: log,
	}
}

// Sanitize recursively walks workspace and safely removes matching generated directories and files.
func (s *DefaultSanitizer) Sanitize(ctx context.Context, workspace string) (*Result, error) {
	if workspace == "" {
		return nil, fmt.Errorf("sanitizer: workspace path cannot be empty")
	}

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("sanitizer: invalid workspace path %q: %w", workspace, err)
	}

	info, err := os.Lstat(absWorkspace)
	if err != nil {
		return nil, fmt.Errorf("sanitizer: workspace does not exist %q: %w", absWorkspace, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("sanitizer: workspace path is not a directory %q", absWorkspace)
	}

	s.logger.Info("sanitizing workspace directory", "workspace", absWorkspace)

	res := &Result{
		DirectoriesRemoved: make([]string, 0),
		FilesRemoved:       make([]string, 0),
	}

	ruleMap := make(map[string]Rule)
	for _, r := range s.rules {
		ruleMap[strings.ToLower(r.Pattern)] = r
	}

	err = filepath.WalkDir(absWorkspace, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == absWorkspace {
			return nil
		}

		rel, relErr := filepath.Rel(absWorkspace, path)
		if relErr != nil || strings.HasPrefix(rel, "..") || rel == "." {
			return nil
		}

		name := d.Name()
		lowerName := strings.ToLower(name)

		if ProtectedNames[lowerName] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		lInfo, lErr := os.Lstat(path)
		if lErr != nil {
			return nil
		}

		// Never follow symbolic links
		if lInfo.Mode()&os.ModeSymlink != 0 {
			if rule, matches := ruleMap[lowerName]; matches && (rule.Type == TypeFile || rule.Type == TypeAny) {
				_ = os.Remove(path)
				res.FilesRemoved = append(res.FilesRemoved, rel)
			}
			return nil
		}

		if rule, matches := ruleMap[lowerName]; matches {
			if d.IsDir() && (rule.Type == TypeDirectory || rule.Type == TypeAny) {
				size := calculateDirSize(path)
				s.logger.Info("removing generated directory", "path", rel)
				removeErr := os.RemoveAll(path)
				if removeErr != nil {
					// Read-only files block removal on Windows. Making every file writable is slow
					// for large dependency folders, so it is only done when the first attempt fails.
					_ = filepath.WalkDir(path, func(p string, _ os.DirEntry, wErr error) error {
						if wErr == nil {
							_ = os.Chmod(p, 0777)
						}
						return nil
					})
					removeErr = os.RemoveAll(path)
				}
				if removeErr == nil {
					res.DirectoriesRemoved = append(res.DirectoriesRemoved, rel)
					res.BytesFreed += size
				} else {
					s.logger.Warn("failed to remove directory", "path", rel, "err", removeErr)
				}
				return filepath.SkipDir
			} else if !d.IsDir() && (rule.Type == TypeFile || rule.Type == TypeAny) {
				size := lInfo.Size()
				s.logger.Info("removing generated file", "path", rel)
				if removeErr := os.Remove(path); removeErr == nil {
					res.FilesRemoved = append(res.FilesRemoved, rel)
					res.BytesFreed += size
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sanitizer: error walking workspace: %w", err)
	}

	return res, nil
}

func calculateDirSize(dir string) int64 {
	var size int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, iErr := d.Info(); iErr == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
