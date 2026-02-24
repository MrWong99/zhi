package core

import (
	"fmt"
	"path/filepath"

	"github.com/MrWong99/zhi/pkg/providers/config/structuredfile"
	"github.com/MrWong99/zhi/pkg/providers/store/jsonfile"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// NewStructuredFileProvider is a ConfigFactory that creates a structuredfile
// config provider. It reads the "directory" option from the options map;
// if absent, it uses the structuredfile default directory.
func NewStructuredFileProvider(workspace string, options map[string]any) (config.Plugin, error) {
	dir := structuredfile.DefaultDir
	if options != nil {
		if d, ok := options["directory"]; ok {
			ds, ok := d.(string)
			if !ok {
				return nil, fmt.Errorf("structuredfile: \"directory\" option must be a string, got %T", d)
			}
			dir = ds
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	logger.Debug("loading structuredfile provider", "dir", dir)
	return structuredfile.NewFromDir(dir)
}

// DefaultStoreDir is the default directory for the built-in JSON file store,
// relative to the workspace root.
const DefaultStoreDir = ".zhi/store"

// NewJSONFileStoreProvider is a StoreFactory that creates a jsonfile store
// provider. It reads the "directory" option from the options map; if absent,
// it uses DefaultStoreDir relative to the workspace.
func NewJSONFileStoreProvider(workspace string, options map[string]any) (store.Plugin, error) {
	dir := DefaultStoreDir
	if options != nil {
		if d, ok := options["directory"]; ok {
			ds, ok := d.(string)
			if !ok {
				return nil, fmt.Errorf("jsonfile: \"directory\" option must be a string, got %T", d)
			}
			dir = ds
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	logger.Debug("loading jsonfile store provider", "dir", dir)
	return jsonfile.New(jsonfile.Config{Dir: dir})
}
