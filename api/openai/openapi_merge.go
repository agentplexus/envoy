package openai

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

//go:embed openapi/openai-minimal.yaml
var ogenSpecFS embed.FS

// handleOpenAPIJSON serves the merged OpenAPI spec in JSON format.
func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, _ *http.Request) {
	spec, err := s.getMergedSpec()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleOpenAPIYAML serves the merged OpenAPI spec in YAML format.
func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, _ *http.Request) {
	spec, err := s.getMergedSpec()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	if err := yaml.NewEncoder(w).Encode(spec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getMergedSpec returns the merged OpenAPI spec combining ogen and Huma endpoints.
func (s *Server) getMergedSpec() (*openapi3.T, error) {
	// 1. Load the ogen input spec (OpenAI-compatible endpoints)
	ogenBytes, err := ogenSpecFS.ReadFile("openapi/openai-minimal.yaml")
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	ogenSpec, err := loader.LoadFromData(ogenBytes)
	if err != nil {
		return nil, err
	}

	// 2. Get the Huma-generated spec
	humaOpenAPI := s.humaAPI.OpenAPI()

	// 3. Convert Huma spec to openapi3.T for merging
	humaBytes, err := json.Marshal(humaOpenAPI)
	if err != nil {
		return nil, err
	}

	humaSpec, err := loader.LoadFromData(humaBytes)
	if err != nil {
		return nil, err
	}

	// 4. Merge specs: start with Huma (custom endpoints), add ogen (OpenAI endpoints)
	merged := humaSpec

	// Update info
	merged.Info.Title = "OmniAgent API"
	merged.Info.Version = "1.0.0"
	merged.Info.Description = "OpenAI-compatible API with OmniAgent extensions for agents, tools, cron jobs, usage analytics, and semantic memory."

	// Add servers
	merged.Servers = openapi3.Servers{
		{
			URL:         "http://localhost:8080",
			Description: "Local development server",
		},
		{
			URL:         "https://api.example.com",
			Description: "Production server (replace with actual URL)",
		},
	}

	// Merge ogen paths (OpenAI-compatible endpoints)
	if ogenSpec.Paths != nil {
		if merged.Paths == nil {
			merged.Paths = openapi3.NewPaths()
		}
		for path, item := range ogenSpec.Paths.Map() {
			// Add OpenAI prefix to ogen paths
			fullPath := s.config.OpenAIPrefix + path
			merged.Paths.Set(fullPath, item)
		}
	}

	// Merge ogen components
	if ogenSpec.Components != nil {
		if merged.Components == nil {
			merged.Components = &openapi3.Components{}
		}

		// Merge schemas
		if ogenSpec.Components.Schemas != nil {
			if merged.Components.Schemas == nil {
				merged.Components.Schemas = make(openapi3.Schemas)
			}
			for name, schema := range ogenSpec.Components.Schemas {
				// Only add if not already present (Huma takes precedence)
				if _, exists := merged.Components.Schemas[name]; !exists {
					merged.Components.Schemas[name] = schema
				}
			}
		}

		// Merge security schemes
		if ogenSpec.Components.SecuritySchemes != nil {
			if merged.Components.SecuritySchemes == nil {
				merged.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
			}
			for name, scheme := range ogenSpec.Components.SecuritySchemes {
				if _, exists := merged.Components.SecuritySchemes[name]; !exists {
					merged.Components.SecuritySchemes[name] = scheme
				}
			}
		}
	}

	// Add security requirement if API keys are configured
	if len(s.config.APIKeys) > 0 {
		merged.Security = openapi3.SecurityRequirements{
			{"BearerAuth": {}},
		}
	}

	// Ensure BearerAuth security scheme exists with description
	if merged.Components.SecuritySchemes == nil {
		merged.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
	}
	bearerAuth, exists := merged.Components.SecuritySchemes["BearerAuth"]
	if !exists {
		merged.Components.SecuritySchemes["BearerAuth"] = &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "Bearer token authentication. Use your API key as the token.",
			},
		}
	} else if bearerAuth.Value != nil && bearerAuth.Value.Description == "" {
		// Add description if missing
		bearerAuth.Value.Description = "Bearer token authentication. Use your API key as the token."
	}

	// Add descriptions to schemas that are missing them
	addSchemaDescriptions(merged.Components.Schemas)

	// Fix nullable fields for OpenAPI 3.1 compatibility
	fixNullableSchemas(merged.Components.Schemas)

	// Fix missing type fields
	fixMissingTypes(merged.Components.Schemas)

	// Remove redundant $schema descriptions (Huma generates identical ones)
	removeSchemaFieldDescriptions(merged.Components.Schemas)

	// Tag schemas with their naming source for validation
	tagSchemaNamingSources(merged.Components.Schemas)

	// Organize tags
	merged.Tags = openapi3.Tags{
		{Name: "Chat", Description: "Chat completion endpoints (OpenAI-compatible)"},
		{Name: "Models", Description: "Model information endpoints"},
		{Name: "Tools", Description: "Tool management endpoints"},
		{Name: "Agents", Description: "Agent management endpoints"},
		{Name: "Cron", Description: "Scheduled job endpoints"},
		{Name: "Usage", Description: "Usage analytics endpoints"},
		{Name: "Memory", Description: "Semantic memory endpoints"},
		{Name: "System", Description: "System endpoints"},
	}

	return merged, nil
}

// schemaDescriptions maps schema names to their descriptions.
var schemaDescriptions = map[string]string{
	"AgentInfo":                          "Agent configuration and metadata",
	"AgentUsage":                         "Usage statistics for an agent",
	"Axis":                               "Chart axis configuration",
	"ChartIR":                            "Chart intermediate representation for rendering",
	"ChatCompletionChoice":               "A chat completion choice",
	"ChatCompletionContentPart":          "Content part of a chat message",
	"ChatCompletionLogprobs":             "Log probability information",
	"ChatCompletionMessage":              "A chat completion message",
	"ChatCompletionMessageParam":         "Input message for chat completion",
	"ChatCompletionMessageToolCall":      "Tool call in a chat message",
	"ChatCompletionNamedToolChoice":      "Named tool choice for chat completion",
	"ChatCompletionStreamChoice":         "Streaming chat completion choice",
	"ChatCompletionStreamDelta":          "Delta content in streaming response",
	"ChatCompletionStreamOptions":        "Options for streaming chat completions",
	"ChatCompletionStreamToolCall":       "Tool call in streaming response",
	"ChatCompletionTokenLogprob":         "Token log probability",
	"ChatCompletionTool":                 "Tool definition for chat completion",
	"CloneAgentRequest":                  "Request to clone an existing agent",
	"Column":                             "Chart data column definition",
	"CompletionUsage":                    "Token usage statistics for a completion",
	"CreateAgentRequest":                 "Request to create a new agent",
	"CreateChatCompletionRequest":        "Request for chat completion",
	"CreateChatCompletionResponse":       "Response from chat completion",
	"CreateChatCompletionStreamResponse": "Streaming response from chat completion",
	"CreateCronJobRequest":               "Request to create a cron job",
	"CronActionInfo":                     "Cron job action configuration",
	"CronJobInfo":                        "Cron job configuration and status",
	"CronJobResult":                      "Result of a cron job execution",
	"CronScheduleInfo":                   "Cron job schedule information",
	"Dataset":                            "Chart dataset",
	"EnableDisableCronJobOutputBody":     "Response for enabling/disabling a cron job",
	"Encode":                             "Chart encoding configuration",
	"ErrorDetail":                        "Error detail information",
	"ErrorModel":                         "Error response model",
	"ErrorResponse":                      "Error response from the API",
	"FunctionCall":                       "Function call in chat message",
	"FunctionDefinition":                 "Function definition for tool use",
	"FunctionObject":                     "Function object for tool calls",
	"Grid":                               "Chart grid configuration",
	"HealthOutputBody":                   "Health check response",
	"ImageUrl":                           "Image URL in chat message",
	"Legend":                             "Chart legend configuration",
	"ListAgentsOutputBody":               "Response containing list of agents",
	"ListCollectionsOutputBody":          "Response containing list of memory collections",
	"ListCronJobsOutputBody":             "Response containing list of cron jobs",
	"ListMemoriesOutputBody":             "Response containing list of memories",
	"ListModelsResponse":                 "Response listing available models",
	"ListToolsOutputBody":                "Response containing list of available tools",
	"Mark":                               "Chart mark (visual element) configuration",
	"MemoryCollection":                   "A collection of semantic memories",
	"MemoryInfo":                         "Semantic memory entry",
	"MemoryRecord":                       "A stored memory record with metadata",
	"MemorySearchResult":                 "Search result from semantic memory query",
	"Model":                              "Model information",
	"ModelUsage":                         "Usage statistics by model",
	"ReloadAgentOutputBody":              "Response from reloading an agent",
	"ResponseFormat":                     "Response format specification for chat completion",
	"ResponseFormatJSONObject":           "JSON object response format",
	"ResponseFormatJSONSchema":           "JSON schema response format",
	"ResponseFormatText":                 "Text response format",
	"SearchMemoriesOutputBody":           "Response containing memory search results",
	"StoreMemoryRequest":                 "Request to store a memory",
	"Style":                              "Chart style configuration",
	"ToolInfo":                           "Tool information and parameters",
	"Tooltip":                            "Chart tooltip configuration",
	"TopLogprob":                         "Top log probability entry",
	"UpdateAgentRequest":                 "Request to update an agent",
	"UpdateCronJobRequest":               "Request to update a cron job",
	"UsageBucket":                        "Time-bucketed usage data",
	"UsageRecord":                        "Individual usage record",
	"UsageRecordsOutputBody":             "Response containing usage records",
	"UsageSummary":                       "Aggregated usage summary",
	"UsageTimeseries":                    "Time series of usage data",
}

// addSchemaDescriptions adds descriptions to schemas that are missing them.
func addSchemaDescriptions(schemas openapi3.Schemas) {
	for name, schemaRef := range schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}
		if schemaRef.Value.Description == "" {
			if desc, ok := schemaDescriptions[name]; ok {
				schemaRef.Value.Description = desc
			}
		}
	}
}

// fixNullableSchemas converts OpenAPI 3.0 nullable: true to OpenAPI 3.1 type arrays.
func fixNullableSchemas(schemas openapi3.Schemas) {
	for _, schemaRef := range schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}
		fixNullableInSchema(schemaRef.Value)
	}
}

// fixNullableInSchema recursively fixes nullable fields in a schema.
func fixNullableInSchema(schema *openapi3.Schema) {
	if schema == nil {
		return
	}

	// Fix nullable at this level - OpenAPI 3.1 uses type arrays instead of nullable
	// The kin-openapi library handles this automatically when Nullable is set,
	// but we need to clear Nullable to avoid the deprecated keyword in output
	if schema.Nullable {
		schema.Nullable = false
		// Ensure type includes null (kin-openapi should handle this)
		if schema.Type != nil && len(*schema.Type) == 1 {
			types := *schema.Type
			hasNull := false
			for _, t := range types {
				if t == "null" {
					hasNull = true
					break
				}
			}
			if !hasNull {
				*schema.Type = append(types, "null")
			}
		}
	}

	// Recurse into properties
	for _, propRef := range schema.Properties {
		if propRef != nil && propRef.Value != nil {
			fixNullableInSchema(propRef.Value)
		}
	}

	// Recurse into items
	if schema.Items != nil && schema.Items.Value != nil {
		fixNullableInSchema(schema.Items.Value)
	}

	// Recurse into allOf, anyOf, oneOf
	for _, ref := range schema.AllOf {
		if ref != nil && ref.Value != nil {
			fixNullableInSchema(ref.Value)
		}
	}
	for _, ref := range schema.AnyOf {
		if ref != nil && ref.Value != nil {
			fixNullableInSchema(ref.Value)
		}
	}
	for _, ref := range schema.OneOf {
		if ref != nil && ref.Value != nil {
			fixNullableInSchema(ref.Value)
		}
	}
}

// Naming convention sources
const (
	NamingSourceOpenAI    = "openai"
	NamingSourceEchartify = "echartify"
	NamingSourceOmniAgent = "omniagent"
)

// openAISchemas are schemas inherited from the OpenAI API specification.
var openAISchemas = map[string]bool{
	"ChatCompletionChoice":               true,
	"ChatCompletionContentPart":          true,
	"ChatCompletionLogprobs":             true,
	"ChatCompletionMessage":              true,
	"ChatCompletionMessageParam":         true,
	"ChatCompletionMessageToolCall":      true,
	"ChatCompletionNamedToolChoice":      true,
	"ChatCompletionStreamChoice":         true,
	"ChatCompletionStreamDelta":          true,
	"ChatCompletionStreamOptions":        true,
	"ChatCompletionStreamToolCall":       true,
	"ChatCompletionTokenLogprob":         true,
	"ChatCompletionTool":                 true,
	"CompletionUsage":                    true,
	"CreateChatCompletionRequest":        true,
	"CreateChatCompletionResponse":       true,
	"CreateChatCompletionStreamResponse": true,
	"ErrorResponse":                      true,
	"FunctionCall":                       true,
	"FunctionDefinition":                 true,
	"FunctionObject":                     true,
	"ImageUrl":                           true,
	"ListModelsResponse":                 true,
	"Model":                              true,
	"ResponseFormat":                     true,
	"ResponseFormatJSONObject":           true,
	"ResponseFormatJSONSchema":           true,
	"ResponseFormatText":                 true,
	"TopLogprob":                         true,
}

// echartifySchemas are schemas inherited from the echartify/chartir library.
var echartifySchemas = map[string]bool{
	"Axis":    true,
	"ChartIR": true,
	"Column":  true,
	"Dataset": true,
	"Encode":  true,
	"Grid":    true,
	"Legend":  true,
	"Mark":    true,
	"Style":   true,
	"Tooltip": true,
}

// tagSchemaNamingSources adds x-naming-source and x-naming-convention extensions to schemas.
func tagSchemaNamingSources(schemas openapi3.Schemas) {
	for name, schemaRef := range schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}

		var source, convention string
		if openAISchemas[name] {
			source = NamingSourceOpenAI
			convention = "snake_case"
		} else if echartifySchemas[name] {
			source = NamingSourceEchartify
			convention = "camelCase"
		} else {
			source = NamingSourceOmniAgent
			convention = "snake_case"
		}

		// Initialize extensions map if needed
		if schemaRef.Value.Extensions == nil {
			schemaRef.Value.Extensions = make(map[string]any)
		}
		schemaRef.Value.Extensions["x-naming-source"] = source
		schemaRef.Value.Extensions["x-naming-convention"] = convention
	}
}

// removeSchemaFieldDescriptions removes redundant descriptions from $schema fields.
// Huma auto-generates "A URL to the JSON Schema for this object." for every $schema field,
// which adds noise without value. The field name is self-explanatory.
func removeSchemaFieldDescriptions(schemas openapi3.Schemas) {
	for _, schemaRef := range schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}
		if prop, exists := schemaRef.Value.Properties["$schema"]; exists && prop.Value != nil {
			prop.Value.Description = ""
		}
	}
}

// fixMissingTypes adds type fields to schemas that are missing them.
func fixMissingTypes(schemas openapi3.Schemas) {
	for _, schemaRef := range schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}
		fixMissingTypesInSchema(schemaRef.Value)
	}
}

// fixMissingTypesInSchema recursively fixes missing type fields.
func fixMissingTypesInSchema(schema *openapi3.Schema) {
	if schema == nil {
		return
	}

	// Fix empty schemas (no type, no properties, no ref)
	// These match anything in JSON Schema but cause validation warnings
	if schema.Type == nil && len(schema.Properties) == 0 &&
		schema.Items == nil && len(schema.AllOf) == 0 &&
		len(schema.AnyOf) == 0 && len(schema.OneOf) == 0 {
		// Default to object type for empty schemas
		schema.Type = &openapi3.Types{"object"}
		if schema.AdditionalProperties.Schema == nil && schema.AdditionalProperties.Has == nil {
			// Allow any additional properties
			t := true
			schema.AdditionalProperties.Has = &t
		}
	}

	// Recurse into properties
	for _, propRef := range schema.Properties {
		if propRef != nil && propRef.Value != nil {
			fixMissingTypesInSchema(propRef.Value)
		}
	}

	// Recurse into additionalProperties
	if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		fixMissingTypesInSchema(schema.AdditionalProperties.Schema.Value)
	}

	// Recurse into items
	if schema.Items != nil && schema.Items.Value != nil {
		fixMissingTypesInSchema(schema.Items.Value)
	}

	// Recurse into allOf, anyOf, oneOf
	for _, ref := range schema.AllOf {
		if ref != nil && ref.Value != nil {
			fixMissingTypesInSchema(ref.Value)
		}
	}
	for _, ref := range schema.AnyOf {
		if ref != nil && ref.Value != nil {
			fixMissingTypesInSchema(ref.Value)
		}
	}
	for _, ref := range schema.OneOf {
		if ref != nil && ref.Value != nil {
			fixMissingTypesInSchema(ref.Value)
		}
	}
}
