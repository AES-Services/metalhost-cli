package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/AES-Services/metalhost-cli/internal/config"
	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1/iamv1connect"
	"github.com/AES-Services/metalhost-sdk/metalhost"
)

// loopbackTimeout caps how long the CLI waits for the browser-side OIDC redirect to land back
// on the loopback listener. Long enough that a slow user can finish the flow, short enough
// that a stale process doesn't hang forever.
const loopbackTimeout = 5 * time.Minute

// authFlowsCommands attaches the signup / verify / login(email,oidc) / link subcommands to an
// existing `auth` command. Called from newAuthCommand to keep root.go small.
func authFlowsCommands(auth *cobra.Command, opts *rootOptions) {
	auth.AddCommand(newSignupCommand(opts))
	auth.AddCommand(newVerifyEmailCommand(opts))
	auth.AddCommand(newLinkOidcCommand(opts))
}

// newSignupCommand prompts for email + password (or accepts via flags), calls SignUp, and
// instructs the user to check their email + run `mh auth verify --token`. The endpoint must
// be supplied (env METALHOST_ENDPOINT or --endpoint) since signup runs before any profile is
// configured.
func newSignupCommand(opts *rootOptions) *cobra.Command {
	var email, displayName string
	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new Metalhost account (email + password). Triggers a verification email.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpoint := strings.TrimSpace(opts.endpoint)
			if endpoint == "" {
				endpoint = strings.TrimSpace(os.Getenv("METALHOST_ENDPOINT"))
			}
			if endpoint == "" {
				return errors.New("--endpoint or METALHOST_ENDPOINT is required for signup")
			}
			pr := newPromptReader(cmd)
			email = strings.TrimSpace(strings.ToLower(pr.promptIfEmpty(email, "Email: ")))
			if email == "" {
				return errors.New("email is required")
			}
			displayName = strings.TrimSpace(pr.promptIfEmpty(displayName, "Organization display name: "))
			if displayName == "" {
				return errors.New("display-name is required")
			}
			password, err := pr.readPassword("Password (≥8 chars): ")
			if err != nil {
				return err
			}
			if len(password) < 8 {
				return errors.New("password must be at least 8 characters")
			}
			confirm, err := pr.readPassword("Confirm password: ")
			if err != nil {
				return err
			}
			if confirm != password {
				return errors.New("passwords do not match")
			}

			client := iamClientForEndpoint(endpoint, opts.userAgentString())
			resp, err := client.SignUp(cmd.Context(), connect.NewRequest(&iamv1.SignUpRequest{
				DisplayName: displayName, Email: email, Password: password,
			}))
			if err != nil {
				return err
			}
			if !resp.Msg.GetVerificationPending() {
				// Server is in legacy direct-create mode (closed beta). Save the returned key.
				return saveLoginResult(cmd, opts, endpoint, resp.Msg.GetPrincipal(),
					resp.Msg.GetSecret(), resp.Msg.GetOrganizationName(), resp.Msg.GetProjectName())
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"A verification email has been sent to %s.\n"+
					"Click the link in the email, or run:\n  metalhost auth verify --token <TOKEN>\n",
				email)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "email address")
	cmd.Flags().StringVar(&displayName, "display-name", "", "human-readable organization name")
	return cmd
}

// newVerifyEmailCommand consumes a verification token from `mh auth signup` and saves the
// resulting API key to the active profile (or the default if none is selected).
func newVerifyEmailCommand(opts *rootOptions) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Complete email verification using the token from the signup email.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpoint := strings.TrimSpace(opts.endpoint)
			if endpoint == "" {
				endpoint = strings.TrimSpace(os.Getenv("METALHOST_ENDPOINT"))
			}
			if endpoint == "" {
				return errors.New("--endpoint or METALHOST_ENDPOINT is required")
			}
			if strings.TrimSpace(token) == "" {
				return errors.New("--token is required")
			}
			client := iamClientForEndpoint(endpoint, opts.userAgentString())
			resp, err := client.VerifyEmail(cmd.Context(), connect.NewRequest(&iamv1.VerifyEmailRequest{Token: token}))
			if err != nil {
				return err
			}
			return saveLoginResult(cmd, opts, endpoint, resp.Msg.GetPrincipal(),
				resp.Msg.GetSecret(), resp.Msg.GetOrganizationName(), resp.Msg.GetProjectName())
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "verification token from the signup email")
	return cmd
}

// passwordLogin runs the email/password Login flow and saves the resulting API key. Surfaced
// from newAuthCommand's existing login subcommand via the --email flag.
func passwordLogin(cmd *cobra.Command, opts *rootOptions, email string) error {
	endpoint := strings.TrimSpace(opts.endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("METALHOST_ENDPOINT"))
	}
	if endpoint == "" {
		return errors.New("--endpoint or METALHOST_ENDPOINT is required")
	}
	pr := newPromptReader(cmd)
	email = strings.TrimSpace(strings.ToLower(pr.promptIfEmpty(email, "Email: ")))
	if email == "" {
		return errors.New("email is required")
	}
	password, err := pr.readPassword("Password: ")
	if err != nil {
		return err
	}
	client := iamClientForEndpoint(endpoint, opts.userAgentString())
	resp, err := client.Login(cmd.Context(), connect.NewRequest(&iamv1.LoginRequest{
		Email: email, Password: password,
	}))
	if err != nil {
		return err
	}
	return saveLoginResult(cmd, opts, endpoint, resp.Msg.GetPrincipal(), resp.Msg.GetSecret(), "", "")
}

// oidcLogin runs the OIDC loopback browser flow against the named provider and saves the API
// key. The provider must already be registered server-side (admin OIDC RPCs); standard slugs
// are "google" and "github" but operators can register others.
func oidcLogin(cmd *cobra.Command, opts *rootOptions, providerSlug string) error {
	endpoint := strings.TrimSpace(opts.endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("METALHOST_ENDPOINT"))
	}
	if endpoint == "" {
		return errors.New("--endpoint or METALHOST_ENDPOINT is required")
	}
	providerName := "oidc-providers/" + strings.TrimSpace(providerSlug)
	client := iamClientForEndpoint(endpoint, opts.userAgentString())

	loopback, err := startLoopbackListener(cmd.Context())
	if err != nil {
		return err
	}
	defer loopback.shutdown()

	begin, err := client.StartOidcLogin(cmd.Context(), connect.NewRequest(&iamv1.StartOidcLoginRequest{
		ProviderName: providerName, RedirectUri: loopback.redirectURI,
	}))
	if err != nil {
		return err
	}
	if err := openBrowser(begin.Msg.GetAuthorizeUrl()); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "open this URL to continue:\n  %s\n", begin.Msg.GetAuthorizeUrl())
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Opening browser…")
	}

	code, state, err := loopback.wait(cmd.Context())
	if err != nil {
		return err
	}
	if state != begin.Msg.GetState() {
		return errors.New("state mismatch — possible CSRF; aborting")
	}
	complete, err := client.CompleteOidcLogin(cmd.Context(), connect.NewRequest(&iamv1.CompleteOidcLoginRequest{
		ProviderName: providerName, Code: code, State: state, RedirectUri: loopback.redirectURI,
	}))
	if err != nil {
		return err
	}
	return saveLoginResult(cmd, opts, endpoint, complete.Msg.GetPrincipal(), complete.Msg.GetSecret(), "", "")
}

// newLinkOidcCommand attaches an OIDC identity to the already-authenticated user. Usage:
// `mh auth link --oidc google` opens the browser to Google, the resulting (provider, subject)
// is recorded against the caller's principal so subsequent OIDC logins resolve to the same
// account.
func newLinkOidcCommand(opts *rootOptions) *cobra.Command {
	var providerSlug string
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link an OIDC identity (google|github) to the authenticated user.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(ctx.profile.APIKey) == "" {
				return errors.New("must be logged in (set METALHOST_API_KEY or run `metalhost auth login`)")
			}
			providerSlug = strings.TrimSpace(providerSlug)
			if providerSlug == "" {
				return errors.New("--oidc google|github is required")
			}
			providerName := "oidc-providers/" + providerSlug
			client, err := ctx.iamClient()
			if err != nil {
				return err
			}
			loopback, err := startLoopbackListener(cmd.Context())
			if err != nil {
				return err
			}
			defer loopback.shutdown()

			begin, err := client.LinkOidcProvider(cmd.Context(), connect.NewRequest(&iamv1.LinkOidcProviderRequest{
				ProviderName: providerName, RedirectUri: loopback.redirectURI,
			}))
			if err != nil {
				return err
			}
			if err := openBrowser(begin.Msg.GetAuthorizeUrl()); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "open this URL to continue:\n  %s\n", begin.Msg.GetAuthorizeUrl())
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Opening browser…")
			}

			code, state, err := loopback.wait(cmd.Context())
			if err != nil {
				return err
			}
			if state != begin.Msg.GetState() {
				return errors.New("state mismatch — possible CSRF; aborting")
			}
			if _, err := client.CompleteOidcLogin(cmd.Context(), connect.NewRequest(&iamv1.CompleteOidcLoginRequest{
				ProviderName: providerName, Code: code, State: state, RedirectUri: loopback.redirectURI,
			})); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Linked %s to %s.\n", providerName, ctx.profile.Endpoint)
			return nil
		},
	}
	cmd.Flags().StringVar(&providerSlug, "oidc", "", "provider slug (google|github)")
	return cmd
}

// saveLoginResult persists the freshly-minted API key + identity hints to the active config
// profile (creates a "default" profile when none is selected). Mirrors what `auth login
// --api-key` does today; also bumps the profile's organization/project pointers when the
// caller has them.
func saveLoginResult(cmd *cobra.Command, opts *rootOptions, endpoint, principal, secret, orgName, projectName string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("server returned no API key secret")
	}
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(opts.profile)
	if name == "" {
		name = strings.TrimSpace(cfg.CurrentProfile)
	}
	if name == "" {
		name = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*config.Profile{}
	}
	prof, ok := cfg.Profiles[name]
	if !ok {
		prof = &config.Profile{}
		cfg.Profiles[name] = prof
	}
	prof.Endpoint = endpoint
	prof.APIKey = secret
	if strings.TrimSpace(orgName) != "" {
		prof.Organization = orgName
	}
	if strings.TrimSpace(projectName) != "" {
		prof.Project = projectName
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = name
	}
	if err := config.Save(opts.configPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s. Credentials saved to profile %q.\n", principal, name)
	return nil
}

// iamClientForEndpoint builds an unauthenticated IAM client. Used for signup / verify / login
// where there's no API key yet — the shared commandContext path requires one.
func iamClientForEndpoint(endpoint, userAgent string) iamv1connect.IamServiceClient {
	httpClient := &http.Client{
		Transport: metalhost.Config{UserAgent: userAgent}.RoundTripper(http.DefaultTransport),
	}
	cfg := metalhost.Config{Endpoint: endpoint, HTTPClient: httpClient, UserAgent: userAgent}
	return iamv1connect.NewIamServiceClient(cfg.Client(), cfg.BaseURL())
}

// loopback bundles the localhost listener + the captured (code, state) so loopback OIDC flows
// can be expressed as start → wait → shutdown without leaking goroutines.
type loopback struct {
	listener    net.Listener
	server      *http.Server
	redirectURI string
	resultCh    chan loopbackResult
}

type loopbackResult struct {
	code, state string
	err         error
}

func startLoopbackListener(ctx context.Context) (*loopback, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("loopback listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	lb := &loopback{
		listener:    ln,
		redirectURI: fmt.Sprintf("http://127.0.0.1:%d/callback", port),
		resultCh:    make(chan loopbackResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		code := params.Get("code")
		state := params.Get("state")
		errParam := params.Get("error")
		if errParam != "" {
			lb.resultCh <- loopbackResult{err: fmt.Errorf("provider returned error: %s — %s", errParam, params.Get("error_description"))}
			http.Error(w, "authentication failed: "+errParam, http.StatusBadRequest)
			return
		}
		if code == "" || state == "" {
			lb.resultCh <- loopbackResult{err: errors.New("provider redirect missing code or state")}
			http.Error(w, "missing code/state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body style="font-family:system-ui;text-align:center;padding:4em">
<h2>Signed in.</h2><p>You can close this tab and return to your terminal.</p>
</body></html>`))
		lb.resultCh <- loopbackResult{code: code, state: state}
	})
	lb.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = lb.server.Serve(ln) }()
	_ = ctx
	return lb, nil
}

func (l *loopback) wait(ctx context.Context) (string, string, error) {
	select {
	case res := <-l.resultCh:
		return res.code, res.state, res.err
	case <-ctx.Done():
		return "", "", ctx.Err()
	case <-time.After(loopbackTimeout):
		return "", "", errors.New("timed out waiting for browser redirect")
	}
}

func (l *loopback) shutdown() {
	if l == nil || l.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = l.server.Shutdown(ctx)
}

// promptReader is the shared bufio.Reader used by interactive prompts within a single command
// invocation. We MUST share one reader across prompts: each bufio.Reader has its own internal
// buffer, so creating one per prompt drops piped lines after the first into the discarded
// buffer. Lazily initialized per command via newPromptReader.
type promptReader struct {
	r *bufio.Reader
	w *cobra.Command
}

func newPromptReader(cmd *cobra.Command) *promptReader {
	return &promptReader{r: bufio.NewReader(cmd.InOrStdin()), w: cmd}
}

// readLine returns one line from the shared reader, with the prompt written to stderr so
// stdout (often used for the saved-credential message) stays clean.
func (p *promptReader) readLine(prompt string) string {
	fmt.Fprint(p.w.ErrOrStderr(), prompt)
	line, _ := p.r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

// readPassword reads a password from /dev/tty without echoing, falling back to a line on the
// shared reader when stdin isn't a TTY (CI / piped). Both branches print the prompt to stderr.
func (p *promptReader) readPassword(prompt string) (string, error) {
	fmt.Fprint(p.w.ErrOrStderr(), prompt)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := p.r.ReadString('\n')
		if err != nil && err.Error() != "EOF" {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(p.w.ErrOrStderr())
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

// promptIfEmpty falls back to readLine when the flag-supplied value is empty.
func (p *promptReader) promptIfEmpty(existing, prompt string) string {
	if strings.TrimSpace(existing) != "" {
		return existing
	}
	return p.readLine(prompt)
}

// openBrowser launches the default browser on the user's OS for the given URL. Best-effort —
// callers should print the URL anyway so a headless / SSH session can copy-paste it.
func openBrowser(target string) error {
	if _, err := url.Parse(target); err != nil {
		return err
	}
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", target)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		c = exec.Command("xdg-open", target)
	}
	return c.Start()
}
