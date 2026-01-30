package core

import (
	"context"
	"fmt"
	"io"
	"os"
)

// ExportAsJSON renders enabled components' configuration as indented JSON
// and writes it to the given output path (or stdout if "-" or empty).
func ExportAsJSON(ctx context.Context, tree *TreeData, output string) error {
	return exportBuiltin(ctx, tree, "json", output)
}

// ExportAsYAML renders enabled components' configuration as YAML
// and writes it to the given output path (or stdout if "-" or empty).
func ExportAsYAML(ctx context.Context, tree *TreeData, output string) error {
	return exportBuiltin(ctx, tree, "yaml", output)
}

// ExportAsTOML renders enabled components' configuration as TOML
// and writes it to the given output path (or stdout if "-" or empty).
func ExportAsTOML(ctx context.Context, tree *TreeData, output string) error {
	return exportBuiltin(ctx, tree, "toml", output)
}

// ExportAsDotenv renders enabled components' configuration as dotenv
// and writes it to the given output path (or stdout if "-" or empty).
func ExportAsDotenv(ctx context.Context, tree *TreeData, output string) error {
	return exportBuiltin(ctx, tree, "dotenv", output)
}

func exportBuiltin(ctx context.Context, tree *TreeData, format, output string) error {
	result, err := Export(ctx, tree, ExportRunConfig{
		Format:     format,
		OutputPath: output,
		DryRun:     output == "" || output == "-",
	})
	if err != nil {
		return err
	}

	if output == "" || output == "-" {
		var w io.Writer = os.Stdout
		fmt.Fprint(w, result.Content)
		if len(result.Content) > 0 && result.Content[len(result.Content)-1] != '\n' {
			fmt.Fprintln(w)
		}
	}
	return nil
}
