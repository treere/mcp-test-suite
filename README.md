# MCP Integration Test Suite

A Go-based integration test runner for MCP (Model Context Protocol) servers.

## Quick Start

```bash
cd test-suite

# Build the test runner
go build -o bin/test-suite ./cmd/runner

# Run the tests (requires server running on localhost:4000)
./bin/test-suite config.yaml
```

## Prerequisites

- Go 1.23+
- MCP server running on configured endpoint

## Configuration

Edit `config.yaml` to customize tests:

```yaml
server:
  url: "http://localhost:4000/mcp"
  timeout: 10s

tests:
  - name: my_test
    method: tools/call
    params:
      name: my_tool
      arguments:
        arg1: "value"
    expect:
      result.field: expected_value
    session: true  # Use session (default: true)
```

## Test Options

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Test name (required) |
| `method` | string | JSON-RPC method (required) |
| `params` | object | Request parameters |
| `expect` | object | Assertions |
| `session` | bool | Use session (default: true) |

## Expectation Syntax

### Simple values
```yaml
expect:
  field: "value"
  nested.field: "value"
```

### Array access
```yaml
expect:
  tools[0].name: "tool_name"
  tools[*].name: "any_tool"  # wildcard
```

### String contains
```yaml
expect:
  content[0].text_contains: "substring"
```

### HTTP status codes
```yaml
expect:
  http_status: 202
```

### Error responses
```yaml
expect:
  error: true
```

## Running Tests

### Against running server
```bash
./bin/test-suite config.yaml
```

### Custom config
```bash
./bin/test-suite path/to/config.yaml
```

### With server startup (bash)
```bash
# Terminal 1: Start server
mix run --no-halt

# Terminal 2: Run tests
./bin/test-suite config.yaml
```

## Development

```bash
# Run unit tests
go test -v ./...

# Build
go build -o bin/test-suite ./cmd/runner
```

## Project Structure

```
test-suite/
├── cmd/runner/main.go    # Entry point
├── config/config.go       # Config loading
├── config.yaml           # Test definitions
├── runner/runner.go      # Test execution
├── runner/runner_test.go # Unit tests
└── go.mod
```
