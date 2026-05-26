package command

import (
	"errors"
	"fmt"
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

// confirmDestructive is the shared --yes / interactive prompt guard for irreversible
// operations (delete vm, delete disk, delete project, delete org). `yes=true` skips the
// prompt; otherwise we read y/yes from stdin. The `target` is shown verbatim in the prompt
// so the user can sanity-check what's being destroyed before confirming.
func confirmDestructive(cmd *cobra.Command, yes bool, action, target string) error {
	if yes {
		return nil
	}
	pr := newPromptReader(cmd)
	ans := strings.ToLower(strings.TrimSpace(pr.readLine(fmt.Sprintf("%s %s — this is irreversible. Continue? [y/N]: ", action, target))))
	if ans != "y" && ans != "yes" {
		return errors.New("aborted")
	}
	return nil
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
