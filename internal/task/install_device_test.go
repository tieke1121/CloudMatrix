package task

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/execx"
)

func TestStagedAPKRemainsTheValidatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.apk")
	initial := make([]byte, 2048)
	copy(initial, "PK\x03\x04")
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	meta, err := ValidateAPKFile(path, 4096)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := StageAPK(meta, dir)
	if err != nil {
		t.Fatal(err)
	}
	replacement := bytes.Clone(initial)
	replacement[100] = 7
	if err := os.WriteFile(path, replacement, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(staged)
	if err != nil || !bytes.Equal(data, initial) {
		t.Fatalf("staged APK no longer matches validated bytes: %v", err)
	}
	if _, err := StageAPK(meta, dir); err == nil {
		t.Fatal("modified source was accepted after validation")
	}
}

type installFunc func(context.Context, string, string, time.Duration) (*execx.Result, error)

func (f installFunc) Install(ctx context.Context, serial, path string, timeout time.Duration) (*execx.Result, error) {
	return f(ctx, serial, path, timeout)
}

func TestInstallAPKRequiresConfirmedSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const serial = "127.0.0.1:45001"
	const apk = `C:\test packages\app.apk`
	for _, tc := range []struct {
		name   string
		result execx.Result
		code   string
	}{
		{"success", execx.Result{Stdout: []byte("Performing Streamed Install\r\nSuccess\r\n")}, ""},
		{"no success marker", execx.Result{Stdout: []byte("Performing Streamed Install\n")}, "INSTALL_FAILED"},
		{"nonzero", execx.Result{ExitCode: 1, Stdout: []byte("Success")}, "INSTALL_FAILED"},
		{"already installed", execx.Result{ExitCode: 1, Stderr: []byte("INSTALL_FAILED_ALREADY_EXISTS")}, "INSTALL_FAILED_ALREADY_EXISTS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := installFunc(func(gotCtx context.Context, gotSerial, gotPath string, timeout time.Duration) (*execx.Result, error) {
				if gotCtx != ctx || gotSerial != serial || gotPath != apk || timeout != time.Second {
					t.Fatalf("install arguments were not forwarded exactly")
				}
				return &tc.result, nil
			})
			err := InstallAPKOnDevice(ctx, client, serial, apk, time.Second)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var platformErr *domain.PlatformError
			if !errors.As(err, &platformErr) || platformErr.Code != tc.code {
				t.Fatalf("expected error code %s, got %v", tc.code, err)
			}
		})
	}
}
