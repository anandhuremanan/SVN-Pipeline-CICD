package builder

import (
	"fmt"
	"sync"

	"github.com/anandhuremanan/cicd-engine/internal/models"
)

// Registry manages registration and resolution of project-specific Builders.
type Registry interface {
	// Register adds a Builder implementation to the registry.
	Register(builder Builder)
	// BuilderFor resolves the matching Builder for a given project.
	BuilderFor(project models.Project) (Builder, error)
}

// DefaultRegistry implements Registry with thread-safe builder lookup.
type DefaultRegistry struct {
	mu       sync.RWMutex
	builders []Builder
}

// NewRegistry creates a new empty DefaultRegistry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		builders: make([]Builder, 0),
	}
}

// Register registers a new Builder.
func (r *DefaultRegistry) Register(builder Builder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builders = append(r.builders, builder)
}

// BuilderFor iterates through registered builders and returns the first matching builder.
func (r *DefaultRegistry) BuilderFor(project models.Project) (Builder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, b := range r.builders {
		if b.CanBuild(project) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("builder registry: no builder registered for project %q (type: %s)", project.Name, project.Type)
}
