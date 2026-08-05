package models

import (
	"time"
)

// StageStatus represents the execution state of a pipeline stage.
type StageStatus string

const (
	// StageStatusPending indicates stage has not started.
	StageStatusPending StageStatus = "PENDING"
	// StageStatusRunning indicates stage is currently executing.
	StageStatusRunning StageStatus = "RUNNING"
	// StageStatusSuccess indicates stage completed successfully.
	StageStatusSuccess StageStatus = "SUCCESS"
	// StageStatusFailed indicates stage failed.
	StageStatusFailed StageStatus = "FAILED"
	// StageStatusSkipped indicates stage was skipped.
	StageStatusSkipped StageStatus = "SKIPPED"
)

// StageResult summarizes the execution outcome of an individual pipeline stage.
type StageResult struct {
	// Name is the name of the stage (e.g., "Repository Checkout").
	Name string `json:"name"`
	// Status is the outcome status.
	Status StageStatus `json:"status"`
	// Duration is the total execution time of the stage.
	Duration time.Duration `json:"duration"`
	// ErrorMessage captures any failure error message from the stage.
	ErrorMessage string `json:"error_message,omitempty"`
}

// PipelineResult captures the overall outcome of a pipeline run.
type PipelineResult struct {
	// Success indicates if all required stages succeeded.
	Success bool `json:"success"`
	// Duration is the total pipeline execution time.
	Duration time.Duration `json:"duration"`
	// StageResults lists results of each executed stage.
	StageResults []StageResult `json:"stage_results"`
	// BuildResults lists build results of all scanned and built projects.
	BuildResults []BuildResult `json:"build_results"`
	// ErrorMessage records top-level pipeline failure error message if any.
	ErrorMessage string `json:"error_message,omitempty"`
}
