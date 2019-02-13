package flagutil

import (
	"flag"
)

// OptionGroup provides an interface which can be implemented by an
// option handler (e.g. for GitHub or Kubernetes) to support generic
// option-group handling.
type OptionGroup interface {
	// AddFlags injects options into the given FlagSet.
	AddFlags(fs *flag.FlagSet)

	// Validate validates options.
	Validate(dryRun bool) error
}
