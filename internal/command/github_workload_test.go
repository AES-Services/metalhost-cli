package command

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	"google.golang.org/protobuf/proto"
)

type workloadTransport func(*http.Request) (*http.Response, error)

func (f workloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitHubProofRejectsWrongHostsAndNeverReturnsProviderSecrets(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: workloadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("secret-provider-response"))}, nil
	})}
	for _, u := range []string{"http://pipelines.actions.githubusercontent.com/token", "https://actions.githubusercontent.com.evil.invalid/token", "https://evil.invalid/token", "https://pipelines.actions.githubusercontent.com:8443/token", "https://user:pass@pipelines.actions.githubusercontent.com/token"} {
		if _, err := requestGitHubProof(context.Background(), client, u, "request-secret", "aud"); err == nil {
			t.Fatalf("accepted %s", u)
		}
	}
	if calls != 0 {
		t.Fatal("unsafe request reached transport")
	}
	_, err := requestGitHubProof(context.Background(), client, "https://pipelines.actions.githubusercontent.com/token?value=private", "request-secret", "aud")
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe provider failure %v", err)
	}
}

func TestGitHubWorkloadRunsWithoutReadingOrWritingProfiles(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://pipelines.actions.githubusercontent.com/token?api-version=2")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "request-secret")
	t.Setenv("METALHOST_API_KEY", "saved-long-lived-secret")
	t.Setenv("METALHOST_PROFILE", "must-not-load-this-profile")
	t.Setenv("METALHOST_PROJECT", "projects/old")
	t.Setenv("METALHOST_ORGANIZATION", "organizations/old")
	t.Setenv("METALHOST_ENDPOINT", "https://api.example.invalid")
	t.Setenv("MH_TEST_WORKLOAD_CHILD", "1")
	secret := "mha_" + strings.Repeat("a", 43)
	requests := 0
	old := http.DefaultTransport
	http.DefaultTransport = workloadTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		response := &http.Response{StatusCode: 200, Header: http.Header{}}
		if r.URL.Hostname() == "pipelines.actions.githubusercontent.com" {
			if r.URL.Query().Get("audience") != "urn:metalhost:staging" || r.Header.Get("Authorization") != "Bearer request-secret" {
				t.Fatal("wrong GitHub audience or request authorization")
			}
			response.Body = io.NopCloser(strings.NewReader(`{"value":"signed-assertion"}`))
			return response, nil
		}
		if r.URL.Hostname() != "api.example.invalid" || r.URL.Path != "/aes.iam.v1.AutomationService/ExchangeGitHubToken" || r.Header.Get("Authorization") != "" {
			t.Fatal("saved credentials leaked into workload exchange")
		}
		body, _ := io.ReadAll(r.Body)
		var req iamv1.ExchangeGitHubTokenRequest
		if err := proto.Unmarshal(body, &req); err != nil || req.TrustName != "github-trusts/test" || req.Assertion != "signed-assertion" {
			t.Fatal("incorrect exchange request")
		}
		wire, _ := proto.Marshal(&iamv1.ExchangeGitHubTokenResponse{AccessToken: secret, TokenType: "Bearer", ExpiresAtUnix: time.Now().Add(15 * time.Minute).Unix(), ProjectName: "projects/workload"})
		response.Header.Set("Content-Type", "application/proto")
		response.Body = io.NopCloser(bytes.NewReader(wire))
		return response, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	config := filepath.Join(t.TempDir(), "must-not-exist.yaml")
	cmd := NewRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--config", config, "auth", "github", "--trust", "github-trusts/test", "--audience", "urn:metalhost:staging", "--", os.Args[0], "-test.run=^TestGitHubWorkloadChild$"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests %d", requests)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatal("workload login wrote a profile")
	}
	if !strings.Contains(output.String(), "::add-mask::signed-assertion") || !strings.Contains(output.String(), "::add-mask::"+secret) || strings.Contains(output.String(), "saved-long-lived-secret") {
		t.Fatal("missing mask or exposed fallback secret")
	}
}

func TestGitHubWorkloadChild(t *testing.T) {
	if os.Getenv("MH_TEST_WORKLOAD_CHILD") != "1" {
		return
	}
	if os.Getenv("METALHOST_API_KEY") != "mha_"+strings.Repeat("a", 43) || os.Getenv("METALHOST_PROJECT") != "projects/workload" || os.Getenv("METALHOST_PROFILE") != "" || os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") != "" || os.Getenv("METALHOST_ORGANIZATION") != "" {
		t.Fatal("incorrect child credential boundary")
	}
}

func TestGitHubWorkloadHasNoLocalCredentialFallback(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("METALHOST_API_KEY", "saved-secret")
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"auth", "github", "--trust", "github-trusts/test", "--audience", "urn:test", "--", "false"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a fallback") {
		t.Fatalf("missing workflow silently fell back: %v", err)
	}
}
