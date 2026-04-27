package command

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

type pageFlags struct {
	pageSize  int32
	pageToken string
	limit     int32
}

func addPageFlags(cmd *cobra.Command, f *pageFlags) {
	cmd.Flags().Int32Var(&f.pageSize, "page-size", 50, "page size")
	cmd.Flags().StringVar(&f.pageToken, "page-token", "", "page token")
	cmd.Flags().Int32Var(&f.limit, "limit", 0, "maximum results to request")
}

func effectivePageSize(f pageFlags) int32 {
	if f.limit > 0 && (f.pageSize == 0 || f.limit < f.pageSize) {
		return f.limit
	}
	if f.pageSize == 0 {
		return 50
	}
	return f.pageSize
}

func requireProject(ctx *commandContext, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if ctx.profile.Project != "" {
		return ctx.profile.Project, nil
	}
	return "", errors.New("project is required; pass --project or set METALHOST_PROJECT/profile default")
}

func stringMapFromPairs(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		if ok && strings.TrimSpace(k) != "" {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}
