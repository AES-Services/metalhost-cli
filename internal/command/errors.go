package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"connectrpc.com/connect"
)

// Exit codes. 1 is the catch-all; the rest let scripts branch on failure class
// (e.g. retry on 6/unavailable, re-auth on 4/unauthenticated).
const (
	exitGeneric       = 1
	exitInvalid       = 2
	exitPermission    = 3
	exitUnauthacted   = 4
	exitNotFound      = 5
	exitUnavailable   = 6
	exitAlreadyExists = 9
	exitInterrupted   = 130
)

// HandleError renders err to w in a human-friendly way and returns the process
// exit code. It unwraps connect errors to show the status + message + an
// actionable hint instead of the raw "unauthenticated: ..." string, and maps
// the connect code to a stable exit code for scripting.
func HandleError(w io.Writer, err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(w, "Aborted.")
		return exitInterrupted
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return exitGeneric
	}

	code := connectErr.Code()
	fmt.Fprintf(w, "Error: %s (%s)\n", connectErr.Message(), code.String())
	if hint := hintForCode(code); hint != "" {
		fmt.Fprintf(w, "  %s\n", hint)
	}
	return exitCodeFor(code)
}

func hintForCode(code connect.Code) string {
	switch code {
	case connect.CodeUnauthenticated:
		return "Not signed in or the API key is invalid — run `metalhost auth login` or set METALHOST_API_KEY."
	case connect.CodePermissionDenied:
		return "Your principal lacks permission for this action — check the org/project role with `metalhost iam members list`."
	case connect.CodeNotFound:
		return "Resource not found — confirm the name (try the matching `list` command)."
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded:
		return "The API was unreachable — check your --endpoint and network, then retry."
	case connect.CodeAlreadyExists:
		return "A resource with that name already exists."
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition:
		return "The request was rejected — re-check the flags and resource state."
	default:
		return ""
	}
}

func exitCodeFor(code connect.Code) int {
	switch code {
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return exitInvalid
	case connect.CodePermissionDenied:
		return exitPermission
	case connect.CodeUnauthenticated:
		return exitUnauthacted
	case connect.CodeNotFound:
		return exitNotFound
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded:
		return exitUnavailable
	case connect.CodeAlreadyExists:
		return exitAlreadyExists
	default:
		return exitGeneric
	}
}
