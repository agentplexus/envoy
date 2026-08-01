# Bug: GET /openai/v1/models returns 404

## Summary

The `/openai/v1/models` endpoint returns `404 page not found` even though the route is registered. This breaks OpenAI-compatible clients (including the built-in Web UI) that call this endpoint to list available models.

## Reproduction

```bash
# Start omniagent server
PROVIDER=ollama MODEL=llama3:8b PORT=8090 ./my-agent

# This returns 404:
curl http://localhost:8090/openai/v1/models
# Output: 404 page not found

# But chat completions work fine:
curl http://localhost:8090/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"my-agent","messages":[{"role":"user","content":"hello"}]}'
# Output: {"id":"chatcmpl-...","choices":[...]}
```

## Root Cause

In `api/openai/server.go`, the handlers `handleOgenModels` and `handleOgenModel` forward requests directly to the ogen server:

```go
// Line 269-270
r.Get(cfg.OpenAIPrefix+"/models", s.handleOgenModels)
r.Get(cfg.OpenAIPrefix+"/models/{model}", s.handleOgenModel)

// Line 367-374
func (s *Server) handleOgenModels(w http.ResponseWriter, r *http.Request) {
    s.ogenSrv.ServeHTTP(w, r)
}

func (s *Server) handleOgenModel(w http.ResponseWriter, r *http.Request) {
    s.ogenSrv.ServeHTTP(w, r)
}
```

**The problem**: The ogen-generated server expects paths **without** the `/openai/v1` prefix (i.e., `/models`), but the request arrives with the full path `/openai/v1/models`. The ogen router doesn't match and returns 404.

From `api/openai/internal/ogen/oas_server_gen.go`:
```go
// ListModels implements listModels operation.
// GET /models   <-- expects /models, not /openai/v1/models
ListModels(ctx context.Context) (*ListModelsResponse, error)
```

## Suggested Fix

Rewrite the URL path before forwarding to ogen:

```go
// handleOgenModels handles GET /openai/v1/models through ogen
func (s *Server) handleOgenModels(w http.ResponseWriter, r *http.Request) {
    // Rewrite path for ogen which expects /models (without prefix)
    r2 := r.Clone(r.Context())
    r2.URL.Path = "/models"
    r2.RequestURI = "/models"
    s.ogenSrv.ServeHTTP(w, r2)
}

// handleOgenModel handles GET /openai/v1/models/{model} through ogen
func (s *Server) handleOgenModel(w http.ResponseWriter, r *http.Request) {
    // Rewrite path for ogen which expects /models/{model} (without prefix)
    model := chi.URLParam(r, "model")
    r2 := r.Clone(r.Context())
    r2.URL.Path = "/models/" + model
    r2.RequestURI = "/models/" + model
    s.ogenSrv.ServeHTTP(w, r2)
}
```

## Impact

- **Web UI**: Shows "Error: HTTP 404" when user sends a message (the UI calls `/models` on load or before chat)
- **OpenAI SDK clients**: Fail when listing models
- **Compatibility**: Breaks OpenAI API compatibility

## Affected Versions

- Current `main` branch (commit `938c85f`)

## Workaround

None for the Web UI. API users can skip the `/models` call and directly use `/chat/completions` if they know the model ID.

## Files to Modify

- `api/openai/server.go` (lines 367-374)

## Testing

After fix, verify:
```bash
curl http://localhost:8090/openai/v1/models
# Should return: {"object":"list","data":[{"id":"my-agent","object":"model",...}]}

curl http://localhost:8090/openai/v1/models/my-agent
# Should return: {"id":"my-agent","object":"model",...}
```
