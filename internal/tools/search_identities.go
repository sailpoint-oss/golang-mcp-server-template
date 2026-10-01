// Package tools holds the MCP tool registrations.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	api "github.com/sailpoint-oss/golang-sdk/v3/search"

	"github.com/sailpoint-oss/golang-mcp-server-template/internal/logger"
	"github.com/sailpoint-oss/golang-mcp-server-template/internal/sailpoint"
)

const (
	maxLimit     = 250
	defaultLimit = 25
)

// SearchIdentitiesArgs is the tool input. The `jsonschema` tag supplies each
// property's description; numeric bounds and defaults are attached in
// searchIdentitiesSchema, which the tag syntax does not cover.
//
// Every optional field is tagged omitempty so it lands outside the schema's
// "required" list.
type SearchIdentitiesArgs struct {
	Query string `json:"query" jsonschema:"Search query in SailPoint/Elasticsearch query-string syntax, scoped to the identities index. Examples: \"*\" for everything, \"attributes.department:Engineering\", \"name:Aaron*\", \"attributes.cloudLifecycleState:active AND attributes.country:US\", \"@access(name:\\\"Administrator\\\")\"."`

	Limit         int32    `json:"limit,omitempty" jsonschema:"Maximum identities to return (1-250). Defaults to 25."`
	Offset        int32    `json:"offset,omitempty" jsonschema:"Offset into the result set, for paging."`
	Sort          []string `json:"sort,omitempty" jsonschema:"Fields to sort by; prefix with \"-\" for descending. Example: [\"displayName\", \"-created\"]."`
	Attributes    []string `json:"attributes,omitempty" jsonschema:"Restrict returned fields, which keeps responses small. Example: [\"id\", \"name\", \"displayName\", \"email\", \"attributes.department\"]."`
	IncludeNested bool     `json:"includeNested,omitempty" jsonschema:"Include nested objects (access, accounts, apps) on each identity. Substantially larger responses."`
	Count         bool     `json:"count,omitempty" jsonschema:"Also return the total number of matching identities, ignoring limit/offset. Adds latency."`
}

// SearchIdentitiesResult is the tool output. The MCP SDK derives the tool's
// output schema from it and returns it both as structured content and as
// pretty-printed JSON text.
type SearchIdentitiesResult struct {
	Query      string           `json:"query"`
	Returned   int              `json:"returned"`
	TotalCount *int64           `json:"totalCount,omitempty"`
	Offset     int32            `json:"offset"`
	Limit      int32            `json:"limit"`
	Identities []map[string]any `json:"identities"`
}

// RegisterSearchIdentities adds the search_identities tool to the server.
func RegisterSearchIdentities(server *mcp.Server) error {
	schema, err := searchIdentitiesSchema()
	if err != nil {
		return err
	}
	outputSchema, err := searchIdentitiesOutputSchema()
	if err != nil {
		return err
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:  "search_identities",
		Title: "Search identities",
		Description: "Search identities in SailPoint Identity Security Cloud using the search API. " +
			"Use this to find users by name, email, attribute, lifecycle state, manager, source, or entitlement/role access. " +
			"Returns matching identity documents from the 'identities' index.",
		InputSchema:  schema,
		OutputSchema: outputSchema,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: boolPtr(true),
		},
	}, searchIdentities)

	return nil
}

func searchIdentities(ctx context.Context, _ *mcp.CallToolRequest, args SearchIdentitiesArgs) (*mcp.CallToolResult, SearchIdentitiesResult, error) {
	// JSON Schema defaults are advertised to the client but not applied by the
	// SDK, so an omitted limit arrives as the zero value.
	limit := args.Limit
	if limit == 0 {
		limit = defaultLimit
	}

	client, err := sailpoint.Client()
	if err != nil {
		return nil, SearchIdentitiesResult{}, err
	}

	search := *api.NewSearch()
	search.Indices = []api.Index{api.INDEX_IDENTITIES}
	search.QueryType = queryTypePtr(api.QUERYTYPE_SAILPOINT)
	search.Query = &api.Query{Query: &args.Query}
	search.IncludeNested = &args.IncludeNested
	if len(args.Sort) > 0 {
		search.Sort = args.Sort
	}
	if len(args.Attributes) > 0 {
		search.QueryResultFilter = &api.QueryResultFilter{Includes: args.Attributes}
	}

	identities, response, err := client.SearchAPI.SearchPostV1(ctx).
		Search(search).
		Limit(limit).
		Offset(args.Offset).
		Count(args.Count).
		Execute()
	if err != nil {
		message := describeError(err, response)
		logger.Log("search_identities failed", message)
		return nil, SearchIdentitiesResult{}, fmt.Errorf("identity search failed: %s", message)
	}

	if identities == nil {
		identities = []map[string]any{}
	}

	result := SearchIdentitiesResult{
		Query:      args.Query,
		Returned:   len(identities),
		TotalCount: totalCount(response),
		Offset:     args.Offset,
		Limit:      limit,
		Identities: identities,
	}

	// Emit the same pretty-printed JSON the TypeScript server returns, rather
	// than letting the SDK fill Content with compact JSON.
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, SearchIdentitiesResult{}, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
	}, result, nil
}

// searchIdentitiesSchema infers the schema from SearchIdentitiesArgs, then adds
// the constraints the struct tags cannot express.
func searchIdentitiesSchema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[SearchIdentitiesArgs](nil)
	if err != nil {
		return nil, fmt.Errorf("search_identities input schema: %w", err)
	}

	schema.Properties["query"].MinLength = intPtr(1)

	limit := schema.Properties["limit"]
	limit.Minimum = floatPtr(1)
	limit.Maximum = floatPtr(maxLimit)
	limit.Default = json.RawMessage(strconv.Itoa(defaultLimit))

	schema.Properties["offset"].Minimum = floatPtr(0)

	dropNullTypes(schema)

	return schema, nil
}

// searchIdentitiesOutputSchema infers the schema from SearchIdentitiesResult.
// It is built here rather than left to the SDK so dropNullTypes can run on it.
func searchIdentitiesOutputSchema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[SearchIdentitiesResult](nil)
	if err != nil {
		return nil, fmt.Errorf("search_identities output schema: %w", err)
	}

	dropNullTypes(schema)

	return schema, nil
}

// dropNullTypes rewrites properties inferred as {"type": ["null", X]} to
// {"type": X}. Inference treats every slice and pointer as nullable, but none of
// these fields is ever null on the wire: optional inputs are omitted, the
// identities slice is always non-nil, and a nil totalCount is omitted. The
// array form of "type" is also rejected by clients that map tool schemas onto a
// single-type dialect (such as Gemini function declarations).
func dropNullTypes(schema *jsonschema.Schema) {
	for _, property := range schema.Properties {
		var nonNull []string
		for _, t := range property.Types {
			if t != "null" {
				nonNull = append(nonNull, t)
			}
		}
		if len(nonNull) == 1 && len(property.Types) == 2 {
			property.Type = nonNull[0]
			property.Types = nil
		}
	}
}

// totalCount reads the X-Total-Count header the search API sets when count=true.
func totalCount(response *http.Response) *int64 {
	if response == nil {
		return nil
	}
	raw := response.Header.Get("X-Total-Count")
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

// describeError surfaces the HTTP status and response body, which is where the
// SDK puts the useful detail for auth failures and malformed queries.
func describeError(err error, response *http.Response) string {
	var apiErr api.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		status := ""
		if response != nil {
			status = "HTTP " + response.Status + " "
		}
		if body := apiErr.Body(); len(body) > 0 {
			return fmt.Sprintf("%s%s %s", status, apiErr.Error(), body)
		}
		return status + apiErr.Error()
	}
	return err.Error()
}

func boolPtr(v bool) *bool                        { return &v }
func intPtr(v int) *int                           { return &v }
func floatPtr(v float64) *float64                 { return &v }
func queryTypePtr(v api.QueryType) *api.QueryType { return &v }
