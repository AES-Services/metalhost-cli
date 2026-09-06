package command

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

type backupCall struct {
	path string
	key  string
	body map[string]any
}

func backupCommandTest(t *testing.T, args []string, responses ...string) ([]backupCall, error) {
	t.Helper()
	for _, key := range []string{"METALHOST_PROFILE", "METALHOST_API_KEY", "METALHOST_ENDPOINT", "METALHOST_ORGANIZATION", "METALHOST_PROJECT", "METALHOST_REGION", "METALHOST_FORMAT"} {
		t.Setenv(key, "")
	}
	calls := make(chan backupCall, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) != 2 {
			http.Error(w, "bad RPC path", 400)
			return
		}
		method, err := findMethod(parts[0], parts[1])
		if err != nil {
			t.Error(err)
			http.Error(w, "unknown method", 400)
			return
		}
		wire, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		input := dynamicpb.NewMessage(method.Input())
		if err := proto.Unmarshal(wire, input); err != nil {
			t.Error(err)
			return
		}
		encoded, err := protojson.Marshal(input)
		if err != nil {
			t.Error(err)
			return
		}
		var body map[string]any
		if err := json.Unmarshal(encoded, &body); err != nil {
			t.Error(err)
		}
		calls <- backupCall{r.URL.Path, r.Header.Get("Idempotency-Key"), body}
		response := "{}"
		if len(responses) > 0 {
			response, responses = responses[0], responses[1:]
		}
		output := dynamicpb.NewMessage(method.Output())
		if err := protojson.Unmarshal([]byte(response), output); err != nil {
			t.Error(err)
			return
		}
		wire, err = proto.Marshal(output)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/proto")
		_, _ = w.Write(wire)
	}))
	defer server.Close()
	cmd := NewRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(append([]string{"--config", t.TempDir() + "/config.yaml", "--endpoint", server.URL, "--project", "projects/p", "--format", "json"}, args...))
	err := cmd.Execute()
	var result []backupCall
	for len(calls) > 0 {
		result = append(result, <-calls)
	}
	return result, err
}

func TestBackupCommandsSendStableKeys(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         []string
		path         string
		field, value string
	}{
		{"disk", []string{"disk", "backup", "create", "projects/p/disks/d", "--idempotency-key", "retry-1"}, "/aes.storage.v1.StorageService/CreateSnapshot", "sourceDisk", "projects/p/disks/d"},
		{"vm", []string{"vm", "backup", "create", "--vm", "virtual-machines/v", "--display-name", "save", "--idempotency-key", "retry-1"}, "/aes.compute.v1.ComputeService/SnapshotVirtualMachine", "vmName", "virtual-machines/v"},
		{"restore-vm", []string{"vm", "from-backup", "--snapshot", "vm-snapshots/b", "--display-name", "restored", "--idempotency-key", "retry-1"}, "/aes.compute.v1.ComputeService/CreateVirtualMachineFromBackup", "vmSnapshotName", "vm-snapshots/b"},
		{"restore-disk", []string{"disk", "create", "--from-backup", "disk-snapshots/b", "--id", "restored", "--size-gib", "10", "--idempotency-key", "retry-1"}, "/aes.storage.v1.StorageService/CreateDisk", "fromSnapshot", "disk-snapshots/b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := backupCommandTest(t, tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 {
				t.Fatalf("calls = %d", len(calls))
			}
			c := calls[0]
			if c.path != tc.path || c.key != "retry-1" || c.body[tc.field] != tc.value {
				t.Fatalf("unexpected call: %+v", c)
			}
			if tc.name == "restore-disk" && c.body["diskId"] != "restored" {
				t.Fatalf("disk ID lost: %+v", c.body)
			}
		})
	}
}

func TestBackupScheduleCreateAndPatch(t *testing.T) {
	calls, err := backupCommandTest(t, []string{"storage", "backup-schedule", "create", "--cadence", "weekly", "--hour-utc", "3", "--weekday-utc", "4", "--keep", "2", "--idempotency-key", "schedule-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	schedule := calls[0].body["schedule"].(map[string]any)
	for field, want := range map[string]any{"projectName": "projects/p", "target": "projects/p", "cadence": "WEEKLY", "hourUtc": float64(3), "weekdayUtc": float64(4), "retentionCount": float64(2), "enabled": true} {
		if schedule[field] != want {
			t.Errorf("%s = %v, want %v", field, schedule[field], want)
		}
	}
	if calls[0].key != "schedule-1" {
		t.Fatal("missing schedule key")
	}
	for _, tc := range []struct{ flag, value, mask string }{{"enabled", "false", "enabled"}, {"keep", "4", "retentionCount"}, {"hour-utc", "0", "hourUtc"}} {
		t.Run(tc.flag, func(t *testing.T) {
			calls, err := backupCommandTest(t, []string{"storage", "backup-schedule", "update", "snapshot-schedules/s", "--" + tc.flag + "=" + tc.value})
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 || calls[0].body["updateMask"] != tc.mask {
				t.Fatalf("unexpected patch: %+v", calls)
			}
		})
	}
}

func TestBackupPaginationAndDelete(t *testing.T) {
	for _, tc := range []struct {
		args        []string
		field, path string
	}{
		{[]string{"disk", "backup", "list", "--disk", "projects/p/disks/d", "--all"}, "diskSnapshots", "/aes.storage.v1.StorageService/ListSnapshots"},
		{[]string{"storage", "backup-schedule", "list", "--all"}, "schedules", "/aes.storage.v1.StorageService/ListSnapshotSchedules"},
	} {
		calls, err := backupCommandTest(t, tc.args, `{"`+tc.field+`":[{"name":"first"}],"nextPageToken":"next"}`, `{"`+tc.field+`":[{"name":"second"}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if len(calls) != 2 || calls[1].body["pageToken"] != "next" || calls[1].path != tc.path || calls[1].body["projectName"] != "projects/p" {
			t.Fatalf("unexpected pages: %+v", calls)
		}
		if !reflect.DeepEqual(calls[0].body["sourceDisk"], calls[1].body["sourceDisk"]) {
			t.Fatal("source filter lost")
		}
	}
	calls, err := backupCommandTest(t, []string{"disk", "backup", "delete", "disk-snapshots/b", "--yes"})
	if err != nil || len(calls) != 1 || calls[0].path != "/aes.storage.v1.StorageService/DeleteSnapshot" {
		t.Fatalf("delete: %+v, %v", calls, err)
	}
}

func TestBackupCommandsRejectInvalidInputWithoutRPC(t *testing.T) {
	for _, args := range [][]string{
		{"storage", "backup-schedule", "update", "snapshot-schedules/s"},
		{"storage", "backup-schedule", "create", "--keep", "0"},
		{"storage", "backup-schedule", "create", "--hour-utc", "24"},
		{"storage", "backup-schedule", "create", "--cadence", "never"},
		{"disk", "create", "--from-backup", "disk-snapshots/b"},
		{"disk", "create", "--id", "d", "--from-backup", "disk-snapshots/b", "--from-image-url", "https://example.com/image"},
	} {
		calls, err := backupCommandTest(t, args)
		if err == nil || len(calls) != 0 {
			t.Fatalf("args %v: calls=%+v err=%v", args, calls, err)
		}
	}
}
