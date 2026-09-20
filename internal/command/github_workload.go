package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"connectrpc.com/connect"
	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1/iamv1connect"
	"github.com/spf13/cobra"
)

func newGitHubWorkloadCommand(opts *rootOptions) *cobra.Command {
	var trust, audience string
	cmd := &cobra.Command{Use: "github --trust POLICY --audience AUDIENCE -- COMMAND [ARGS...]", Short: "Run a command with a short-lived GitHub Actions credential; never saves a profile", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("GITHUB_ACTIONS") != "true" {
			return errors.New("this command requires GitHub Actions with id-token: write; saved API keys are not a fallback")
		}
		if trust == "" || audience == "" {
			return errors.New("--trust and --audience from the approved GitHub connection are required")
		}
		endpoint := strings.TrimSpace(opts.endpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("METALHOST_ENDPOINT"))
		}
		if endpoint == "" {
			endpoint = "https://api.metalhost.net"
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("workload exchange requires an explicit HTTPS API origin without credentials or query parameters")
		}
		client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		proof, err := requestGitHubProof(cmd.Context(), client, os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"), audience)
		if err != nil {
			return err
		}
		maskGitHubSecret(cmd.OutOrStdout(), proof)
		api := iamv1connect.NewAutomationServiceClient(client, strings.TrimSuffix(endpoint, "/"), connect.WithReadMaxBytes(64<<10))
		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()
		result, err := api.ExchangeGitHubToken(ctx, connect.NewRequest(&iamv1.ExchangeGitHubTokenRequest{TrustName: trust, Assertion: proof}))
		if err != nil {
			return fmt.Errorf("workload exchange failed (%s); no saved credential was used", connect.CodeOf(err))
		}
		credential := result.Msg
		if credential.TokenType != "Bearer" || !strings.HasPrefix(credential.AccessToken, "mha_") || len(credential.AccessToken) != 47 || credential.ProjectName == "" || strings.ContainsAny(credential.AccessToken, "\r\n") || credential.ExpiresAtUnix <= time.Now().Unix() || credential.ExpiresAtUnix > time.Now().Add(16*time.Minute).Unix() {
			return errors.New("invalid workload credential response")
		}
		maskGitHubSecret(cmd.OutOrStdout(), credential.AccessToken)
		child := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
		child.Env = workloadEnvironment(os.Environ(), endpoint, credential.ProjectName, credential.AccessToken)
		child.Stdin, child.Stdout, child.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
		return child.Run()
	}}
	cmd.Flags().StringVar(&trust, "trust", "", "approved GitHub trust policy name")
	cmd.Flags().StringVar(&audience, "audience", "", "exact audience shown in the GitHub connection")
	return cmd
}

func requestGitHubProof(ctx context.Context, client *http.Client, raw, token, audience string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".actions.githubusercontent.com") || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || token == "" || strings.ContainsAny(token, "\r\n") {
		return "", errors.New("GitHub OIDC request settings are unavailable or invalid; grant this job id-token: write")
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", errors.New("could not construct GitHub OIDC request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		return "", errors.New("GitHub OIDC request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub OIDC returned HTTP %d", response.StatusCode)
	}
	rawBody, err := io.ReadAll(io.LimitReader(response.Body, 32769))
	if err != nil || len(rawBody) > 32768 {
		return "", errors.New("invalid GitHub OIDC response size")
	}
	var body struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(rawBody, &body) != nil || body.Value == "" || len(body.Value) > 16384 || strings.ContainsAny(body.Value, "\r\n") {
		return "", errors.New("invalid GitHub OIDC response")
	}
	return body.Value, nil
}

func maskGitHubSecret(w io.Writer, secret string) {
	escaped := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(secret)
	fmt.Fprintln(w, "::add-mask::"+escaped)
}

func workloadEnvironment(parent []string, endpoint, project, secret string) []string {
	out := make([]string, 0, len(parent)+3)
	for _, v := range parent {
		key, _, _ := strings.Cut(v, "=")
		switch key {
		case "METALHOST_API_KEY", "METALHOST_PROJECT", "METALHOST_ENDPOINT", "METALHOST_PROFILE", "METALHOST_ORGANIZATION", "METALHOST_REGION", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "ACTIONS_ID_TOKEN_REQUEST_URL":
			continue
		}
		out = append(out, v)
	}
	return append(out, "METALHOST_API_KEY="+secret, "METALHOST_PROJECT="+project, "METALHOST_ENDPOINT="+endpoint)
}
