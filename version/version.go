package version

import (
	"go.uber.org/atomic"
)

var (
	Version       = "v0.0"    // git name-rev --tags --name-only $(git rev-parse HEAD)
	CommitHash    = "unknown" // git rev-parse HEAD
	BuiltAt       = "unknown" // LC_ALL=C date
	ClosingStatus = atomic.NewBool(false)
)
