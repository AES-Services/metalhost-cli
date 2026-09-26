package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	outputfmt "github.com/AES-Services/metalhost-cli/internal/output"
	"github.com/AES-Services/metalhost-sdk/metalhost"
	"github.com/spf13/cobra"
)

func newMonitoringConfigCommand(opts *rootOptions) *cobra.Command {
	var target, keyFile string
	cmd := &cobra.Command{Use: "config", Short: "Print credential-free Grafana or Prometheus configuration", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		endpoints, err := (metalhost.Config{Endpoint: cc.profile.Endpoint}).MonitoringEndpoints(cc.profile.Project)
		if err != nil {
			return err
		}
		quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
		switch target {
		case "grafana":
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "apiVersion: 1\ndatasources:\n  - name: Metalhost\n    type: prometheus\n    access: proxy\n    url: %s\n    jsonData:\n      httpMethod: POST\n      timeInterval: 30s\n      queryTimeout: 10s\n      prometheusType: Mimir\n      prometheusVersion: 3.2.1\n      manageAlerts: false\n      allowAsRecordingRulesTarget: false\n      disableRecordingRules: true\n      httpHeaderName1: Authorization\n    secureJsonData:\n      httpHeaderValue1: 'Bearer $METALHOST_METRICS_KEY'\n", quote(endpoints.Prometheus))
		case "prometheus":
			if strings.TrimSpace(keyFile) == "" {
				return errors.New("--credentials-file is required")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "scrape_configs:\n  - job_name: metalhost\n    scrape_interval: 60s\n    scrape_timeout: 10s\n    honor_timestamps: true\n    authorization:\n      credentials_file: %s\n    http_sd_configs:\n      - url: %s\n        refresh_interval: 60s\n        authorization:\n          credentials_file: %s\n", quote(keyFile), quote(endpoints.ScrapeTargets), quote(keyFile))
		default:
			return errors.New("--target must be grafana or prometheus")
		}
		return err
	}}
	cmd.Flags().StringVar(&target, "target", "grafana", "grafana or prometheus")
	cmd.Flags().StringVar(&keyFile, "credentials-file", "/etc/prometheus/secrets/metalhost-metrics.key", "credential path on the Prometheus server (contents are never read)")
	return cmd
}

func newPromQLCommand(opts *rootOptions) *cobra.Command {
	var start, end, step string
	cmd := &cobra.Command{Use: "promql QUERY", Short: "Run a tenant-isolated hosted PromQL query", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		cfg, err := cc.sdkConfig()
		if err != nil {
			return err
		}
		endpoints, err := cfg.MonitoringEndpoints(cc.profile.Project)
		if err != nil {
			return err
		}
		values := url.Values{"query": {args[0]}}
		path := "/api/v1/query"
		if len(args[0]) == 0 || len(args[0]) > 32768 {
			return errors.New("query must contain 1–32768 bytes")
		}
		if start != "" || end != "" {
			from, fromErr := time.Parse(time.RFC3339, start)
			to, toErr := time.Parse(time.RFC3339, end)
			interval, stepErr := time.ParseDuration(step)
			if fromErr != nil || toErr != nil || stepErr != nil || !from.Before(to) || to.Sub(from) > 7*24*time.Hour || interval < 30*time.Second {
				return errors.New("range requires RFC3339 --start and --end within seven days and --step of at least 30s")
			}
			values.Set("start", strconv.FormatInt(from.Unix(), 10))
			values.Set("end", strconv.FormatInt(to.Unix(), 10))
			values.Set("step", strconv.FormatFloat(interval.Seconds(), 'f', -1, 64))
			path = "/api/v1/query_range"
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoints.Prometheus+path+"?"+values.Encode(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		client := *cfg.Client()
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err := client.Do(req)
		if err != nil {
			return errors.New("monitoring query unavailable; check connectivity and credentials")
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
		if err != nil || len(raw) > 4<<20 {
			return errors.New("monitoring response incomplete or exceeds 4 MiB")
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("monitoring query returned HTTP %d; check permissions, query limits and service health", response.StatusCode)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil || body["status"] != "success" {
			return errors.New("monitoring query returned an invalid or unsuccessful response")
		}
		return outputfmt.Write(cmd.OutOrStdout(), cc.profile.Format, body)
	}}
	cmd.Flags().StringVar(&start, "start", "", "range start, RFC3339")
	cmd.Flags().StringVar(&end, "end", "", "range end, RFC3339")
	cmd.Flags().StringVar(&step, "step", "30s", "range step, minimum 30s")
	return cmd
}
