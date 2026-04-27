// Package metalhostcli is the customer-facing entry point for the Metalhost
// CLI library. The `metalhost` binary in this repo is a thin wrapper around
// NewRootCommand below.
//
// What's actually here is glue: this file aliases the proto-free helpers from
// pkg/metalhostcli/runtime, then layers the customer command tree (vm, disk,
// network, ...) on top via internal/command.
//
// Internal admin tools should import the `runtime` sub-package directly so
// they don't pick up customer proto descriptors transitively. See that
// package's doc comment for the reason.
package metalhostcli

import (
	"github.com/spf13/cobra"

	"github.com/AES-Services/metalhost-cli/internal/command"
	"github.com/AES-Services/metalhost-cli/pkg/metalhostcli/runtime"
)

// Options re-exports runtime.Options so callers can keep using
// `metalhostcli.Options{...}` literals without depending on the runtime
// sub-package directly.
type Options = runtime.Options

// Profile re-exports runtime.Profile (the config-file profile shape).
type Profile = runtime.Profile

// Runtime re-exports the proto-free runtime helper.
type Runtime = runtime.Runtime

// NewRootCommand builds the full customer CLI: bare root from the runtime
// package, plus the customer command tree (vm, disk, network, ...) attached
// from internal/command.
func NewRootCommand(opts Options) *cobra.Command {
	root := runtime.NewRootCommand(opts)
	command.AttachCustomerCommands(root, command.RootCommandOptions{
		Use:       opts.Use,
		Short:     opts.Short,
		UserAgent: opts.UserAgent,
	})
	return root
}

// RuntimeFromCommand is a thin pass-through to runtime.RuntimeFromCommand
// for callers that already import this package.
func RuntimeFromCommand(cmd *cobra.Command, userAgent string) (*Runtime, error) {
	return runtime.RuntimeFromCommand(cmd, userAgent)
}
