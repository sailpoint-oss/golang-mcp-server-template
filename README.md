# golang-mcp-server-template

A Go [Model Context Protocol](https://modelcontextprotocol.io) server for
SailPoint Identity Security Cloud, built on the official
[SailPoint Go SDK](https://github.com/sailpoint-oss/golang-sdk)
(`github.com/sailpoint-oss/golang-sdk/v3`) and the official
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

Currently exposes one tool: **`search_identities`**.

## Layout

| Path                                       | Purpose                                                                        |
| ------------------------------------------ | ------------------------------------------------------------------------------ |
| [main.go](main.go)                         | Entry point — builds the MCP server, registers tools, serves over stdio        |
| [internal/logger/](internal/logger/)       | Keeps stdout clean for JSON-RPC and routes all logging to stderr               |
| [internal/sailpoint/](internal/sailpoint/) | Config resolution, validation, and the shared `*sdk.APIClient`                 |
| [internal/tools/](internal/tools/)         | One file per tool; see `search_identities.go` for the reference implementation |
| [cmd/smoke/](cmd/smoke/)                   | Smoke-test client: handshake, `tools/list`, and one live tool call             |
| [Makefile](Makefile)                       | `build`, `run`, `vet`, `test`, `smoke`                                         |

## Setup

```bash
make build          # -> bin/sailpoint-mcp-server
```

Requires Go 1.25+.

## Configuration

Credentials are read from `SAIL_BASE_URL`, `SAIL_CLIENT_ID` and
`SAIL_CLIENT_SECRET`. At startup the server loads them from `.env` in the
project root. Create a
personal access token in your tenant under **Preferences → Personal Access
Tokens**; it needs a role that can read identities (e.g. `sp:scopes:all` or
an admin/helpdesk role).

```bash
cp .env.example .env   # then fill in your three values
chmod 600 .env
```

The server loads `.env` from its own project folder, so it works no matter which
directory the MCP client launches it from. Keep the file private: it is
gitignored, and `chmod 600 .env` makes it readable only by you. Variables set in
your shell or in an MCP client's `env` block take precedence over `.env`.

The server validates the resolved base URL, client ID, client secret and token
URL at startup and exits with a descriptive message if any are missing, rather
than failing on the first tool call.

## Running

```bash
make run           # ./bin/sailpoint-mcp-server — speaks MCP over stdio
```

### Register with Claude Code

```bash
claude mcp add sailpoint -- /absolute/path/to/golang-mcp-server-template/bin/sailpoint-mcp-server
```

### Register with Claude Desktop / other clients

```json
{
  "mcpServers": {
    "sailpoint": {
      "command": "/absolute/path/to/golang-mcp-server-template/bin/sailpoint-mcp-server"
    }
  }
}
```

## Tool: `search_identities`

Wraps `POST /search/v1` (`SearchAPI.SearchPostV1`) against the `identities`
index with `queryType: SAILPOINT`.

| Parameter       | Type       | Default | Description                                                              |
| --------------- | ---------- | ------- | ------------------------------------------------------------------------ |
| `query`         | `string`   | —       | Query-string syntax, e.g. `attributes.department:Engineering`             |
| `limit`         | `integer`  | `25`    | 1–250 results                                                            |
| `offset`        | `integer`  | `0`     | Paging offset                                                            |
| `sort`          | `string[]` | —       | e.g. `["displayName", "-created"]`                                       |
| `attributes`    | `string[]` | —       | Restrict returned fields (`queryResultFilter.includes`) to keep responses small |
| `includeNested` | `boolean`  | `false` | Include nested `access`, `accounts`, `apps` objects                      |
| `count`         | `boolean`  | `false` | Also return the total match count from `X-Total-Count`                   |

Example query strings:

```
*
name:Aaron*
attributes.cloudLifecycleState:active AND attributes.country:US
@access(name:"Administrator")
```

The response carries `query`, `returned`, `offset`, `limit`, optional
`totalCount`, and the `identities` array — both as pretty-printed JSON text and
as MCP structured content (the tool declares an output schema).

Errors (auth failures, bad queries) are returned as MCP tool errors with the
HTTP status and response body, rather than crashing the server.

## Development

```bash
make vet     # go vet ./...
make test    # go test ./...
make smoke   # handshake + tools/list + one live search_identities call
```

`make smoke` requires working credentials for the tool call to succeed; the
handshake and `tools/list` portions work without them.

## Notes

- **Everything logs to stderr.** The SailPoint SDK prints diagnostics to stdout
  with `fmt.Print` — the deprecation notice in `NewDefaultConfiguration`, and
  token-request failures in `getAccessToken` — which would corrupt the JSON-RPC
  framing. [internal/logger](internal/logger/logger.go) captures the real stdout
  at package init, points `os.Stdout` and the standard `log` package at stderr,
  and hands the real descriptor to `mcp.IOTransport`. This is why the server uses
  `IOTransport` rather than `StdioTransport`, which reads `os.Stdout` at connect
  time.
- **`sdk.NewDefaultConfiguration` panics** when it cannot find any configuration
  source, and when a config file is present but malformed.
  [internal/sailpoint](internal/sailpoint/sailpoint.go) recovers from that and
  converts it to an error so startup fails with a message instead of a stack trace.
- **The SDK fetches a fresh OAuth token per request** and does not cache it. If
  the client-credentials exchange fails, `getAccessToken` returns an empty token
  rather than an error, so bad credentials surface as an HTTP 401 from the search
  call instead of an auth-specific message.
- The SDK's `retryablehttp` client logs every request by default; the logger is
  disabled in `NewConfiguration` to keep stderr readable.
- JSON Schema `default` values are advertised to clients but not applied by the
  MCP SDK, so an omitted `limit` arrives as `0` and the handler substitutes `25`.
  The `jsonschema` struct tag only carries a description, so `minimum`, `maximum`
  and `default` are attached to the inferred schema in `searchIdentitiesSchema`.

## Adding more tools

Add a `Register*` function under [internal/tools/](internal/tools/) following
[search_identities.go](internal/tools/search_identities.go), then call it from
[main.go](main.go). Reuse `sailpoint.Client()` — it returns the one shared
`*sdk.APIClient`, which exposes every API version (`V3`, `Beta`, `V2024`,
`V2025`, `V2026`, `NERM`) so no new accessor is needed per service.
