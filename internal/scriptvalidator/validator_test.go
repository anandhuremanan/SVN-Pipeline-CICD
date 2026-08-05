package scriptvalidator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/scriptvalidator"
)

func writePackageJSON(t *testing.T, dir string, scripts map[string]string) {
	t.Helper()
	content := `{"scripts":{`
	parts := make([]string, 0, len(scripts))
	for k, v := range scripts {
		parts = append(parts, `"`+k+`":"`+v+`"`)
	}
	content += strings.Join(parts, ",") + "}}"
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644)
}

func TestResolveScript_Default(t *testing.T) {
	proj := models.Project{
		Name: "client",
		Type: models.ProjectTypeReact,
	}
	if got := scriptvalidator.ResolveScript(proj); got != "build" {
		t.Errorf("expected 'build', got %q", got)
	}
}

func TestResolveScript_Override(t *testing.T) {
	proj := models.Project{
		Name:  "client",
		Type:  models.ProjectTypeReact,
		Build: models.BuildConfig{Script: "deploy"},
	}
	if got := scriptvalidator.ResolveScript(proj); got != "deploy" {
		t.Errorf("expected 'deploy', got %q", got)
	}
}

func TestIsOverridden(t *testing.T) {
	plain := models.Project{Name: "a", Type: models.ProjectTypeReact}
	if scriptvalidator.IsOverridden(plain) {
		t.Error("expected plain project not to be overridden")
	}

	overridden := models.Project{
		Name:  "a",
		Type:  models.ProjectTypeReact,
		Build: models.BuildConfig{Script: "deploy"},
	}
	if !scriptvalidator.IsOverridden(overridden) {
		t.Error("expected overridden project to return true")
	}
}

func TestValidateAll_DefaultScriptExists(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{
		"build": "vite build",
		"lint":  "eslint .",
	})

	projects := []models.Project{
		{Name: "client", Type: models.ProjectTypeReact, RootDir: dir},
	}

	if err := scriptvalidator.ValidateAll(projects); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateAll_OverrideScriptExists(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{
		"build":  "vite build",
		"deploy": "vite build --mode production",
	})

	projects := []models.Project{
		{
			Name:    "client",
			Type:    models.ProjectTypeReact,
			RootDir: dir,
			Build:   models.BuildConfig{Script: "deploy"},
		},
	}

	if err := scriptvalidator.ValidateAll(projects); err != nil {
		t.Errorf("expected no error for valid override, got: %v", err)
	}
}

func TestValidateAll_MissingDefaultScript(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{
		"deploy": "vite build --mode production",
		"lint":   "eslint .",
	})

	projects := []models.Project{
		{Name: "client", Type: models.ProjectTypeReact, RootDir: dir},
	}

	err := scriptvalidator.ValidateAll(projects)
	if err == nil {
		t.Fatal("expected validation error for missing 'build' script, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "build") {
		t.Errorf("expected error to mention 'build', got: %s", msg)
	}
	if !strings.Contains(msg, "deploy") {
		t.Errorf("expected error to list available scripts, got: %s", msg)
	}
}

func TestValidateAll_MissingOverrideScript(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{
		"build": "vite build",
		"lint":  "eslint .",
		"test":  "vitest",
	})

	projects := []models.Project{
		{
			Name:    "client",
			Type:    models.ProjectTypeReact,
			RootDir: dir,
			Build:   models.BuildConfig{Script: "deploy-prod"},
		},
	}

	err := scriptvalidator.ValidateAll(projects)
	if err == nil {
		t.Fatal("expected validation error for missing 'deploy-prod' script, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "deploy-prod") {
		t.Errorf("expected error to mention 'deploy-prod', got: %s", msg)
	}
	// Should list all available scripts
	for _, s := range []string{"build", "lint", "test"} {
		if !strings.Contains(msg, s) {
			t.Errorf("expected error to list available script %q, got: %s", s, msg)
		}
	}
}

func TestValidateAll_DotnetProjectSkipped(t *testing.T) {
	dir := t.TempDir()
	// No package.json — dotnet projects should be skipped silently
	projects := []models.Project{
		{Name: "api", Type: models.ProjectTypeDotnet, RootDir: dir},
	}

	if err := scriptvalidator.ValidateAll(projects); err != nil {
		t.Errorf("dotnet projects should be skipped, got error: %v", err)
	}
}

func TestValidateAll_MissingPackageJSONIsSkipped(t *testing.T) {
	dir := t.TempDir()
	// No package.json written — validation should not fail here, builder will surface it

	projects := []models.Project{
		{Name: "client", Type: models.ProjectTypeReact, RootDir: dir},
	}

	if err := scriptvalidator.ValidateAll(projects); err != nil {
		t.Errorf("missing package.json should be skipped in pre-validation, got: %v", err)
	}
}
