package config

import "testing"

// TestUnsafeTreeReaderGetReturnsMetadataCopy verifies that the lock-free
// reader handed to validators returns a copy whose Metadata map is not the
// tree's live map, honouring the TreeReader "returned Values are copies"
// contract and preventing a data race / silent corruption.
func TestUnsafeTreeReaderGetReturnsMetadataCopy(t *testing.T) {
	tree := NewTree()
	if err := tree.Set("a/target", &Value{
		Val:      "value",
		Metadata: map[string]any{"orig": true},
	}); err != nil {
		t.Fatalf("Set target: %v", err)
	}

	if err := tree.Set("a/mutator", &Value{
		Val: "trigger",
		Validators: []ValidateFunc{
			func(_ any, tr TreeReader) []ValidationResult {
				if v, ok := tr.Get("a/target"); ok {
					// Legal per the documented contract: mutate the copy.
					v.Metadata["injected"] = true
				}
				return nil
			},
		},
	}); err != nil {
		t.Fatalf("Set mutator: %v", err)
	}

	tree.Validate()

	got, ok := tree.Get("a/target")
	if !ok {
		t.Fatal("target missing after Validate")
	}
	if _, injected := got.Metadata["injected"]; injected {
		t.Error("validator mutation leaked into the tree's live Metadata map")
	}
}
