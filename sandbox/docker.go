package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// DockerConfig configures a Docker-based sandbox.
type DockerConfig struct {
	// Image is the Docker image to use (default: "alpine:latest").
	Image string

	// Mounts defines volume mounts for filesystem access.
	Mounts []DockerMount

	// NetworkMode controls network access ("none", "bridge", "host").
	NetworkMode string

	// Memory limit in bytes (0 = unlimited).
	MemoryLimit int64

	// CPU quota (0 = unlimited, 100000 = 1 CPU).
	CPUQuota int64

	// Timeout is the maximum execution time.
	Timeout time.Duration

	// Environment variables to pass to the container.
	Env []string

	// User to run as inside the container (e.g., "nobody", "1000:1000").
	User string

	// ReadonlyRootfs makes the container's root filesystem read-only.
	ReadonlyRootfs bool

	// CapDrop lists Linux capabilities to drop (e.g., "ALL").
	CapDrop []string

	// CapAdd lists Linux capabilities to add.
	CapAdd []string

	// SecurityOpt lists security options (e.g., "no-new-privileges").
	SecurityOpt []string

	// MaxOutputBytes limits output size (default: 1MB).
	MaxOutputBytes int
}

// DockerMount defines a volume mount.
type DockerMount struct {
	// HostPath is the path on the host system.
	HostPath string

	// ContainerPath is the path inside the container.
	ContainerPath string

	// ReadOnly makes the mount read-only.
	ReadOnly bool
}

// DefaultDockerConfig returns a secure default configuration.
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Image:          "alpine:latest",
		NetworkMode:    "none",
		MemoryLimit:    256 * 1024 * 1024, // 256MB
		CPUQuota:       50000,             // 0.5 CPU
		Timeout:        30 * time.Second,
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		MaxOutputBytes: 1024 * 1024, // 1MB
	}
}

// DockerSandbox provides Docker-based isolation for command execution.
type DockerSandbox struct {
	cli    *client.Client
	config DockerConfig
	host   *HostFunctions // App-level permission checks
}

// NewDockerSandbox creates a new Docker sandbox.
func NewDockerSandbox(ctx context.Context, config DockerConfig, appConfig *Config) (*DockerSandbox, error) {
	// Create Docker client
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	// Verify Docker is accessible
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		cli.Close()
		return nil, fmt.Errorf("docker not accessible: %w", err)
	}

	var host *HostFunctions
	if appConfig != nil {
		host = NewHostFunctions(*appConfig)
	}

	return &DockerSandbox{
		cli:    cli,
		config: config,
		host:   host,
	}, nil
}

// Close releases the Docker client resources.
func (d *DockerSandbox) Close() error {
	return d.cli.Close()
}

// EnsureImage pulls the configured image if not present.
func (d *DockerSandbox) EnsureImage(ctx context.Context) error {
	// Check if image exists locally
	_, err := d.cli.ImageInspect(ctx, d.config.Image)
	if err == nil {
		return nil // Image exists
	}

	// Pull the image
	resp, err := d.cli.ImagePull(ctx, d.config.Image, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", d.config.Image, err)
	}
	defer resp.Close()

	// Consume the reader to complete the pull
	_, err = io.Copy(io.Discard, resp)
	return err
}

// Run executes a command inside a Docker container.
func (d *DockerSandbox) Run(ctx context.Context, command string, args []string) (*Result, error) {
	start := time.Now()

	// Apply app-level permission checks if configured
	if d.host != nil {
		if err := d.host.validateCommand(command); err != nil {
			return nil, err
		}
	}

	// Apply timeout
	if d.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.config.Timeout)
		defer cancel()
	}

	// Build command
	cmd := append([]string{command}, args...)

	// Convert mounts
	var mounts []mount.Mount
	for _, m := range d.config.Mounts {
		// Validate mount paths against app-level config
		if d.host != nil {
			if _, err := d.host.validatePath(m.HostPath); err != nil {
				return nil, fmt.Errorf("mount validation failed: %w", err)
			}
		}

		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.HostPath,
			Target:   m.ContainerPath,
			ReadOnly: m.ReadOnly,
		})
	}

	// Create container
	createResp, err := d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: d.config.Image,
			Cmd:   cmd,
			Env:   d.config.Env,
			User:  d.config.User,
			Tty:   false,
		},
		HostConfig: &container.HostConfig{
			NetworkMode:    container.NetworkMode(d.config.NetworkMode),
			ReadonlyRootfs: d.config.ReadonlyRootfs,
			CapDrop:        d.config.CapDrop,
			CapAdd:         d.config.CapAdd,
			SecurityOpt:    d.config.SecurityOpt,
			Mounts:         mounts,
			Resources: container.Resources{
				Memory:   d.config.MemoryLimit,
				CPUQuota: d.config.CPUQuota,
			},
			// Note: AutoRemove disabled to allow fetching logs before cleanup
		},
		NetworkingConfig: &network.NetworkingConfig{},
	})
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	containerID := createResp.ID

	// Ensure cleanup on error
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.cli.ContainerRemove(removeCtx, containerID, client.ContainerRemoveOptions{Force: true})
	}()

	// Start container
	if _, err := d.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Wait for container to finish
	waitResult := d.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	var exitCode int
	select {
	case <-ctx.Done():
		// Timeout - stop the container
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.cli.ContainerStop(stopCtx, containerID, client.ContainerStopOptions{})
		return nil, NewTimeoutError(d.config.Timeout)
	case err := <-waitResult.Error:
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				// Timeout - stop the container
				stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _ = d.cli.ContainerStop(stopCtx, containerID, client.ContainerStopOptions{})
				return nil, NewTimeoutError(d.config.Timeout)
			}
			return nil, fmt.Errorf("wait for container: %w", err)
		}
	case status := <-waitResult.Result:
		exitCode = int(status.StatusCode)
	}

	// Get container logs
	logs, err := d.cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}
	defer logs.Close()

	// Separate stdout and stderr using stdcopy
	var stdout, stderr bytes.Buffer
	maxBytes := d.config.MaxOutputBytes
	if maxBytes == 0 {
		maxBytes = 1024 * 1024 // 1MB default
	}

	stdoutWriter := &limitedWriter{w: &stdout, max: maxBytes}
	stderrWriter := &limitedWriter{w: &stderr, max: maxBytes}
	_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, logs)

	return &Result{
		Output:   stdout.Bytes(),
		Error:    stderr.Bytes(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

// RunShell executes a shell command inside a Docker container.
func (d *DockerSandbox) RunShell(ctx context.Context, shellCommand string) (*Result, error) {
	// Use sh -c to execute shell commands
	return d.Run(ctx, "sh", []string{"-c", shellCommand})
}

// RunWithStdin executes a command with stdin input.
func (d *DockerSandbox) RunWithStdin(ctx context.Context, stdin []byte, command string, args []string) (*Result, error) {
	start := time.Now()

	// Apply app-level permission checks if configured
	if d.host != nil {
		if err := d.host.validateCommand(command); err != nil {
			return nil, err
		}
	}

	// Apply timeout
	if d.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.config.Timeout)
		defer cancel()
	}

	// Build command
	cmd := append([]string{command}, args...)

	// Convert mounts
	var mounts []mount.Mount
	for _, m := range d.config.Mounts {
		if d.host != nil {
			if _, err := d.host.validatePath(m.HostPath); err != nil {
				return nil, fmt.Errorf("mount validation failed: %w", err)
			}
		}

		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.HostPath,
			Target:   m.ContainerPath,
			ReadOnly: m.ReadOnly,
		})
	}

	// Create container with stdin enabled
	createResp, err := d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:        d.config.Image,
			Cmd:          cmd,
			Env:          d.config.Env,
			User:         d.config.User,
			Tty:          false,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			OpenStdin:    true,
			StdinOnce:    true,
		},
		HostConfig: &container.HostConfig{
			NetworkMode:    container.NetworkMode(d.config.NetworkMode),
			ReadonlyRootfs: d.config.ReadonlyRootfs,
			CapDrop:        d.config.CapDrop,
			CapAdd:         d.config.CapAdd,
			SecurityOpt:    d.config.SecurityOpt,
			Mounts:         mounts,
			Resources: container.Resources{
				Memory:   d.config.MemoryLimit,
				CPUQuota: d.config.CPUQuota,
			},
			// Note: AutoRemove disabled to allow fetching logs before cleanup
		},
		NetworkingConfig: &network.NetworkingConfig{},
	})
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	containerID := createResp.ID

	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.cli.ContainerRemove(removeCtx, containerID, client.ContainerRemoveOptions{Force: true})
	}()

	// Attach to container for stdin
	attachResp, err := d.cli.ContainerAttach(ctx, containerID, client.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach to container: %w", err)
	}
	defer attachResp.Close()

	// Start container
	if _, err := d.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Write stdin
	if len(stdin) > 0 {
		if _, err := attachResp.Conn.Write(stdin); err != nil {
			return nil, fmt.Errorf("write stdin: %w", err)
		}
	}
	// Close stdin to signal EOF
	_ = attachResp.CloseWrite()

	// Read output
	var stdout, stderr bytes.Buffer
	maxBytes := d.config.MaxOutputBytes
	if maxBytes == 0 {
		maxBytes = 1024 * 1024
	}

	stdoutWriter := &limitedWriter{w: &stdout, max: maxBytes}
	stderrWriter := &limitedWriter{w: &stderr, max: maxBytes}
	_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, attachResp.Reader)

	// Wait for container
	waitResult := d.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	var exitCode int
	select {
	case err := <-waitResult.Error:
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _ = d.cli.ContainerStop(stopCtx, containerID, client.ContainerStopOptions{})
				return nil, NewTimeoutError(d.config.Timeout)
			}
			return nil, fmt.Errorf("wait for container: %w", err)
		}
	case status := <-waitResult.Result:
		exitCode = int(status.StatusCode)
	}

	return &Result{
		Output:   stdout.Bytes(),
		Error:    stderr.Bytes(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

// IsDockerAvailable checks if Docker is accessible.
func IsDockerAvailable(ctx context.Context) bool {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return false
	}
	defer cli.Close()

	_, err = cli.Ping(ctx, client.PingOptions{})
	return err == nil
}

// ParseNetworkMode validates and returns a network mode string.
func ParseNetworkMode(mode string) (string, error) {
	mode = strings.ToLower(mode)
	switch mode {
	case "none", "bridge", "host":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid network mode: %s (must be none, bridge, or host)", mode)
	}
}
