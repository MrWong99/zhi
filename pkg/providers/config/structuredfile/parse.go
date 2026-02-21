package structuredfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"gopkg.in/yaml.v3"
)

// entry holds the parsed data for a single configuration leaf node.
type entry struct {
	Val              any
	Metadata         map[string]any
	ValidationSource string   // Go code body; only populated from YAML files.
	Imports          []string // Additional Go stdlib packages for validation code.
}

// parseFile reads a JSON or YAML file and returns a flat map of config path
// to entry. The file type is determined by extension (.json, .yaml, .yml).
func parseFile(path string) (map[string]*entry, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".yaml", ".yml":
	default:
		return nil, fmt.Errorf("unsupported file extension %q", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	entries := make(map[string]*entry)
	if err := flatten(raw, nil, entries); err != nil {
		return nil, fmt.Errorf("processing %s: %w", path, err)
	}
	return entries, nil
}

// isLeaf reports whether m represents a configuration leaf node. A node is a
// leaf if and only if it contains a "val" key.
func isLeaf(m map[string]any) bool {
	_, ok := m["val"]
	return ok
}

// flatten recursively walks a nested map and populates out with entries keyed
// by slash-delimited configuration paths.
func flatten(m map[string]any, prefix []string, out map[string]*entry) error {
	for key, val := range m {
		segments := slices.Concat(prefix, []string{key})

		child, isMap := val.(map[string]any)
		if !isMap {
			return fmt.Errorf("expected map at %s, got %T", strings.Join(segments, "/"), val)
		}

		if isLeaf(child) {
			path := strings.Join(segments, "/")
			if err := config.ValidatePath(path); err != nil {
				return fmt.Errorf("invalid path %q: %w", path, err)
			}

			e := &entry{Val: child["val"]}

			if md, ok := child["metadata"]; ok {
				mdMap, ok := md.(map[string]any)
				if !ok {
					return fmt.Errorf("metadata at %s must be a map, got %T", path, md)
				}
				e.Metadata = mdMap
			}

			if vs, ok := child["validation"]; ok {
				src, ok := vs.(string)
				if !ok {
					return fmt.Errorf("validation at %s must be a string, got %T", path, vs)
				}
				e.ValidationSource = src
			}

			if imp, ok := child["imports"]; ok {
				impSlice, ok := imp.([]any)
				if !ok {
					return fmt.Errorf("imports at %s must be a list of strings, got %T", path, imp)
				}
				for i, item := range impSlice {
					s, ok := item.(string)
					if !ok {
						return fmt.Errorf("imports[%d] at %s must be a string, got %T", i, path, item)
					}
					e.Imports = append(e.Imports, s)
				}
			}

			out[path] = e
		} else {
			if err := flatten(child, segments, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadDir reads all JSON and YAML files from dir in lexicographic order and
// returns a merged flat map of entries. If the same path appears in multiple
// files the last file (by name sort order) wins.
func loadDir(dir string) (map[string]*entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	merged := make(map[string]*entry)
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(de.Name()))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}

		entries, err := parseFile(filepath.Join(dir, de.Name()))
		if err != nil {
			return nil, err
		}
		maps.Copy(merged, entries)
	}
	return merged, nil
}
