package builder

import (
	"github.com/anandhuremanan/cicd-engine/internal/logger"
	"github.com/anandhuremanan/cicd-engine/internal/process"
)

// NewDefaultRegistry constructs a Builder Registry pre-loaded with all standard builders.
func NewDefaultRegistry(runner process.Runner, log logger.Logger) Registry {
	if log == nil {
		log = logger.NopLogger()
	}
	reg := NewRegistry()
	reg.Register(NewReactBuilder(runner, log))
	reg.Register(NewNextBuilder(runner, log))
	reg.Register(NewDotnetBuilder(runner, log))
	return reg
}
