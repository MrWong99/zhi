package transform_test

import (
	"context"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

type mockTransformPlugin struct {
	beforeDisplayCalled bool
	afterSaveCalled     bool
	policy              transform.ValidatePolicy
}

func (m *mockTransformPlugin) BeforeDisplay(_ context.Context, _ *config.Tree) error {
	m.beforeDisplayCalled = true
	return nil
}

func (m *mockTransformPlugin) AfterSave(_ context.Context, _ *config.Tree) error {
	m.afterSaveCalled = true
	return nil
}

func (m *mockTransformPlugin) ValidatePolicy(_ context.Context) (transform.ValidatePolicy, error) {
	return m.policy, nil
}

func TestTransformDelegatingPlugin_Delegates(t *testing.T) {
	t.Parallel()
	mock := &mockTransformPlugin{policy: transform.ValidateAfterTransform}
	dp := transform.NewDelegatingPlugin(mock)
	ctx := context.Background()
	tree := config.NewTree()

	if err := dp.BeforeDisplay(ctx, tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}
	if !mock.beforeDisplayCalled {
		t.Error("BeforeDisplay was not delegated")
	}

	if err := dp.AfterSave(ctx, tree); err != nil {
		t.Fatalf("AfterSave: %v", err)
	}
	if !mock.afterSaveCalled {
		t.Error("AfterSave was not delegated")
	}

	policy, err := dp.ValidatePolicy(ctx)
	if err != nil {
		t.Fatalf("ValidatePolicy: %v", err)
	}
	if policy != transform.ValidateAfterTransform {
		t.Errorf("ValidatePolicy = %v, want ValidateAfterTransform", policy)
	}
}

func TestTransformNewDelegatingPlugin_PanicsOnNil(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil base")
		}
	}()
	transform.NewDelegatingPlugin(nil)
}
