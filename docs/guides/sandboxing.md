# Sandboxing

OmniAgent provides layered security for tool execution through capability-based permissions and runtime isolation.

## Overview

Tools can be sandboxed at multiple levels:

1. **App-Level Permissions** - Capability-based access control
2. **WASM Isolation** - Lightweight sandboxing via wazero
3. **Docker Isolation** - Full container isolation

## App-Level Permissions

### Capabilities

Tools request specific capabilities:

| Capability | Description |
|------------|-------------|
| `fs_read` | Read files from allowed paths |
| `fs_write` | Write files to allowed paths |
| `net_http` | Make HTTP requests to allowed hosts |
| `exec_run` | Execute allowed commands |

### Configuration

```go
config := sandbox.Config{
    Capabilities: []sandbox.Capability{
        sandbox.CapFSRead,
        sandbox.CapNetHTTP,
    },
    AllowedPaths: []string{"/tmp/data", "/home/user/docs"},
    AllowedHosts: []string{"api.example.com"},
    AllowedCommands: []string{"ls", "cat"},
}
```

## WASM Sandbox

For lightweight isolation, tools can run in a WASM sandbox using [wazero](https://github.com/tetratelabs/wazero).

### Features

- Memory limits
- Timeout enforcement
- No network access by default
- Restricted file system access

### Usage

```go
runtime, err := sandbox.NewRuntime(ctx, sandbox.Config{
    Capabilities:  []sandbox.Capability{sandbox.CapFSRead},
    MemoryLimitMB: 16,
    Timeout:       30 * time.Second,
    AllowedPaths:  []string{"/tmp/data"},
})

result, err := runtime.Run(ctx, wasmModule, args)
```

### Host Functions

The WASM sandbox exposes controlled host functions:

| Function | Capability Required | Description |
|----------|---------------------|-------------|
| `fs_read` | `CapFSRead` | Read file contents |
| `fs_write` | `CapFSWrite` | Write file contents |
| `http_fetch` | `CapNetHTTP` | Make HTTP requests |
| `exec_run` | `CapExecRun` | Execute commands |

## Docker Sandbox

For OS-level isolation, tools can run inside Docker containers.

### Features

- Full process isolation
- Network restrictions
- Capability dropping
- Read-only mounts

### Usage

```go
sandbox, err := sandbox.NewDockerSandbox(ctx, sandbox.DockerConfig{
    Image:       "alpine:latest",
    NetworkMode: "none",           // No network access
    CapDrop:     []string{"ALL"},  // Drop all capabilities
    Mounts: []sandbox.DockerMount{
        {
            HostPath:      "/tmp/data",
            ContainerPath: "/data",
            ReadOnly:      true,
        },
    },
}, &appConfig)

result, err := sandbox.Run(ctx, "cat", []string{"/data/file.txt"})
```

### Network Modes

| Mode | Description |
|------|-------------|
| `none` | No network access |
| `bridge` | Isolated network with NAT |
| `host` | Full host network access (not recommended) |

### Security Hardening

```go
config := sandbox.DockerConfig{
    Image:       "alpine:latest",
    NetworkMode: "none",
    CapDrop:     []string{"ALL"},
    ReadOnlyRootfs: true,
    NoNewPrivileges: true,
}
```

### GPU Passthrough

For GPU-accelerated workloads (ML inference, rendering), enable NVIDIA GPU passthrough:

```go
sandbox, err := sandbox.NewDockerSandbox(ctx, sandbox.DockerConfig{
    Image: "nvidia/cuda:12.0-base",
    GPU: &sandbox.GPUConfig{
        Enabled:      true,
        DeviceIDs:    []string{"0", "1"},  // Specific GPUs
        Capabilities: []string{"compute", "utility"},
        Driver:       "nvidia",
    },
})
```

#### GPU Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `Enabled` | bool | Enable GPU passthrough |
| `DeviceIDs` | []string | GPU device IDs (empty = all) |
| `Capabilities` | []string | GPU capabilities to enable |
| `Driver` | string | GPU driver (default: `nvidia`) |
| `Count` | int | Number of GPUs (-1 = all, 0 = none) |

#### Capabilities

| Capability | Description |
|------------|-------------|
| `compute` | CUDA/OpenCL compute |
| `utility` | nvidia-smi and management |
| `graphics` | OpenGL/Vulkan rendering |
| `video` | Video encoding/decoding |
| `display` | Display output |

#### Examples

Use all available GPUs:

```go
GPU: &sandbox.GPUConfig{
    Enabled: true,
    Count:   -1,  // All GPUs
}
```

Use specific GPUs by ID:

```go
GPU: &sandbox.GPUConfig{
    Enabled:   true,
    DeviceIDs: []string{"GPU-abc123", "GPU-def456"},
}
```

ML inference workload:

```go
GPU: &sandbox.GPUConfig{
    Enabled:      true,
    Count:        1,
    Capabilities: []string{"compute"},
}
```

!!! note "Requirements"
    GPU passthrough requires:

    - NVIDIA GPU with supported drivers
    - [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html) installed
    - Docker configured with nvidia runtime

## Best Practices

### Principle of Least Privilege

Only grant the minimum capabilities required:

```go
// Bad: Too permissive
config := sandbox.Config{
    Capabilities: []sandbox.Capability{
        sandbox.CapFSRead,
        sandbox.CapFSWrite,
        sandbox.CapNetHTTP,
        sandbox.CapExecRun,
    },
}

// Good: Minimal permissions
config := sandbox.Config{
    Capabilities: []sandbox.Capability{sandbox.CapFSRead},
    AllowedPaths: []string{"/tmp/readonly"},
}
```

### Path Restrictions

Always restrict file access to specific directories:

```go
config := sandbox.Config{
    AllowedPaths: []string{
        "/tmp/workspace",
        "/home/user/data",
    },
}
```

### Command Allowlists

Only allow specific commands for exec:

```go
config := sandbox.Config{
    AllowedCommands: []string{"ls", "cat", "grep"},
}
```

### Timeouts

Always set reasonable timeouts:

```go
config := sandbox.Config{
    Timeout: 30 * time.Second,
}
```

## Choosing a Sandbox

| Use Case | Recommended |
|----------|-------------|
| Simple file operations | App-level permissions |
| Untrusted code execution | WASM sandbox |
| Complex tools with dependencies | Docker sandbox |
| Maximum isolation | Docker with `none` network |
