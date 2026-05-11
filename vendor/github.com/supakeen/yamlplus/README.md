# yamlplus

A Go library that extends YAML with cross-file references, allowing you to split large configuration files into smaller, reusable pieces.

## Features

- Reference YAML anchors across multiple files using the `!xref` tag
- Support for standard YAML map merges with cross-file references
- Streaming `Decoder` API with options like `KnownFields`
- Load files individually, by directory, or recursively
- Built on top of `go.yaml.in/yaml/v3`
- Circular dependency detection

## Installation

```bash
go get github.com/supakeen/yamlplus
```

## Quick Start

Given two YAML files:

**database.yaml:**
```yaml
connection: &db-config
  host: localhost
  port: 5432
  timeout: 30s
```

**app.yaml:**
```yaml
service:
  name: myapp
  database: !xref "database.yaml#db-config"
```

Load and unmarshal them:

```go
package main

import (
    "fmt"
    "os"
    "github.com/supakeen/yamlplus"
)

func main() {
    loader := yamlplus.NewLoader(os.DirFS("config"))
    loader.RegisterFile("database.yaml")
    
    var config map[string]any
    data, _ := os.ReadFile("config/app.yaml")
    loader.Unmarshal(data, &config)
    
    // Access the cross-referenced database configuration
    service := config["service"].(map[string]any)
    db := service["database"].(map[string]any)
    fmt.Printf("Database: %s:%d\n", db["host"], db["port"])
}
```

## Decoder

For streaming decoding or when you need options like strict field checking, use
`NewDecoder` instead of `Unmarshal`:

```go
file, _ := os.Open("config/app.yaml")
defer file.Close()

dec := loader.NewDecoder(file)
dec.KnownFields(true) // error on unknown fields

var config AppConfig
if err := dec.Decode(&config); err != nil {
    log.Fatal(err)
}
```

The `Decoder` supports all the same `!xref` resolution as `Unmarshal`. It also
handles multi-document YAML streams — call `Decode` repeatedly until it returns
`io.EOF`:

```go
dec := loader.NewDecoder(file)

for {
    var doc map[string]any
    if err := dec.Decode(&doc); err == io.EOF {
        break
    } else if err != nil {
        log.Fatal(err)
    }
    fmt.Println(doc)
}
```

## Reference Syntax

The `!xref` tag supports two forms:

### Reference a specific anchor

```yaml
config: !xref "filename.yaml#anchorname"
```

### Reference an entire file

```yaml
config: !xref "filename.yaml"
```

When referencing a file without an anchor, the first document in the file is used.

## Map Merges

Cross-file references work with YAML's map merge syntax:

**defaults.yaml:**
```yaml
defaults: &api-defaults
  timeout: 30s
  retries: 3
  log_level: info
```

**service.yaml:**
```yaml
production:
  <<: !xref "defaults.yaml#api-defaults"
  timeout: 60s        # Override the default
  endpoint: /api/v1
```

Result after unmarshaling:
```yaml
production:
  timeout: 60s
  retries: 3
  log_level: info
  endpoint: /api/v1
```

## Loading Files

### Load individual files

```go
loader := yamlplus.NewLoader(os.DirFS("config"))
loader.RegisterFile("base.yaml")
loader.RegisterFile("database.yaml")
```

### Load all YAML files in a directory

```go
loader.RegisterDirectory("configs")
```

This loads all `.yaml` and `.yml` files in the directory (non-recursive).

### Load files recursively

```go
loader.RegisterRecursively("configs")
```

This walks the directory tree and loads all YAML files.

## Path-Based Namespacing

Files are registered and referenced using their exact path relative to the filesystem root:

```go
loader := yamlplus.NewLoader(os.DirFS("/etc"))
loader.RegisterFile("app/config.yaml")

// Must use the same path in references:
data := []byte(`settings: !xref "app/config.yaml"`)
```

## Circular Dependency Detection

The library detects circular references and returns an error:

**a.yaml:**
```yaml
value: !xref "b.yaml"
```

**b.yaml:**
```yaml
value: !xref "a.yaml"
```

Attempting to unmarshal will result in an error: `circular dependency detected`.

## Thread Safety

A `Loader` is safe for concurrent `Unmarshal` calls after all files have been registered. However, `RegisterFile`, `RegisterDirectory`, and `RegisterRecursively` should not be called concurrently with each other or with `Unmarshal`.

## Examples

See the [examples in the documentation](https://pkg.go.dev/github.com/supakeen/yamlplus#pkg-examples) for more usage patterns.

## Testing

```bash
go test ./...
```

## License

MIT
