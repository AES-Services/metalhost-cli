package command

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	supportv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/support/v1"
)

// newSupportCommand wires the customer-facing support namespace. Today only the
// ticket subtree is implemented; future siblings (KB articles, status-page incidents,
// etc.) hang off the same `mh support` root.
func newSupportCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "support",
		Short:   "Customer support tools",
		Aliases: []string{"sup"},
	}
	cmd.AddCommand(newSupportTicketCommand(opts))
	return cmd
}

func newSupportTicketCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ticket",
		Aliases: []string{"tickets"},
		Short:   "Manage support tickets",
	}
	cmd.AddCommand(
		newSupportTicketCreateCommand(opts),
		newSupportTicketListCommand(opts),
		newSupportTicketGetCommand(opts),
		newSupportTicketReplyCommand(opts),
		newSupportTicketCloseCommand(opts),
	)
	return cmd
}

func newSupportTicketCreateCommand(opts *rootOptions) *cobra.Command {
	var (
		org      string
		subject  string
		body     string
		category string
		priority string
		refs     []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open a new support ticket",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.supportClient()
			if err != nil {
				return err
			}
			cat, err := parseSupportCategory(category)
			if err != nil {
				return err
			}
			pri, err := parseSupportPriority(priority)
			if err != nil {
				return err
			}
			parsedRefs, err := parseSupportResourceRefs(refs)
			if err != nil {
				return err
			}
			resp, err := client.CreateTicket(cmd.Context(), connect.NewRequest(&supportv1.CreateTicketRequest{
				OrganizationName: strings.TrimSpace(org),
				Subject:          strings.TrimSpace(subject),
				Body:             body,
				Category:         cat,
				Priority:         pri,
				ResourceRefs:     parsedRefs,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "organization name (required)")
	cmd.Flags().StringVarP(&subject, "subject", "s", "", "ticket subject (required)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "initial message body (required)")
	cmd.Flags().StringVar(&category, "category", "GENERAL", "category (GENERAL|TECHNICAL|BILLING|ABUSE|FEATURE)")
	cmd.Flags().StringVar(&priority, "priority", "NORMAL", "priority (LOW|NORMAL|HIGH|URGENT)")
	cmd.Flags().StringArrayVar(&refs, "resource", nil, "attach a resource as type=ID (repeatable, e.g. --resource vm=vm-123)")
	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("subject")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newSupportTicketListCommand(opts *rootOptions) *cobra.Command {
	var (
		pages  pageFlags
		org    string
		status string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your organization's tickets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.supportClient()
			if err != nil {
				return err
			}
			st, err := parseSupportStatus(status)
			if err != nil {
				return err
			}
			resp, err := client.ListTickets(cmd.Context(), connect.NewRequest(&supportv1.ListTicketsRequest{
				OrganizationName: strings.TrimSpace(org),
				StatusFilter:     st,
				PageSize:         effectivePageSize(pages),
				PageToken:        pages.pageToken,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "organization name (required)")
	cmd.Flags().StringVar(&status, "status", "", "filter by status (OPEN|PENDING_CUSTOMER|PENDING_STAFF|RESOLVED|CLOSED); default = all non-CLOSED")
	addPageFlags(cmd, &pages)
	_ = cmd.MarkFlagRequired("org")
	return cmd
}

func newSupportTicketGetCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get TICKET_ID",
		Short: "Show a ticket and its message thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.supportClient()
			if err != nil {
				return err
			}
			resp, err := client.GetTicket(cmd.Context(), connect.NewRequest(&supportv1.GetTicketRequest{
				Name: args[0],
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	return cmd
}

func newSupportTicketReplyCommand(opts *rootOptions) *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "reply TICKET_ID",
		Short: "Post a reply to a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.supportClient()
			if err != nil {
				return err
			}
			resp, err := client.ReplyTicket(cmd.Context(), connect.NewRequest(&supportv1.ReplyTicketRequest{
				Name: args[0],
				Body:     body,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "reply body (required)")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newSupportTicketCloseCommand(opts *rootOptions) *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "close TICKET_ID",
		Short: "Close a ticket (with an optional final note)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.supportClient()
			if err != nil {
				return err
			}
			resp, err := client.CloseTicket(cmd.Context(), connect.NewRequest(&supportv1.CloseTicketRequest{
				Name: args[0],
				Body:     body,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "optional final message before close")
	return cmd
}

// ---------------------------------------------------------------------------
// Enum + ref parsers
// ---------------------------------------------------------------------------

func parseSupportStatus(s string) (supportv1.TicketStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return supportv1.TicketStatus_TICKET_STATUS_UNSPECIFIED, nil
	case "OPEN":
		return supportv1.TicketStatus_TICKET_STATUS_OPEN, nil
	case "PENDING_CUSTOMER", "PENDING-CUSTOMER":
		return supportv1.TicketStatus_TICKET_STATUS_PENDING_CUSTOMER, nil
	case "PENDING_STAFF", "PENDING-STAFF":
		return supportv1.TicketStatus_TICKET_STATUS_PENDING_STAFF, nil
	case "RESOLVED":
		return supportv1.TicketStatus_TICKET_STATUS_RESOLVED, nil
	case "CLOSED":
		return supportv1.TicketStatus_TICKET_STATUS_CLOSED, nil
	default:
		return 0, fmt.Errorf("invalid status %q (want OPEN|PENDING_CUSTOMER|PENDING_STAFF|RESOLVED|CLOSED)", s)
	}
}

func parseSupportPriority(s string) (supportv1.TicketPriority, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return supportv1.TicketPriority_TICKET_PRIORITY_UNSPECIFIED, nil
	case "LOW":
		return supportv1.TicketPriority_TICKET_PRIORITY_LOW, nil
	case "NORMAL":
		return supportv1.TicketPriority_TICKET_PRIORITY_NORMAL, nil
	case "HIGH":
		return supportv1.TicketPriority_TICKET_PRIORITY_HIGH, nil
	case "URGENT":
		return supportv1.TicketPriority_TICKET_PRIORITY_URGENT, nil
	default:
		return 0, fmt.Errorf("invalid priority %q (want LOW|NORMAL|HIGH|URGENT)", s)
	}
}

func parseSupportCategory(s string) (supportv1.TicketCategory, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return supportv1.TicketCategory_TICKET_CATEGORY_UNSPECIFIED, nil
	case "GENERAL":
		return supportv1.TicketCategory_TICKET_CATEGORY_GENERAL, nil
	case "TECHNICAL":
		return supportv1.TicketCategory_TICKET_CATEGORY_TECHNICAL, nil
	case "BILLING":
		return supportv1.TicketCategory_TICKET_CATEGORY_BILLING, nil
	case "ABUSE":
		return supportv1.TicketCategory_TICKET_CATEGORY_ABUSE, nil
	case "FEATURE":
		return supportv1.TicketCategory_TICKET_CATEGORY_FEATURE, nil
	default:
		return 0, fmt.Errorf("invalid category %q (want GENERAL|TECHNICAL|BILLING|ABUSE|FEATURE)", s)
	}
}

// parseSupportResourceRefs accepts repeated "type=id" pairs and emits ResourceRef messages.
// Optional display label can be appended with a second `=`: "vm=vm-001=prod-web-1".
func parseSupportResourceRefs(in []string) ([]*supportv1.ResourceRef, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*supportv1.ResourceRef, 0, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 3)
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid --resource %q (want type=ID or type=ID=display)", raw)
		}
		ref := &supportv1.ResourceRef{
			Type: strings.TrimSpace(parts[0]),
			Id:   strings.TrimSpace(parts[1]),
		}
		if len(parts) == 3 {
			ref.Display = strings.TrimSpace(parts[2])
		}
		out = append(out, ref)
	}
	return out, nil
}
