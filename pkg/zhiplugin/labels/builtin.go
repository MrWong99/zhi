package labels

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T {
	return &v
}

// registerBuiltinLabels registers all built-in labels with the given registry.
func registerBuiltinLabels(r *Registry) {
	// UI namespace labels
	for _, label := range uiLabels {
		r.MustRegister(label)
	}

	// Store namespace labels
	for _, label := range storeLabels {
		r.MustRegister(label)
	}

	// Transform namespace labels
	for _, label := range transformLabels {
		r.MustRegister(label)
	}

	// Config namespace labels
	for _, label := range configLabels {
		r.MustRegister(label)
	}

	// Core namespace labels
	for _, label := range coreLabels {
		r.MustRegister(label)
	}
}

// UI namespace - interpreted by UI plugins
var uiLabels = []*Label{
	{
		Name:         "ui.readonly",
		Namespace:    "ui",
		Description:  "Prevents users from modifying this value in the UI. The value is displayed but not editable.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"ui"},
		Examples: []Example{
			{Value: true, Description: "Make field read-only"},
		},
		Since: "0.1.0",
	},
	{
		Name:         "ui.password",
		Namespace:    "ui",
		Description:  "Masks input characters during entry (displays as dots or asterisks). The stored value is unaffected.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"ui"},
		Examples: []Example{
			{Value: true, Description: "Mask password input"},
		},
		Since: "0.1.0",
	},
	{
		Name:         "ui.hidden",
		Namespace:    "ui",
		Description:  "Completely hides this value from the UI. Users cannot see or edit it.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"ui"},
		Since:        "0.1.0",
	},
	{
		Name:        "ui.pattern",
		Namespace:   "ui",
		Description: "Regular expression pattern that input must match. UI should validate and show errors.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$", Description: "Email pattern"},
			{Value: "^\\d{3}-\\d{3}-\\d{4}$", Description: "US phone number"},
		},
		Since: "0.1.0",
	},
	{
		Name:         "ui.multiline",
		Namespace:    "ui",
		Description:  "Indicates the value is multiline text and should use a textarea or similar input.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"ui"},
		Since:        "0.1.0",
	},
	{
		Name:         "ui.order",
		Namespace:    "ui",
		Description:  "Display order hint for UI rendering. Lower numbers appear first.",
		ValueType:    "int",
		DefaultValue: 0,
		AppliesTo:    []string{"ui"},
		Since:        "0.1.0",
	},
	{
		Name:        "ui.group",
		Namespace:   "ui",
		Description: "Groups related configuration values together in the UI.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "Database", Description: "Group database-related settings"},
			{Value: "Security", Description: "Group security-related settings"},
		},
		Since: "0.1.0",
	},
	{
		Name:        "ui.placeholder",
		Namespace:   "ui",
		Description: "Placeholder text shown in empty input fields.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "Enter your email address", Description: "Placeholder for email field"},
		},
		Since: "0.1.0",
	},
	{
		Name:        "ui.format",
		Namespace:   "ui",
		Description: "Hints at the display format for the value (e.g., 'json', 'yaml', 'code').",
		ValueType:   "string",
		Constraints: &Constraints{
			Enum: []string{"text", "json", "yaml", "toml", "code", "markdown", "html"},
		},
		AppliesTo: []string{"ui"},
		Since:     "0.1.0",
	},
	{
		Name:        "ui.confirm",
		Namespace:   "ui",
		Description: "Requires confirmation before changing this value (e.g., for destructive settings).",
		ValueType:   "bool",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: true, Description: "Require confirmation before changing"},
		},
		Since: "0.1.0",
	},
	{
		Name:        "ui.showIf",
		Namespace:   "ui",
		Description: "Conditionally shows this value only when another configuration path equals a specific value. Format: 'path=value'. The value is hidden when the condition is not met.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Constraints: &Constraints{
			Pattern: `^[a-z][a-z0-9./_-]*=.+$`,
		},
		Examples: []Example{
			{Value: "database/type=postgres", Description: "Show only when database type is postgres"},
			{Value: "app/tls/enabled=true", Description: "Show only when TLS is enabled"},
		},
		Since: "0.2.0",
	},
	{
		Name:        "ui.displayName",
		Namespace:   "ui",
		Description: "Human-readable display name for this value in the UI, shown instead of the raw path.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "Database Host", Description: "Friendly name for database/host"},
		},
		Since: "0.2.0",
	},
	{
		Name:        "ui.enum",
		Namespace:   "ui",
		Description: "Restricts input to a predefined list of allowed values. UI should present these as a selection list.",
		ValueType:   "string[]",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: []string{"postgres", "mysql", "sqlite"}, Description: "Database type options"},
			{Value: []string{"debug", "info", "warn", "error"}, Description: "Log level options"},
		},
		Since: "0.2.0",
	},
	{
		Name:        "ui.section",
		Namespace:   "ui",
		Description: "Assigns this value to a collapsible section in the UI. Values in the same section are rendered under a common heading.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "Connection Settings", Description: "Group under connection settings section"},
			{Value: "Advanced", Description: "Group under advanced section"},
		},
		Since: "0.2.0",
	},
}

// Store namespace - interpreted by store plugins
var storeLabels = []*Label{
	{
		Name:         "store.writeonly",
		Namespace:    "store",
		Description:  "Value can be written but not read back from the store. Useful for passwords and secrets that should never be retrieved.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"store"},
		Examples: []Example{
			{Value: true, Description: "Mark password as write-only"},
		},
		Since: "0.1.0",
	},
	{
		Name:         "store.encrypt",
		Namespace:    "store",
		Description:  "Forces encryption for this specific value even if store-wide encryption is not enabled. Requires store encryption capability.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"store"},
		Since:        "0.1.0",
	},
	{
		Name:         "store.noversion",
		Namespace:    "store",
		Description:  "Excludes this value from version history. Useful for frequently-changing values that don't need history.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"store"},
		Since:        "0.1.0",
	},
	{
		Name:        "store.ttl",
		Namespace:   "store",
		Description: "Time-to-live in seconds. Store may automatically delete the value after this duration.",
		ValueType:   "int",
		AppliesTo:   []string{"store"},
		Constraints: &Constraints{
			Min: ptr(0.0),
		},
		Since: "0.1.0",
	},
	{
		Name:        "store.maxversions",
		Namespace:   "store",
		Description: "Maximum number of versions to retain for this value. Older versions beyond this limit are automatically deleted.",
		ValueType:   "int",
		AppliesTo:   []string{"store"},
		Constraints: &Constraints{
			Min: ptr(1.0),
		},
		Examples: []Example{
			{Value: 5, Description: "Keep at most 5 versions"},
			{Value: 1, Description: "Keep only the latest version (no history)"},
		},
		Since: "0.2.0",
	},
}

// Transform namespace - interpreted by transform plugins
var transformLabels = []*Label{
	{
		Name:         "transform.hidden",
		Namespace:    "transform",
		Description:  "Transform plugins cannot access this value. Useful for sensitive data that should bypass transformations.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"transform"},
		Since:        "0.1.0",
	},
	{
		Name:        "transform.skip",
		Namespace:   "transform",
		Description: "List of transform plugin names to skip for this value.",
		ValueType:   "string[]",
		AppliesTo:   []string{"transform"},
		Examples: []Example{
			{Value: []string{"encryption", "compression"}, Description: "Skip encryption and compression transforms"},
		},
		Since: "0.1.0",
	},
	{
		Name:        "transform.order",
		Namespace:   "transform",
		Description: "Override the default transform order for this value. Lower numbers are processed first.",
		ValueType:   "int",
		AppliesTo:   []string{"transform"},
		Since:       "0.1.0",
	},
}

// Config namespace - interpreted by config plugins
var configLabels = []*Label{
	{
		Name:         "config.required",
		Namespace:    "config",
		Description:  "Value must be present and non-empty. Config plugin validation will fail if missing.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"config"},
		Since:        "0.1.0",
	},
	{
		Name:         "config.immutable",
		Namespace:    "config",
		Description:  "Value cannot be changed after initial creation. Config plugin will reject updates.",
		ValueType:    "bool",
		DefaultValue: false,
		AppliesTo:    []string{"config"},
		Since:        "0.1.0",
	},
	{
		Name:        "config.default",
		Namespace:   "config",
		Description: "Default value to use if the configuration value is not set.",
		ValueType:   "any",
		AppliesTo:   []string{"config"},
		Since:       "0.1.0",
	},
	{
		Name:        "config.env",
		Namespace:   "config",
		Description: "Environment variable name that can override this value.",
		ValueType:   "string",
		AppliesTo:   []string{"config"},
		Examples: []Example{
			{Value: "DATABASE_HOST", Description: "Allow override via DATABASE_HOST env var"},
		},
		Since: "0.1.0",
	},
}

// Core namespace - interpreted by the engine itself
var coreLabels = []*Label{
	{
		Name:        "core.description",
		Namespace:   "core",
		Description: "Human-readable description of what this configuration value does.",
		ValueType:   "string",
		AppliesTo:   []string{"ui", "config"},
		Since:       "0.1.0",
	},
	{
		Name:        "core.type",
		Namespace:   "core",
		Description: "Semantic type hint for the value (e.g., 'email', 'url', 'hostname', 'port', 'filepath').",
		ValueType:   "string",
		Constraints: &Constraints{
			Enum: []string{
				"string", "int", "float", "bool",
				"email", "url", "hostname", "port", "filepath",
				"duration", "bytes", "date", "datetime",
				"ip", "cidr", "uuid",
			},
		},
		AppliesTo: []string{"ui", "config", "store"},
		Since:     "0.1.0",
	},
	{
		Name:        "core.deprecated",
		Namespace:   "core",
		Description: "Marks this configuration value as deprecated with migration guidance.",
		ValueType:   "string",
		AppliesTo:   []string{"ui", "config"},
		Examples: []Example{
			{Value: "Use database/connection_string instead", Description: "Deprecation with migration path"},
		},
		Since: "0.1.0",
	},
	{
		Name:        "core.doc",
		Namespace:   "core",
		Description: "URL to documentation for this configuration value.",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Since:       "0.1.0",
	},
	{
		Name:        "core.example",
		Namespace:   "core",
		Description: "Example value to show users what format is expected.",
		ValueType:   "any",
		AppliesTo:   []string{"ui"},
		Since:       "0.1.0",
	},
	{
		Name:        "core.unit",
		Namespace:   "core",
		Description: "Unit of measurement for the value (e.g., 'seconds', 'bytes', 'percent').",
		ValueType:   "string",
		AppliesTo:   []string{"ui"},
		Examples: []Example{
			{Value: "seconds", Description: "Value is in seconds"},
			{Value: "MB", Description: "Value is in megabytes"},
		},
		Since: "0.1.0",
	},
}

// Well-known label name constants for use in code.
const (
	// UI labels
	LabelUIReadonly    = "ui.readonly"
	LabelUIPassword    = "ui.password"
	LabelUIHidden      = "ui.hidden"
	LabelUIPattern     = "ui.pattern"
	LabelUIMultiline   = "ui.multiline"
	LabelUIOrder       = "ui.order"
	LabelUIGroup       = "ui.group"
	LabelUIPlaceholder = "ui.placeholder"
	LabelUIFormat      = "ui.format"
	LabelUIConfirm     = "ui.confirm"
	LabelUIShowIf      = "ui.showIf"
	LabelUIDisplayName = "ui.displayName"
	LabelUIEnum        = "ui.enum"
	LabelUISection     = "ui.section"

	// Store labels
	LabelStoreWriteonly   = "store.writeonly"
	LabelStoreEncrypt     = "store.encrypt"
	LabelStoreNoVersion   = "store.noversion"
	LabelStoreTTL         = "store.ttl"
	LabelStoreMaxVersions = "store.maxversions"

	// Transform labels
	LabelTransformHidden = "transform.hidden"
	LabelTransformSkip   = "transform.skip"
	LabelTransformOrder  = "transform.order"

	// Config labels
	LabelConfigRequired  = "config.required"
	LabelConfigImmutable = "config.immutable"
	LabelConfigDefault   = "config.default"
	LabelConfigEnv       = "config.env"

	// Core labels
	LabelCoreDescription = "core.description"
	LabelCoreType        = "core.type"
	LabelCoreDeprecated  = "core.deprecated"
	LabelCoreDoc         = "core.doc"
	LabelCoreExample     = "core.example"
	LabelCoreUnit        = "core.unit"
)
