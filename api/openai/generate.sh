#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "Generating ogen server code..."

# Generate server code
go run github.com/ogen-go/ogen/cmd/ogen@v1.22.0 \
     --package ogen \
     --target internal/ogen \
     --clean \
     --config ogen.yml \
     openapi/openai-minimal.yaml

echo "Applying ogen-tools post-processing..."

# Apply ogen-fixnull if needed
if [ -f internal/ogen/oas_json_gen.go ]; then
    go run github.com/plexusone/ogen-tools/cmd/ogen-fixnull@latest internal/ogen/oas_json_gen.go || true
fi

# Apply ogen-fixerror if needed
if [ -f internal/ogen/oas_response_decoders_gen.go ]; then
    go run github.com/plexusone/ogen-tools/cmd/ogen-fixerror@latest internal/ogen/oas_response_decoders_gen.go || true
fi

echo "Running go mod tidy..."
cd ../..
go mod tidy

echo "Verifying build..."
go build ./...

echo "Done!"
