package structuredfile

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// validateFunc is the signature of a compiled validation function extracted
// from a YAML validation field.
type validateFunc func(v config.Value, tree config.TreeReader) ([]config.ValidationResult, error)

// configSymbols exports the config package types and constants to Yaegi so
// that interpreted validation code can import and use them.
var configSymbols = interp.Exports{
	"github.com/MrWong99/zhi/pkg/zhiplugin/config/config": {
		// Types — exported as reflect.Type wrapped in a zero-value pointer.
		"Value":            reflect.ValueOf((*config.Value)(nil)),
		"TreeReader":       reflect.ValueOf((*config.TreeReader)(nil)),
		"ValidationResult": reflect.ValueOf((*config.ValidationResult)(nil)),
		"Severity":         reflect.ValueOf((*config.Severity)(nil)),

		// Severity constants.
		"Info":     reflect.ValueOf(config.Info),
		"Warning":  reflect.ValueOf(config.Warning),
		"Blocking": reflect.ValueOf(config.Blocking),
	},
}

// compileValidation takes the raw Go function body from a YAML validation
// field and returns a callable validateFunc. The body has access to two
// variables:
//
//	v    config.Value       — the value being validated
//	tree config.TreeReader  — read-only view of the full configuration tree
//
// The body must return ([]config.ValidationResult, error).
//
// The imports parameter lists additional Go standard library packages that
// should be available in the validation code (e.g. "strings", "math",
// "regexp", "fmt"). These are injected as extra import statements alongside
// the always-present config import.
func compileValidation(path, body string, imports []string) (validateFunc, error) {
	var extraImports strings.Builder
	for _, pkg := range imports {
		fmt.Fprintf(&extraImports, "\t%q\n", pkg)
	}

	src := fmt.Sprintf(`package validation

import (
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
%s)

func Validate(v config.Value, tree config.TreeReader) ([]config.ValidationResult, error) {
	%s
}
`, extraImports.String(), body)

	i := interp.New(interp.Options{})
	if err := i.Use(configSymbols); err != nil {
		return nil, fmt.Errorf("compiling validation for %s: loading symbols: %w", path, err)
	}
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("compiling validation for %s: loading stdlib symbols: %w", path, err)
	}

	_, err := i.Eval(src)
	if err != nil {
		return nil, fmt.Errorf("compiling validation for %s: %w", path, err)
	}

	v, err := i.Eval("validation.Validate")
	if err != nil {
		return nil, fmt.Errorf("compiling validation for %s: extracting function: %w", path, err)
	}

	fn, ok := v.Interface().(func(config.Value, config.TreeReader) ([]config.ValidationResult, error))
	if !ok {
		return nil, fmt.Errorf("compiling validation for %s: unexpected function signature %T", path, v.Interface())
	}

	return fn, nil
}
