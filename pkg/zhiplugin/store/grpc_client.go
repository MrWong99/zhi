package store

import (
	"context"
	"encoding/json"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	configpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/store/proto"
)

// GRPCClient implements Plugin by talking to the gRPC StoreServiceClient
// generated from the proto definition.
type GRPCClient struct {
	client pb.StoreServiceClient
}

// --- Capabilities ---

func (c *GRPCClient) Capabilities(ctx context.Context) (*Capabilities, error) {
	resp, err := c.client.Capabilities(ctx, &pb.StoreCapabilitiesRequest{})
	if err != nil {
		return nil, statusToError(err)
	}
	return &Capabilities{
		Versioning:    VersioningMode(resp.GetVersioning()),
		Encryption:    EncryptionStatus(resp.GetEncryption()),
		Auth:          resp.GetAuth(),
		AccessControl: resp.GetAccessControl(),
	}, nil
}

// --- Authentication ---

func (c *GRPCClient) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
	resp, err := c.client.AuthMethods(ctx, &pb.AuthMethodsRequest{})
	if err != nil {
		return nil, statusToError(err)
	}
	methods := make([]AuthMethod, 0, len(resp.GetMethods()))
	for _, m := range resp.GetMethods() {
		am := AuthMethod{
			Type:        m.GetType(),
			Description: m.GetDescription(),
		}
		for _, f := range m.GetFields() {
			am.Fields = append(am.Fields, AuthField{
				Name:        f.GetName(),
				Description: f.GetDescription(),
				Required:    f.GetRequired(),
				Secret:      f.GetSecret(),
			})
		}
		methods = append(methods, am)
	}
	return methods, nil
}

func (c *GRPCClient) Login(ctx context.Context, method string, credentials map[string]string) (*Credential, error) {
	resp, err := c.client.Login(ctx, &pb.LoginRequest{
		Method:      method,
		Credentials: credentials,
	})
	if err != nil {
		return nil, statusToError(err)
	}
	return &Credential{
		Token:     resp.GetToken(),
		ExpiresAt: resp.GetExpiresAt(),
		Metadata:  resp.GetMetadata(),
	}, nil
}

// --- Tree management ---

func (c *GRPCClient) ListTrees(ctx context.Context) ([]string, error) {
	resp, err := c.client.ListTrees(ctx, &pb.ListTreesRequest{})
	if err != nil {
		return nil, statusToError(err)
	}
	return resp.GetIds(), nil
}

func (c *GRPCClient) DeleteTree(ctx context.Context, id string) error {
	_, err := c.client.DeleteTree(ctx, &pb.DeleteTreeRequest{Id: id})
	return statusToError(err)
}

// --- Value operations ---

func (c *GRPCClient) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	resp, err := c.client.GetValues(ctx, &pb.GetValuesRequest{
		Id:    id,
		Paths: paths,
	})
	if err != nil {
		return nil, statusToError(err)
	}
	return entriesFromProto(resp.GetValues())
}

func (c *GRPCClient) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *PutOptions) error {
	entries := valuesToProto(values)
	req := &pb.PutValuesRequest{
		Id:     id,
		Values: entries,
	}
	if opts != nil {
		req.CasVersion = opts.CASVersion
		req.CasVersions = opts.CASVersions
	}
	_, err := c.client.PutValues(ctx, req)
	return statusToError(err)
}

func (c *GRPCClient) DeleteValues(ctx context.Context, id string, paths []string) error {
	_, err := c.client.DeleteValues(ctx, &pb.DeleteValuesRequest{
		Id:    id,
		Paths: paths,
	})
	return statusToError(err)
}

// --- Tree-level versioning ---

func (c *GRPCClient) ListTreeVersions(ctx context.Context, id string) ([]string, error) {
	resp, err := c.client.ListTreeVersions(ctx, &pb.ListTreeVersionsRequest{Id: id})
	if err != nil {
		return nil, statusToError(err)
	}
	return resp.GetVersions(), nil
}

func (c *GRPCClient) GetTreeVersion(ctx context.Context, id string, version string, paths []string) (map[string]config.Value, error) {
	resp, err := c.client.GetTreeVersion(ctx, &pb.GetTreeVersionRequest{
		Id:      id,
		Version: version,
		Paths:   paths,
	})
	if err != nil {
		return nil, statusToError(err)
	}
	return entriesFromProto(resp.GetValues())
}

func (c *GRPCClient) RollbackTree(ctx context.Context, id string, version string) error {
	_, err := c.client.RollbackTree(ctx, &pb.RollbackTreeRequest{
		Id:      id,
		Version: version,
	})
	return statusToError(err)
}

func (c *GRPCClient) DeleteTreeVersion(ctx context.Context, id string, version string) error {
	_, err := c.client.DeleteTreeVersion(ctx, &pb.DeleteTreeVersionRequest{
		Id:      id,
		Version: version,
	})
	return statusToError(err)
}

// --- Value-level versioning ---

func (c *GRPCClient) ListValueVersions(ctx context.Context, id string, path string) ([]string, error) {
	resp, err := c.client.ListValueVersions(ctx, &pb.ListValueVersionsRequest{
		Id:   id,
		Path: path,
	})
	if err != nil {
		return nil, statusToError(err)
	}
	return resp.GetVersions(), nil
}

func (c *GRPCClient) GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error) {
	resp, err := c.client.GetValueVersion(ctx, &pb.GetValueVersionRequest{
		Id:      id,
		Path:    path,
		Version: version,
	})
	if err != nil {
		return config.Value{}, false, statusToError(err)
	}
	if !resp.GetFound() {
		return config.Value{}, false, nil
	}
	v, err := config.ValueFromProto(resp.GetValueJson(), resp.GetMetadataJson())
	if err != nil {
		return config.Value{}, false, err
	}
	return v, true, nil
}

func (c *GRPCClient) RollbackValue(ctx context.Context, id string, path string, version string) error {
	_, err := c.client.RollbackValue(ctx, &pb.RollbackValueRequest{
		Id:      id,
		Path:    path,
		Version: version,
	})
	return statusToError(err)
}

func (c *GRPCClient) DeleteValueVersion(ctx context.Context, id string, path string, version string) error {
	_, err := c.client.DeleteValueVersion(ctx, &pb.DeleteValueVersionRequest{
		Id:      id,
		Path:    path,
		Version: version,
	})
	return statusToError(err)
}

// --- Encryption ---

func (c *GRPCClient) InitEncryption(ctx context.Context, passphrase []byte) error {
	_, err := c.client.InitEncryption(ctx, &pb.InitEncryptionRequest{
		Passphrase: passphrase,
	})
	return statusToError(err)
}

func (c *GRPCClient) RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error {
	_, err := c.client.RotateEncryption(ctx, &pb.RotateEncryptionRequest{
		OldPassphrase: oldPassphrase,
		NewPassphrase: newPassphrase,
	})
	return statusToError(err)
}

// --- Access control ---

func (c *GRPCClient) GrantAccess(ctx context.Context, id string, user string, permissions []Permission) error {
	_, err := c.client.GrantAccess(ctx, &pb.GrantAccessRequest{
		Id:          id,
		User:        user,
		Permissions: permissionsToProto(permissions),
	})
	return statusToError(err)
}

func (c *GRPCClient) RevokeAccess(ctx context.Context, id string, user string, paths []string) error {
	_, err := c.client.RevokeAccess(ctx, &pb.RevokeAccessRequest{
		Id:    id,
		User:  user,
		Paths: paths,
	})
	return statusToError(err)
}

func (c *GRPCClient) ListAccess(ctx context.Context, id string) (map[string][]Permission, error) {
	resp, err := c.client.ListAccess(ctx, &pb.ListAccessRequest{Id: id})
	if err != nil {
		return nil, statusToError(err)
	}
	result := make(map[string][]Permission, len(resp.GetAccess()))
	for user, up := range resp.GetAccess() {
		result[user] = permissionsFromProto(up.GetPermissions())
	}
	return result, nil
}

// --- helpers ---

// valuesToProto serialises a map of path->Value into proto TreeEntry messages.
func valuesToProto(values map[string]config.Value) []*configpb.TreeEntry {
	entries := make([]*configpb.TreeEntry, 0, len(values))
	for path, v := range values {
		valJSON, err := json.Marshal(v.Val)
		if err != nil {
			continue
		}
		var metaJSON []byte
		if v.Metadata != nil {
			metaJSON, _ = json.Marshal(v.Metadata)
		}
		entries = append(entries, &configpb.TreeEntry{
			Path:         path,
			ValueJson:    valJSON,
			MetadataJson: metaJSON,
		})
	}
	return entries
}

// entriesFromProto converts proto TreeEntry messages back into a map of
// path->Value.
func entriesFromProto(entries []*configpb.TreeEntry) (map[string]config.Value, error) {
	result := make(map[string]config.Value, len(entries))
	for _, e := range entries {
		v, err := config.ValueFromProto(e.GetValueJson(), e.GetMetadataJson())
		if err != nil {
			return nil, err
		}
		result[e.GetPath()] = v
	}
	return result, nil
}

func permissionsToProto(perms []Permission) []*pb.PermissionMsg {
	msgs := make([]*pb.PermissionMsg, 0, len(perms))
	for _, p := range perms {
		actions := make([]pb.ActionType, 0, len(p.Actions))
		for _, a := range p.Actions {
			actions = append(actions, pb.ActionType(a))
		}
		msgs = append(msgs, &pb.PermissionMsg{
			Path:    p.Path,
			Actions: actions,
		})
	}
	return msgs
}

func permissionsFromProto(msgs []*pb.PermissionMsg) []Permission {
	perms := make([]Permission, 0, len(msgs))
	for _, m := range msgs {
		p := Permission{Path: m.GetPath()}
		for _, a := range m.GetActions() {
			p.Actions = append(p.Actions, Action(a))
		}
		perms = append(perms, p)
	}
	return perms
}
