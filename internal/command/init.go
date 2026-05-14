package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/catalog/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/catalog/v1/catalogv1connect"
	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1/iamv1connect"
	projectv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1/projectv1connect"
	"github.com/AES-Services/metalhost-sdk/metalhost"

	"github.com/AES-Services/metalhost-cli/internal/config"
)

// authResult is the post-authentication shape that the rest of init relies on. Every auth
// branch (existing API key, email login, signup) ends here with the same fields populated.
type authResult struct {
	apiKey       string
	principal    string
	organization string
	project      string
}

// newInitCommand wires `metalhost init` — the first-run onboarding wizard. Handles four
// auth paths so a brand-new user can go from a fresh laptop to a usable CLI in one command:
//
//   1. Existing API key (legacy / scripted setups)
//   2. Email + password login (existing account, no key)
//   3. Sign up for a new account (creates org + default project)
//   4. OIDC browser login (delegated to `metalhost auth login --oidc <provider>`)
//
// After auth, the wizard fetches the caller's projects + datacenters, lets the user pick
// defaults, and saves everything to a profile that's set as current.
func newInitCommand(opts *rootOptions) *cobra.Command {
	var nonInteractive, force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive setup — auth, default project, default region",
		Long: `Walks through creating (or updating) a CLI profile end to end.

Handles four authentication paths:
  - Existing API key (paste it)
  - Email + password login
  - Sign up for a new account (creates an organization + default project)
  - OIDC browser login (delegated to 'metalhost auth login --oidc')

Saves the credentials, a default project, and a default region as a profile,
and sets the profile as current so subsequent commands need no flags.

Re-run any time to switch endpoints, rotate keys, or change defaults.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, opts, nonInteractive, force)
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "fail instead of prompting (for CI)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing profile without asking")
	return cmd
}

func runInit(cmd *cobra.Command, opts *rootOptions, nonInteractive, force bool) error {
	prompter := newPromptReader(cmd)
	stderr := cmd.ErrOrStderr()
	out := cmd.OutOrStdout()

	// Header
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "  metalhost · first-time setup")
	fmt.Fprintln(stderr, "  ---")
	fmt.Fprintln(stderr, "  This wizard saves a profile to ~/.config/metalhost/config.yaml")
	fmt.Fprintln(stderr, "  so the CLI knows your endpoint, API key, project, and region by")
	fmt.Fprintln(stderr, "  default. Re-run any time to switch endpoints or rotate keys.")
	fmt.Fprintln(stderr)

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}

	// 1. Profile name
	defaultName := strings.TrimSpace(opts.profile)
	if defaultName == "" {
		defaultName = strings.TrimSpace(cfg.CurrentProfile)
	}
	if defaultName == "" {
		defaultName = "default"
	}
	profileName := defaultWhenEmpty(prompter.readLine(fmt.Sprintf("Profile name [%s]: ", defaultName)), defaultName)
	if existing, ok := cfg.Profiles[profileName]; ok && existing != nil && !force {
		if nonInteractive {
			return fmt.Errorf("profile %q already exists; pass --force to overwrite", profileName)
		}
		confirm := strings.ToLower(strings.TrimSpace(prompter.readLine(
			fmt.Sprintf("Profile %q already exists. Overwrite? [y/N]: ", profileName))))
		if confirm != "y" && confirm != "yes" {
			fmt.Fprintln(stderr, "Aborted.")
			return nil
		}
	}

	// 2. Endpoint
	defaultEndpoint := "https://api.metalhost.net"
	if existing, ok := cfg.Profiles[profileName]; ok && existing != nil && existing.Endpoint != "" {
		defaultEndpoint = existing.Endpoint
	}
	endpoint := defaultWhenEmpty(
		prompter.readLine(fmt.Sprintf("API endpoint [%s]: ", defaultEndpoint)),
		defaultEndpoint,
	)
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return fmt.Errorf("endpoint must start with http:// or https:// (got %q)", endpoint)
	}

	// 3. Auth method
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "How would you like to authenticate?")
	fmt.Fprintln(stderr, "  [1] I have an API key (paste it)")
	fmt.Fprintln(stderr, "  [2] Log in with email + password (existing account)")
	fmt.Fprintln(stderr, "  [3] Sign up for a new account")
	fmt.Fprintln(stderr, "  [4] OIDC / browser SSO (Google, GitHub)")
	fmt.Fprintln(stderr)

	if nonInteractive {
		return errors.New("--non-interactive requires METALHOST_API_KEY + METALHOST_ENDPOINT; use `metalhost auth login --api-key` for scripted setup")
	}

	method := strings.TrimSpace(prompter.readLine("Pick [1-4]: "))
	if method == "" {
		method = "1"
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var result *authResult
	switch method {
	case "1":
		result, err = authViaAPIKey(ctx, prompter, stderr, endpoint, opts.userAgentString())
	case "2":
		result, err = authViaLogin(ctx, prompter, stderr, endpoint, opts.userAgentString())
	case "3":
		result, err = authViaSignup(ctx, prompter, stderr, endpoint, opts.userAgentString())
	case "4":
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "OIDC / browser login isn't bundled in init yet. Run:")
		fmt.Fprintln(stderr, "  metalhost auth login --oidc google     # or --oidc github")
		fmt.Fprintln(stderr, "…then re-run `metalhost init` (it'll detect the saved key and skip ahead).")
		return nil
	default:
		return fmt.Errorf("unknown choice %q (expected 1-4)", method)
	}
	if err != nil {
		return err
	}

	// 4. Build an authenticated SDK config for the picker calls.
	sdkCfg := metalhost.Config{
		Endpoint:   endpoint,
		APIKey:     result.apiKey,
		HTTPClient: &http.Client{Transport: metalhost.Config{APIKey: result.apiKey, UserAgent: opts.userAgentString()}.RoundTripper(http.DefaultTransport)},
		UserAgent:  opts.userAgentString(),
	}

	// 5. Project selection.
	//
	// After signup/verify the server has created the org but NOT a project — the welcome
	// wizard creates the first project explicitly (decision §26 / proto SignUpResponse:
	// project_name is only populated in legacy direct-create mode). Mirror that here:
	// if the user has no project, prompt to create one. Otherwise list + pick.
	project := result.project
	if project == "" {
		project, err = pickOrCreateProject(ctx, prompter, stderr, sdkCfg, result.organization, nonInteractive)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stderr, "\nUsing project %s\n", project)
	}

	// 6. Region selection.
	region, err := pickDatacenter(ctx, prompter, stderr, sdkCfg, nonInteractive)
	if err != nil {
		return err
	}

	// 7. Save profile.
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*config.Profile{}
	}
	cfg.Profiles[profileName] = &config.Profile{
		Endpoint:     endpoint,
		APIKey:       result.apiKey,
		Organization: result.organization,
		Project:      project,
		Region:       region,
	}
	cfg.CurrentProfile = profileName
	if err := config.Save(opts.configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// 8. Confirmation + next steps
	fmt.Fprintln(out)
	fmt.Fprintf(out, "✓ Saved profile %q and set it as current.\n", profileName)
	if result.principal != "" {
		fmt.Fprintf(out, "  Authenticated as %s.\n", result.principal)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Next:")
	fmt.Fprintln(out, "  metalhost auth whoami           # confirm the saved identity")
	fmt.Fprintln(out, "  metalhost catalog datacenter list")
	fmt.Fprintln(out, "  metalhost vm list")
	return nil
}

// ─────────────────────── Auth branches ───────────────────────

func authViaAPIKey(ctx context.Context, p *promptReader, stderr io.Writer, endpoint, userAgent string) (*authResult, error) {
	// `io.Writer` is just here to satisfy the function signature shape — we only need a
	// plain io.Writer. Cast at use.
	_ = stderr
	apiKey, err := p.readPassword("API key (input hidden): ")
	if err != nil {
		return nil, fmt.Errorf("read api key: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	// Validate by hitting GetCallerIdentity.
	sdkCfg := metalhost.Config{
		Endpoint:   endpoint,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Transport: metalhost.Config{APIKey: apiKey, UserAgent: userAgent}.RoundTripper(http.DefaultTransport)},
		UserAgent:  userAgent,
	}
	iam := iamv1connect.NewIamServiceClient(sdkCfg.Client(), sdkCfg.BaseURL())
	resp, err := iam.GetCallerIdentity(ctx, connect.NewRequest(&iamv1.GetCallerIdentityRequest{}))
	if err != nil {
		return nil, fmt.Errorf("validate API key: %w", err)
	}
	id := resp.Msg
	r := &authResult{apiKey: apiKey, principal: id.GetPrincipal(), project: id.GetScopeProject()}
	if r.project == "" {
		r.project = id.GetDefaultProject()
	}
	if orgs := id.GetAccessibleOrganizations(); len(orgs) > 0 {
		r.organization = orgs[0]
	}
	return r, nil
}

func authViaLogin(ctx context.Context, p *promptReader, _ io.Writer, endpoint, userAgent string) (*authResult, error) {
	email := strings.TrimSpace(p.readLine("Email: "))
	if email == "" {
		return nil, errors.New("email is required")
	}
	password, err := p.readPassword("Password (input hidden): ")
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, errors.New("password is required")
	}

	client := iamClientForEndpoint(endpoint, userAgent)
	resp, err := client.Login(ctx, connect.NewRequest(&iamv1.LoginRequest{Email: email, Password: password}))
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	r := &authResult{
		apiKey:    resp.Msg.GetSecret(),
		principal: resp.Msg.GetPrincipal(),
	}
	// Login doesn't return org/project — fetch via GetCallerIdentity to fill them in.
	sdkCfg := metalhost.Config{
		Endpoint:   endpoint,
		APIKey:     r.apiKey,
		HTTPClient: &http.Client{Transport: metalhost.Config{APIKey: r.apiKey, UserAgent: userAgent}.RoundTripper(http.DefaultTransport)},
		UserAgent:  userAgent,
	}
	iam := iamv1connect.NewIamServiceClient(sdkCfg.Client(), sdkCfg.BaseURL())
	if idResp, err := iam.GetCallerIdentity(ctx, connect.NewRequest(&iamv1.GetCallerIdentityRequest{})); err == nil {
		r.project = idResp.Msg.GetScopeProject()
		if r.project == "" {
			r.project = idResp.Msg.GetDefaultProject()
		}
		if orgs := idResp.Msg.GetAccessibleOrganizations(); len(orgs) > 0 {
			r.organization = orgs[0]
		}
	}
	return r, nil
}

func authViaSignup(ctx context.Context, p *promptReader, stderr io.Writer, endpoint, userAgent string) (*authResult, error) {
	w := stderr
	email := strings.TrimSpace(p.readLine("Email: "))
	if email == "" {
		return nil, errors.New("email is required")
	}
	displayName := strings.TrimSpace(p.readLine("Organization name (any human-readable label): "))
	password, err := p.readPassword("Choose a password (input hidden): ")
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	password = strings.TrimSpace(password)
	if len(password) < 12 {
		return nil, errors.New("password must be at least 12 characters")
	}
	confirm, err := p.readPassword("Confirm password (input hidden): ")
	if err != nil {
		return nil, fmt.Errorf("read confirm password: %w", err)
	}
	if confirm != password {
		return nil, errors.New("passwords don't match")
	}

	client := iamClientForEndpoint(endpoint, userAgent)
	resp, err := client.SignUp(ctx, connect.NewRequest(&iamv1.SignUpRequest{
		Email:       email,
		DisplayName: displayName,
		Password:    password,
	}))
	if err != nil {
		return nil, fmt.Errorf("signup: %w", err)
	}

	if !resp.Msg.GetVerificationPending() {
		// Legacy direct-create path — server returned the key immediately.
		fmt.Fprintln(w, "  → account created, API key issued")
		return &authResult{
			apiKey:       resp.Msg.GetSecret(),
			principal:    resp.Msg.GetPrincipal(),
			organization: resp.Msg.GetOrganizationName(),
			project:      resp.Msg.GetProjectName(),
		}, nil
	}

	// Verification flow.
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  → verification email sent to %s\n", email)
	fmt.Fprintln(w, "    Click the link in the email OR paste the verification token below.")
	fmt.Fprintln(w, "    Token format: a long base64-ish string from the email body.")
	fmt.Fprintln(w)

	// Poll for a token interactively.
	var verifyResp *connect.Response[iamv1.VerifyEmailResponse]
	for attempts := 0; attempts < 5; attempts++ {
		token := strings.TrimSpace(p.readLine("Verification token: "))
		if token == "" {
			fmt.Fprintln(w, "  → token is required (or Ctrl-C to abort)")
			continue
		}
		verifyResp, err = client.VerifyEmail(ctx, connect.NewRequest(&iamv1.VerifyEmailRequest{Token: token}))
		if err == nil {
			break
		}
		fmt.Fprintf(w, "  → verify failed: %v\n", err)
		fmt.Fprintln(w, "    (paste the token from the email again; expires in 24h)")
		time.Sleep(500 * time.Millisecond)
	}
	if verifyResp == nil {
		return nil, errors.New("email verification did not succeed after 5 attempts; re-run init with the token from the email")
	}

	return &authResult{
		apiKey:       verifyResp.Msg.GetSecret(),
		principal:    verifyResp.Msg.GetPrincipal(),
		organization: verifyResp.Msg.GetOrganizationName(),
		project:      verifyResp.Msg.GetProjectName(),
	}, nil
}

// ─────────────────────── Pickers ───────────────────────

// pickOrCreateProject is the init wrapper around project selection — it offers to create
// the first project when the caller has none (matching the welcome wizard's step 1).
// Used when the auth branch didn't already give us a project (login / api-key paths, or
// signup-with-verification on a server that doesn't pre-create one).
func pickOrCreateProject(ctx context.Context, p *promptReader, stderr io.Writer, sdkCfg metalhost.Config, orgName string, nonInteractive bool) (string, error) {
	pc := projectv1connect.NewProjectServiceClient(sdkCfg.Client(), sdkCfg.BaseURL())
	listResp, err := pc.ListProjects(ctx, connect.NewRequest(&projectv1.ListProjectsRequest{PageSize: 100}))
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}
	if len(listResp.Msg.GetProjects()) > 0 {
		// Existing projects — fall through to the regular picker.
		return pickProject(ctx, p, stderr, sdkCfg, "", nonInteractive)
	}

	// No projects yet. Offer to create one — same step 1 the web wizard runs.
	if orgName == "" {
		return "", errors.New("no projects and no organization in scope — sign up or get added to an org first")
	}
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "You don't have any projects yet under %s.\n", orgName)

	if nonInteractive {
		return "", errors.New("--non-interactive set but no project exists; create one first via `metalhost project create`")
	}

	defaultSlug := "default"
	slug := defaultWhenEmpty(
		p.readLine(fmt.Sprintf("Slug for your first project (used in resource names) [%s]: ", defaultSlug)),
		defaultSlug,
	)
	displayName := strings.TrimSpace(p.readLine(fmt.Sprintf("Display name [%s]: ", slug)))
	if displayName == "" {
		displayName = slug
	}
	projectName := "projects/" + slug

	createResp, err := pc.CreateProject(ctx, connect.NewRequest(&projectv1.CreateProjectRequest{
		Name:        projectName,
		DisplayName: displayName,
		Parent:      orgName,
	}))
	if err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	fmt.Fprintf(stderr, "  → created %s\n", createResp.Msg.GetProject().GetName())
	return createResp.Msg.GetProject().GetName(), nil
}

func pickProject(ctx context.Context, p *promptReader, stderr io.Writer, sdkCfg metalhost.Config, defaultProject string, nonInteractive bool) (string, error) {
	w := stderr
	pc := projectv1connect.NewProjectServiceClient(sdkCfg.Client(), sdkCfg.BaseURL())
	resp, err := pc.ListProjects(ctx, connect.NewRequest(&projectv1.ListProjectsRequest{PageSize: 100}))
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}
	projects := resp.Msg.GetProjects()
	if len(projects) == 0 {
		fmt.Fprintln(w, "  → no projects yet; saving without a default")
		return "", nil
	}
	if len(projects) == 1 {
		choice := projects[0].GetName()
		fmt.Fprintf(w, "\nUsing project %s (only option)\n", choice)
		return choice, nil
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available projects:")
	defaultIdx := -1
	for i, pr := range projects {
		marker := " "
		if pr.GetName() == defaultProject {
			marker = "*"
			defaultIdx = i
		}
		display := pr.GetDisplayName()
		if display == "" {
			display = "(no display name)"
		}
		fmt.Fprintf(w, "  %s [%d] %s — %s\n", marker, i+1, pr.GetName(), display)
	}

	if nonInteractive {
		if defaultIdx >= 0 {
			return projects[defaultIdx].GetName(), nil
		}
		return "", fmt.Errorf("multiple projects available and --non-interactive set")
	}

	defaultLabel := "1"
	if defaultIdx >= 0 {
		defaultLabel = strconv.Itoa(defaultIdx + 1)
	}
	for {
		line := strings.TrimSpace(p.readLine(fmt.Sprintf("Pick a default project [%s]: ", defaultLabel)))
		if line == "" {
			line = defaultLabel
		}
		idx, err := strconv.Atoi(line)
		if err != nil || idx < 1 || idx > len(projects) {
			fmt.Fprintf(w, "  → invalid choice; enter 1-%d\n", len(projects))
			continue
		}
		return projects[idx-1].GetName(), nil
	}
}

func pickDatacenter(ctx context.Context, p *promptReader, stderr io.Writer, sdkCfg metalhost.Config, nonInteractive bool) (string, error) {
	w := stderr
	cc := catalogv1connect.NewCatalogServiceClient(sdkCfg.Client(), sdkCfg.BaseURL())
	resp, err := cc.ListDatacenters(ctx, connect.NewRequest(&catalogv1.ListDatacentersRequest{PageSize: 100}))
	if err != nil {
		return "", fmt.Errorf("list datacenters: %w", err)
	}
	dcs := resp.Msg.GetDatacenters()
	if len(dcs) == 0 {
		fmt.Fprintln(w, "  → no datacenters visible; saving without a default region")
		return "", nil
	}
	if len(dcs) == 1 {
		choice := dcs[0].GetName()
		fmt.Fprintf(w, "\nUsing datacenter %s (only option)\n", choice)
		return choice, nil
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available datacenters:")
	for i, d := range dcs {
		display := d.GetDisplayName()
		if display == "" {
			display = "(no display name)"
		}
		fmt.Fprintf(w, "    [%d] %s — %s\n", i+1, d.GetName(), display)
	}

	if nonInteractive {
		return dcs[0].GetName(), nil
	}
	for {
		line := strings.TrimSpace(p.readLine("Pick a default datacenter [1]: "))
		if line == "" {
			line = "1"
		}
		idx, err := strconv.Atoi(line)
		if err != nil || idx < 1 || idx > len(dcs) {
			fmt.Fprintf(w, "  → invalid choice; enter 1-%d\n", len(dcs))
			continue
		}
		return dcs[idx-1].GetName(), nil
	}
}

// ─────────────────────── Helpers ───────────────────────

func defaultWhenEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

