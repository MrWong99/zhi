# Structured File Configuration Provider

The structured file provider is a `config.Plugin` implementation that loads
its values from JSON and YAML files in a directory. It lives in
`pkg/providers/config/structuredfile`.

## Package layout

```
pkg/providers/config/structuredfile/
  structuredfile.go      Plugin type, Load, List/Get/Set/Validate
  parse.go               file parsing, leaf detection, path flattening
  validate.go            Yaegi-based validation code compilation
```

## How it works

On startup the plugin reads every `.json`, `.yaml` and `.yml` file from
`./structuredfile` (relative to the working directory). Files are processed
in **lexicographic order**; when the same path appears in more than one file
the last file wins. This makes layered configuration straightforward:

```
structuredfile/
  00-defaults.yaml
  10-overrides.json
```

Each file contains a nested map that is flattened into slash-delimited
configuration paths. A node is a **leaf** (an actual configuration entry)
when its map contains a `val` key. Every other node is treated as a branch
that contributes a path segment.

## File format

### YAML

```yaml
pokedex:
  trainer.name:
    val: Ash
    metadata:
      description: Name of the Pokemon trainer
      ui.readonly: true
    validation: |-
      name, ok := v.Val.(string)
      if !ok {
      	return []config.ValidationResult{{
      		Severity: config.Blocking,
      		Message:  "trainer name must be a string",
      	}}, nil
      }
      if name == "" {
      	return []config.ValidationResult{{
      		Severity: config.Blocking,
      		Message:  "every trainer needs a name!",
      	}}, nil
      }
      return nil, nil
  starter:
    val: pikachu
    metadata:
      description: Chosen starter Pokemon
      ui.select.from:
        - pikachu
        - bulbasaur
        - charmander
        - squirtle
```

### JSON

```json
{
  "pokedex": {
    "region": {
      "val": "kanto",
      "metadata": {
        "description": "Home region of the trainer",
        "ui.select.from": ["kanto", "johto", "hoenn", "sinnoh", "unova", "kalos", "alola", "galar", "paldea"]
      }
    },
    "pokedex.goal": {
      "val": 150,
      "metadata": {
        "description": "Number of Pokemon to catch",
        "ui.readonly": true
      }
    }
  }
}
```

Both examples above produce the following configuration paths:

| Path                    | Value       |
|-------------------------|-------------|
| `pokedex/trainer.name`  | `"Ash"`     |
| `pokedex/starter`       | `"pikachu"` |
| `pokedex/region`        | `"kanto"`   |
| `pokedex/pokedex.goal`  | `150`       |

## Leaf node schema

Every leaf node is a map with the following keys:

| Key          | Required | Type             | Description                                                  |
|--------------|----------|------------------|--------------------------------------------------------------|
| `val`        | yes      | any              | The configuration value.                                     |
| `metadata`   | no       | map[string]any   | Arbitrary key-value metadata (descriptions, UI hints, etc.). |
| `validation` | no       | string           | Go function body for validation. YAML only.                  |
| `imports`    | no       | list of strings  | Go standard library packages to import in validation code.   |

## Path mapping

Nested keys become path segments joined by `/`. Dots and hyphens inside a
single key are preserved as-is (they are valid within a path segment).

```yaml
database:        # branch → segment "database"
  host:          # branch → segment "host" → but this is also a leaf:
    val: localhost
```

produces `database/host`.

```yaml
app:
  tls:
    cert.pem:
      val: /etc/ssl/cert.pem
```

produces `app/tls/cert.pem`.

Every produced path is validated against the
[path format](config-plugin.md#path-format) rules before being accepted.

## Validation code

YAML files may attach a `validation` field to any leaf node. The field
contains the **body** of a Go function with this signature:

```go
func(v config.Value, tree config.TreeReader) ([]config.ValidationResult, error)
```

Inside the body the two parameters are available as `v` and `tree`. The code
can use all exported types from the
[config](../../pkg/zhiplugin/config/) package (`Value`,
`TreeReader`, `ValidationResult`, `Severity`, `Info`, `Warning`, `Blocking`).

The body is compiled once at load time using the
[Yaegi](https://github.com/traefik/yaegi) Go interpreter. If compilation
fails the plugin refuses to start and returns the error with the
originating file path and config path for debugging.

JSON files do not support the `validation` field because JSON has no
multiline string syntax.

### Standard library imports

Validation code can use Go standard library packages by listing them in the
`imports` field alongside `validation`. The `imports` field is a list of
package names (e.g. `strings`, `math`, `regexp`, `fmt`, `strconv`,
`net/url`, etc.). Any package from the Go standard library is supported.

> **Security note:** The Yaegi interpreter loads the full Go standard library,
> which means validation code can call `os/exec`, `net/http`, `os.Remove`,
> and any other stdlib function. Configuration authors are considered trusted;
> never load configuration files from untrusted sources without review.

### Example

```yaml
network:
  port:
    val: 8080
    validation: |-
      p, ok := v.Val.(float64)
      if !ok {
      	return []config.ValidationResult{{
      		Severity: config.Blocking,
      		Message:  "port must be a number",
      	}}, nil
      }
      if p < 1 || p > 65535 {
      	return []config.ValidationResult{{
      		Severity: config.Blocking,
      		Message:  "port out of range",
      	}}, nil
      }
      return nil, nil
```

### Using standard library packages

```yaml
pokedex:
  trainer.name:
    val: Ash
    metadata:
      description: Name of the Pokemon trainer
    imports:
      - strings
      - regexp
    validation: |-
      name, ok := v.Val.(string)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "trainer name must be a string",
        }}, nil
      }
      if !strings.HasPrefix(name, "A") {
        return []config.ValidationResult{{
          Severity: config.Warning,
          Message:  "trainer name should start with A",
        }}, nil
      }
      matched, err := regexp.MatchString(`^[A-Za-z]+$`, name)
      if err != nil {
        return nil, err
      }
      if !matched {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "trainer name must only contain letters",
        }}, nil
      }
      return nil, nil
```

### Cross-value validation

Cross-value validation is possible through the `tree` parameter:

```yaml
network:
  tls.enabled:
    val: true
  port:
    val: 443
    validation: |-
      tls, ok := tree.Get("network/tls.enabled")
      if ok {
      	if enabled, _ := tls.Val.(bool); enabled {
      		if p, _ := v.Val.(float64); p != 443 {
      			return []config.ValidationResult{{
      				Severity: config.Warning,
      				Message:  "TLS is on but port is not 443",
      			}}, nil
      		}
      	}
      }
      return nil, nil
```

## Using the provider

### As a library

```go
import "github.com/MrWong99/zhi/pkg/providers/config/structuredfile"

// Load from the default ./structuredfile directory.
p, err := structuredfile.New()

// Or load from a custom directory.
p, err := structuredfile.NewFromDir("/etc/myapp/config")
```

### As a plugin binary

```go
package main

import (
    goplugin "github.com/hashicorp/go-plugin"

    "github.com/MrWong99/zhi/pkg/providers/config/structuredfile"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func main() {
    p, err := structuredfile.New()
    if err != nil {
        panic(err)
    }
    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]goplugin.Plugin{
            "config": &config.GRPCPlugin{Impl: p},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

## Behaviour notes

- **Set is in-memory only.** Calling `Set` updates the value for the
  lifetime of the process but does not write back to disk.
- **Missing directory is not an error.** If `./structuredfile` does not
  exist the plugin starts with an empty configuration tree.
- **Validation is optional.** Paths without a `validation` field are always
  considered valid.
- **Number types.** JSON numbers unmarshal as `float64`. YAML integers
  unmarshal as `int`. Keep this in mind when writing validation code that
  checks numeric values loaded from different file formats.
