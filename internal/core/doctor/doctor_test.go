package doctor

import (
	"slices"
	"testing"
)

func TestParseCategories(t *testing.T) {
	cases := []struct {
		name  string
		in    []string
		want  []Category
		isErr bool
	}{
		{
			name: "empty falls back to defaults",
			in:   nil,
			want: DefaultCategories,
		},
		{
			name: "single category",
			in:   []string{"workspace"},
			want: []Category{CategoryWorkspace},
		},
		{
			name: "dedupes repeated entries",
			in:   []string{"workspace", "plugins", "workspace"},
			want: []Category{CategoryWorkspace, CategoryPlugins},
		},
		{
			name: "all expands to every category",
			in:   []string{"all"},
			want: AllCategories,
		},
		{
			name: "all merges with explicit entries without duplicating",
			in:   []string{"plugins", "all"},
			// order: explicit plugins first, then the remaining categories
			// from AllCategories that weren't already added.
			want: []Category{
				CategoryPlugins,
				CategoryWorkspace,
				CategoryStore,
				CategoryConfig,
				CategoryUpdates,
			},
		},
		{
			name:  "unknown category errors",
			in:    []string{"nonsense"},
			isErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCategories(tc.in)
			if tc.isErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
