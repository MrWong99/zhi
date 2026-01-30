package core

import (
	"fmt"

	"github.com/MrWong99/zhi/pkg/providers/config/structuredfile"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// NewStructuredFileProvider is a ConfigFactory that creates a structuredfile
// config provider. It reads the "directory" option from the options map;
// if absent, it uses the structuredfile default directory.
func NewStructuredFileProvider(options map[string]any) (config.Plugin, error) {
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
	return structuredfile.NewFromDir(dir)
}
