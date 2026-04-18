package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func skipIfNoDocker(t *testing.T) {
	t.Helper()

	// Skip on Windows - Docker tests require Linux containers and Unix mount paths
	if runtime.GOOS == "windows" {
		t.Skip("Docker sandbox tests require Linux containers, skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !IsDockerAvailable(ctx) {
		t.Skip("Docker not available, skipping Docker sandbox tests")
	}
}

func TestDefaultDockerConfig(t *testing.T) {
	cfg := DefaultDockerConfig()

	if cfg.Image != "alpine:latest" {
		t.Errorf("Image = %q, want %q", cfg.Image, "alpine:latest")
	}
	if cfg.NetworkMode != "none" {
		t.Errorf("NetworkMode = %q, want %q", cfg.NetworkMode, "none")
	}
	if cfg.MemoryLimit != 256*1024*1024 {
		t.Errorf("MemoryLimit = %d, want %d", cfg.MemoryLimit, 256*1024*1024)
	}
	if cfg.ReadonlyRootfs != true {
		t.Error("ReadonlyRootfs should be true by default")
	}
	if len(cfg.CapDrop) != 1 || cfg.CapDrop[0] != "ALL" {
		t.Errorf("CapDrop = %v, want [ALL]", cfg.CapDrop)
	}
}

func TestParseNetworkMode(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"none", "none", false},
		{"bridge", "bridge", false},
		{"host", "host", false},
		{"NONE", "none", false},
		{"BRIDGE", "bridge", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseNetworkMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNetworkMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseNetworkMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsDockerAvailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Just test that it doesn't panic
	_ = IsDockerAvailable(ctx)
}

func TestDockerSandbox_Run(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()

	// Use a permissive config for testing
	cfg := DefaultDockerConfig()
	cfg.ReadonlyRootfs = false // Allow writing for echo command

	sandbox, err := NewDockerSandbox(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("NewDockerSandbox() error = %v", err)
	}
	defer sandbox.Close()

	// Ensure image is available
	if err := sandbox.EnsureImage(ctx); err != nil {
		t.Fatalf("EnsureImage() error = %v", err)
	}

	t.Run("echo command", func(t *testing.T) {
		result, err := sandbox.Run(ctx, "echo", []string{"hello", "world"})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", result.ExitCode)
		}
		if string(result.Output) != "hello world\n" {
			t.Errorf("Output = %q, want %q", result.Output, "hello world\n")
		}
	})

	t.Run("exit code", func(t *testing.T) {
		result, err := sandbox.Run(ctx, "sh", []string{"-c", "exit 42"})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.ExitCode != 42 {
			t.Errorf("ExitCode = %d, want 42", result.ExitCode)
		}
	})
}

func TestDockerSandbox_RunShell(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()

	cfg := DefaultDockerConfig()
	cfg.ReadonlyRootfs = false

	sandbox, err := NewDockerSandbox(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("NewDockerSandbox() error = %v", err)
	}
	defer sandbox.Close()

	if err := sandbox.EnsureImage(ctx); err != nil {
		t.Fatalf("EnsureImage() error = %v", err)
	}

	result, err := sandbox.RunShell(ctx, "echo $((1+2))")
	if err != nil {
		t.Fatalf("RunShell() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Output) != "3\n" {
		t.Errorf("Output = %q, want %q", result.Output, "3\n")
	}
}

func TestDockerSandbox_Timeout(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()

	cfg := DefaultDockerConfig()
	cfg.Timeout = 500 * time.Millisecond
	cfg.ReadonlyRootfs = false

	sandbox, err := NewDockerSandbox(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("NewDockerSandbox() error = %v", err)
	}
	defer sandbox.Close()

	if err := sandbox.EnsureImage(ctx); err != nil {
		t.Fatalf("EnsureImage() error = %v", err)
	}

	_, err = sandbox.Run(ctx, "sleep", []string{"10"})
	if err == nil {
		t.Error("expected timeout error")
	}
	var execErr *ExecutionError
	if !errors.As(err, &execErr) {
		t.Errorf("expected ExecutionError, got %T", err)
	} else if execErr.Kind != "timeout" {
		t.Errorf("Kind = %q, want %q", execErr.Kind, "timeout")
	}
}

func TestDockerSandbox_WithAppLevelPermissions(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()

	// Create temp directory for mounts
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	// Use 0644 so the file is readable inside the Docker container
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil { //nolint:gosec // G306: test file needs to be readable in container
		t.Fatal(err)
	}

	// App-level config that allows only specific commands
	appConfig := &Config{
		Capabilities:    []Capability{CapExecRun, CapFSRead},
		AllowedCommands: []string{"cat", "echo"},
		AllowedPaths:    []string{tmpDir},
	}

	dockerCfg := DefaultDockerConfig()
	dockerCfg.ReadonlyRootfs = false
	dockerCfg.Mounts = []DockerMount{
		{HostPath: tmpDir, ContainerPath: "/data", ReadOnly: true},
	}

	sandbox, err := NewDockerSandbox(ctx, dockerCfg, appConfig)
	if err != nil {
		t.Fatalf("NewDockerSandbox() error = %v", err)
	}
	defer sandbox.Close()

	if err := sandbox.EnsureImage(ctx); err != nil {
		t.Fatalf("EnsureImage() error = %v", err)
	}

	t.Run("allowed command succeeds", func(t *testing.T) {
		result, err := sandbox.Run(ctx, "cat", []string{"/data/test.txt"})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if string(result.Output) != "test content" {
			t.Errorf("Output = %q, want %q", result.Output, "test content")
		}
	})

	t.Run("disallowed command fails", func(t *testing.T) {
		_, err := sandbox.Run(ctx, "ls", []string{"/data"})
		if err == nil {
			t.Error("expected error for disallowed command")
		}
	})
}

func TestDockerSandbox_RunWithStdin(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()

	cfg := DefaultDockerConfig()
	cfg.ReadonlyRootfs = false

	sandbox, err := NewDockerSandbox(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("NewDockerSandbox() error = %v", err)
	}
	defer sandbox.Close()

	if err := sandbox.EnsureImage(ctx); err != nil {
		t.Fatalf("EnsureImage() error = %v", err)
	}

	stdin := []byte("hello from stdin\n")
	result, err := sandbox.RunWithStdin(ctx, stdin, "cat", nil)
	if err != nil {
		t.Fatalf("RunWithStdin() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Output) != string(stdin) {
		t.Errorf("Output = %q, want %q", result.Output, stdin)
	}
}
