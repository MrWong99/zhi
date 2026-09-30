package doctor

import (
	"context"
	"fmt"
)

type storeCapabilitiesCheck struct{}

func (storeCapabilitiesCheck) ID() string         { return "store.capabilities" }
func (storeCapabilitiesCheck) Category() Category { return CategoryStore }
func (storeCapabilitiesCheck) Description() string {
	return "Store plugin responds to Capabilities() RPC."
}

func (c storeCapabilitiesCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}

	if env == nil || env.Workspace == nil || env.Workspace.Store.Provider == "" {
		res.Status = StatusSkipped
		res.Message = "no store configured"
		return []Result{res}
	}
	if env.Engine == nil {
		res.Status = StatusSkipped
		res.Message = "engine not initialized"
		if env.EngineErr != nil {
			res.Detail = env.EngineErr.Error()
		}
		return []Result{res}
	}

	caps, err := env.Engine.StoreCapabilities(ctx)
	if err != nil {
		res.Status = StatusError
		res.Message = "store Capabilities() failed"
		res.Detail = err.Error()
		res.Hint = "store plugin may be misconfigured or unreachable"
		return []Result{res}
	}

	res.Status = StatusOK
	res.Message = fmt.Sprintf("versioning=%s, encryption=%s, auth=%t, access_control=%t",
		caps.Versioning, caps.Encryption, caps.Auth, caps.AccessControl)
	return []Result{res}
}

type storeListTreesCheck struct{}

func (storeListTreesCheck) ID() string         { return "store.list_trees" }
func (storeListTreesCheck) Category() Category { return CategoryStore }
func (storeListTreesCheck) Description() string {
	return "Store plugin responds to ListTrees() — proves credentials and read access."
}

func (c storeListTreesCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}

	if env == nil || env.Workspace == nil || env.Workspace.Store.Provider == "" {
		res.Status = StatusSkipped
		res.Message = "no store configured"
		return []Result{res}
	}
	if env.Engine == nil {
		res.Status = StatusSkipped
		res.Message = "engine not initialized"
		if env.EngineErr != nil {
			res.Detail = env.EngineErr.Error()
		}
		return []Result{res}
	}

	trees, err := env.Engine.ListTrees(ctx)
	if err != nil {
		res.Status = StatusError
		res.Message = "store ListTrees() failed"
		res.Detail = err.Error()
		res.Hint = "check auth credentials if this is an authenticated store"
		return []Result{res}
	}

	res.Status = StatusOK
	res.Message = fmt.Sprintf("%d tree(s) accessible", len(trees))
	return []Result{res}
}
