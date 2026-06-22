# Deployment

Deploy OmniAgent-based applications to cloud infrastructure.

## Overview

OmniAgent applications can be deployed as:

1. **Standalone binary** - Direct deployment to VMs
2. **Container** - Docker deployment to container services
3. **Managed service** - Using OmniDeploy for automated infrastructure

This guide covers containerized deployment using [OmniDeploy](https://github.com/plexusone/omnideploy).

## Deployment Checklist

Follow these steps to deploy your OmniAgent application to AWS LightSail.

### Prerequisites

Before starting, ensure you have:

- [ ] Go 1.24+ installed
- [ ] Docker installed and running
- [ ] AWS CLI configured
- [ ] GitHub account (for container registry)

### Step 1: Prepare Deployment Files

Ensure your project has these files:

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage container build |
| `deploy.yaml` | OmniDeploy configuration |
| `.github/workflows/build.yaml` | CI container build (optional) |

### Step 2: Commit and Push to GitHub

```bash
# Add deployment files
git add Dockerfile deploy.yaml .github/

# Commit
git commit -m "build: add Docker and OmniDeploy deployment configuration"

# Push to trigger CI build
git push origin main
```

### Step 3: Build Container Image

**Option A: Via GitHub Actions (recommended)**

Pushing to GitHub triggers the build workflow automatically. The image is pushed to `ghcr.io/<owner>/<repo>:latest`.

**Option B: Build locally**

```bash
# Start Docker if not running
open -a Docker  # macOS

# Build
docker build -t ghcr.io/<owner>/<repo>:latest .

# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u <username> --password-stdin

# Push
docker push ghcr.io/<owner>/<repo>:latest
```

### Step 4: Install OmniDeploy

```bash
go install github.com/plexusone/omnideploy/cmd/omnideploy@latest
```

Verify installation:

```bash
omnideploy --version
```

### Step 5: Configure AWS Credentials

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
```

Or use AWS CLI profiles:

```bash
export AWS_PROFILE="your-profile"
```

### Step 6: Store Secrets in AWS SSM (Recommended)

Store API keys securely instead of in environment variables:

```bash
# Anthropic API key
aws ssm put-parameter \
    --name "/<app-name>/anthropic-api-key" \
    --value "sk-ant-..." \
    --type SecureString

# Other API keys
aws ssm put-parameter \
    --name "/<app-name>/other-api-key" \
    --value "..." \
    --type SecureString
```

Update `deploy.yaml` to reference secrets:

```yaml
secrets:
  - name: ANTHROPIC_API_KEY
    source: ssm:/<app-name>/anthropic-api-key
```

### Step 7: Preview Deployment

```bash
omnideploy preview \
    --config deploy.yaml \
    --target lightsail \
    --backend pulumi
```

Review the resources that will be created.

### Step 8: Deploy

```bash
omnideploy up \
    --config deploy.yaml \
    --target lightsail \
    --backend pulumi \
    --yes
```

The deployment outputs the service URL.

### Step 9: Verify Deployment

```bash
# Health check
curl https://<service-url>/health

# Test chat endpoint
curl https://<service-url>/openai/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"default","messages":[{"role":"user","content":"Hello"}]}'
```

### Step 10: Monitor and Maintain

```bash
# View logs (via AWS Console or CLI)
aws lightsail get-container-log \
    --service-name <app-name> \
    --container-name <app-name>

# Update deployment (after pushing new image)
omnideploy up --config deploy.yaml --target lightsail --backend pulumi --yes

# Destroy when done
omnideploy destroy --stack <app-name> --yes
```

## Building a Custom Agent

Before deployment, you typically create a custom agent that bundles compiled skills with OmniAgent.

### Project Structure

```
my-agent/
├── cmd/
│   └── agent/
│       └── main.go       # Entry point with compiled skills
├── config/
│   └── config.yaml       # Default configuration
├── Dockerfile            # Container build
├── deploy.yaml           # OmniDeploy configuration
├── go.mod
└── go.sum
```

### Entry Point

Create `cmd/agent/main.go`:

```go
package main

import (
    "log"
    "os"

    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/cmd/omniagent/commands"
    "github.com/plexusone/omnistorage/kvs/sqlite"

    // Import your compiled skills
    myskill "github.com/example/my-skill/omniagent/skill"
)

func main() {
    // Configure storage
    storagePath := os.Getenv("STORAGE_PATH")
    if storagePath != "" {
        store, err := sqlite.New(sqlite.Config{Path: storagePath})
        if err != nil {
            log.Fatalf("failed to create storage: %v", err)
        }
        commands.RegisterAgentOption(agent.WithStorage(store))
    }

    // Register compiled skills
    skill, err := myskill.New(myskill.Config{
        APIKey: os.Getenv("MY_SKILL_API_KEY"),
    })
    if err != nil {
        log.Fatalf("failed to create skill: %v", err)
    }
    commands.RegisterAgentOption(agent.WithCompiledSkill(skill))

    // Run the standard OmniAgent CLI
    if err := commands.Execute(); err != nil {
        log.Fatalf("error: %v", err)
    }
}
```

### Configuration

Create `config/config.yaml`:

```yaml
agent:
  name: my-agent
  greeting: "Hello, I'm My Agent. How can I help?"

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514

gateway:
  enabled: true
  addr: ":8080"

skills:
  enabled: true
```

## Containerization

### Dockerfile

Create a multi-stage Dockerfile for minimal image size:

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for private dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/my-agent \
    ./cmd/agent

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN adduser -D -u 1000 omniagent
RUN mkdir -p /data /opt/omniagent && chown -R omniagent:omniagent /data /opt/omniagent

WORKDIR /opt/omniagent

# Copy binary and config
COPY --from=builder /app/my-agent /opt/omniagent/my-agent
COPY --from=builder /app/config/config.yaml /opt/omniagent/config.yaml

USER omniagent

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENV ADDR=":8080" \
    STORAGE_PATH="/data/omniagent.db"

ENTRYPOINT ["/opt/omniagent/my-agent"]
CMD ["gateway", "run"]
```

### Build Locally

```bash
# Build image
docker build -t my-agent:latest .

# Test locally
docker run -p 8080:8080 \
    -e ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
    my-agent:latest
```

## OmniDeploy Configuration

[OmniDeploy](https://github.com/plexusone/omnideploy) provides declarative deployment to multiple cloud targets.

### Installation

```bash
go install github.com/plexusone/omnideploy/cmd/omnideploy@latest
```

### Configuration File

Create `deploy.yaml`:

```yaml
name: my-agent
version: "1.0.0"
region: us-east-1

container:
  image: ghcr.io/example/my-agent:latest
  ports:
    - container_port: 8080
      protocol: HTTP
      name: api
  health_check:
    path: /health
    interval: 30s
    timeout: 5s
    healthy_threshold: 2
    unhealthy_threshold: 3

service:
  replicas: 1
  public: true

resources:
  # LightSail sizes: nano, micro, small, medium, large, xlarge
  size: small

environment:
  ADDR: ":8080"
  STORAGE_PATH: "/data/omniagent.db"
  LLM_PROVIDER: "anthropic"
  LLM_MODEL: "claude-sonnet-4-20250514"

tags:
  app: my-agent
  environment: production
  managed-by: omnideploy
```

### Secrets Management

For sensitive values like API keys, use AWS Secrets Manager:

```yaml
secrets:
  - name: ANTHROPIC_API_KEY
    source: ssm:/my-agent/anthropic-api-key
  - name: MY_SKILL_API_KEY
    source: ssm:/my-agent/skill-api-key
```

Store secrets in AWS:

```bash
aws ssm put-parameter \
    --name "/my-agent/anthropic-api-key" \
    --value "sk-ant-..." \
    --type SecureString
```

## Deploying to AWS LightSail

### Prerequisites

1. AWS credentials configured
2. Container image pushed to registry
3. OmniDeploy installed

### Deploy

```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export AWS_REGION="us-east-1"

# Preview deployment
omnideploy preview --config deploy.yaml --target lightsail --backend pulumi

# Deploy
omnideploy up --config deploy.yaml --target lightsail --backend pulumi --yes
```

### Verify

```bash
# Get service URL from deployment output
curl https://my-agent.xxxxx.us-east-1.cs.amazonlightsail.com/health

# Test chat endpoint
curl https://my-agent.xxxxx.us-east-1.cs.amazonlightsail.com/openai/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"my-agent","messages":[{"role":"user","content":"Hello"}]}'
```

### Destroy

```bash
omnideploy destroy --stack my-agent --yes
```

## CI/CD with GitHub Actions

### Container Build Workflow

Create `.github/workflows/build.yaml`:

```yaml
name: Build and Push Container

on:
  push:
    branches: [main]
    tags: ['v*']
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=semver,pattern={{version}}
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Deployment Workflow

Create `.github/workflows/deploy.yaml`:

```yaml
name: Deploy to LightSail

on:
  workflow_run:
    workflows: ["Build and Push Container"]
    types: [completed]
    branches: [main]

jobs:
  deploy:
    if: ${{ github.event.workflow_run.conclusion == 'success' }}
    runs-on: ubuntu-latest
    environment: production

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Install OmniDeploy
        run: go install github.com/plexusone/omnideploy/cmd/omnideploy@latest

      - name: Deploy
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_REGION: us-east-1
        run: |
          omnideploy up \
            --config deploy.yaml \
            --target lightsail \
            --backend pulumi \
            --yes

      - name: Health Check
        run: |
          for i in {1..30}; do
            if curl -sf "${{ vars.SERVICE_URL }}/health"; then
              echo "Deployment healthy"
              exit 0
            fi
            sleep 10
          done
          echo "Health check failed"
          exit 1
```

### Required Secrets

Configure in **Settings → Secrets and variables → Actions**:

| Secret | Description |
|--------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |

Configure in **Settings → Secrets and variables → Actions → Variables**:

| Variable | Description |
|----------|-------------|
| `SERVICE_URL` | Deployed service URL for health checks |

## Deploy Script

For manual deployments, create `deploy/deploy.sh`:

```bash
#!/bin/bash
# Deploy to AWS LightSail using OmniDeploy

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/deploy.yaml"
STACK_NAME="${STACK_NAME:-my-agent}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[deploy]${NC} $1"; }
warn() { echo -e "${YELLOW}[warn]${NC} $1"; }

# Check prerequisites
check_prereqs() {
    command -v omnideploy &> /dev/null || {
        echo "Install omnideploy: go install github.com/plexusone/omnideploy/cmd/omnideploy@latest"
        exit 1
    }

    [ -z "$AWS_ACCESS_KEY_ID" ] && warn "AWS_ACCESS_KEY_ID not set"
    [ ! -f "$CONFIG_FILE" ] && { echo "Config not found: $CONFIG_FILE"; exit 1; }
}

# Build and push image
build() {
    log "Building container image..."
    cd "$PROJECT_ROOT"
    VERSION=$(git describe --tags --always 2>/dev/null || echo "latest")
    IMAGE="ghcr.io/example/my-agent:$VERSION"

    docker build -t "$IMAGE" -t "ghcr.io/example/my-agent:latest" .
    docker push "$IMAGE"
    docker push "ghcr.io/example/my-agent:latest"
    log "Image pushed: $IMAGE"
}

# Deploy
deploy() {
    log "Deploying to LightSail..."
    omnideploy up \
        --config "$CONFIG_FILE" \
        --target lightsail \
        --backend pulumi \
        --stack "$STACK_NAME" \
        --yes
}

# Preview
preview() {
    log "Previewing deployment..."
    omnideploy preview \
        --config "$CONFIG_FILE" \
        --target lightsail \
        --backend pulumi
}

# Destroy
destroy() {
    warn "This will destroy all resources in stack: $STACK_NAME"
    read -p "Are you sure? (y/N) " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]] && omnideploy destroy --stack "$STACK_NAME" --yes
}

check_prereqs

case "${1:-deploy}" in
    build)   build ;;
    preview) preview ;;
    deploy)  deploy ;;
    destroy) destroy ;;
    full)    build && deploy ;;
    *)
        echo "Usage: $0 {build|preview|deploy|destroy|full}"
        exit 1
        ;;
esac
```

## Resource Sizing

### LightSail Container Sizes

| Size | vCPU | Memory | Monthly Cost |
|------|------|--------|--------------|
| nano | 0.25 | 512MB | ~$7 |
| micro | 0.5 | 1GB | ~$10 |
| small | 1 | 2GB | ~$25 |
| medium | 2 | 4GB | ~$50 |
| large | 4 | 8GB | ~$100 |
| xlarge | 8 | 16GB | ~$200 |

### Recommendations

| Use Case | Size | Notes |
|----------|------|-------|
| Development/Testing | nano | Minimal traffic |
| Personal assistant | micro | Light usage |
| Production API | small | Moderate traffic |
| High-traffic API | medium+ | Scale replicas |

## Other Deployment Targets

OmniDeploy supports additional targets (planned):

| Target | Status | Description |
|--------|--------|-------------|
| lightsail | Available | AWS LightSail containers |
| ecs | Planned | AWS ECS/Fargate |
| kubernetes | Planned | Kubernetes clusters |
| digitalocean | Planned | DigitalOcean App Platform |

See [OmniDeploy documentation](https://github.com/plexusone/omnideploy) for details.
