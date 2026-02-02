package store

import (
	"context"
	"encoding/json"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	configpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/store/proto"
)

// GRPCServer implements the proto StoreServiceServer by delegating to a
// Plugin implementation.
type GRPCServer struct {
	pb.UnimplementedStoreServiceServer
	Impl Plugin
}

// --- Capabilities ---

func (s *GRPCServer) Capabilities(ctx context.Context, _ *pb.StoreCapabilitiesRequest) (*pb.StoreCapabilitiesResponse, error) {
	caps, err := s.Impl.Capabilities(ctx)
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.StoreCapabilitiesResponse{
		Versioning:    pb.VersioningMode(caps.Versioning),
		Encryption:    int32(caps.Encryption),
		Auth:          caps.Auth,
		AccessControl: caps.AccessControl,
	}, nil
}

// --- Authentication ---

func (s *GRPCServer) AuthMethods(ctx context.Context, _ *pb.AuthMethodsRequest) (*pb.AuthMethodsResponse, error) {
	methods, err := s.Impl.AuthMethods(ctx)
	if err != nil {
		return nil, errorToStatus(err)
	}
	msgs := make([]*pb.AuthMethodMsg, 0, len(methods))
	for _, m := range methods {
		msg := &pb.AuthMethodMsg{
			Type:        m.Type,
			Description: m.Description,
		}
		for _, f := range m.Fields {
			msg.Fields = append(msg.Fields, &pb.AuthFieldMsg{
				Name:        f.Name,
				Description: f.Description,
				Required:    f.Required,
				Secret:      f.Secret,
			})
		}
		msgs = append(msgs, msg)
	}
	return &pb.AuthMethodsResponse{Methods: msgs}, nil
}

func (s *GRPCServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	cred, err := s.Impl.Login(ctx, req.GetMethod(), req.GetCredentials())
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.LoginResponse{
		Token:     cred.Token,
		ExpiresAt: cred.ExpiresAt,
		Metadata:  cred.Metadata,
	}, nil
}

// --- Tree management ---

func (s *GRPCServer) ListTrees(ctx context.Context, _ *pb.ListTreesRequest) (*pb.ListTreesResponse, error) {
	ids, err := s.Impl.ListTrees(ctx)
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.ListTreesResponse{Ids: ids}, nil
}

func (s *GRPCServer) DeleteTree(ctx context.Context, req *pb.DeleteTreeRequest) (*pb.DeleteTreeResponse, error) {
	if err := s.Impl.DeleteTree(ctx, req.GetId()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.DeleteTreeResponse{}, nil
}

// --- Value operations ---

func (s *GRPCServer) GetValues(ctx context.Context, req *pb.GetValuesRequest) (*pb.GetValuesResponse, error) {
	values, err := s.Impl.GetValues(ctx, req.GetId(), req.GetPaths())
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.GetValuesResponse{Values: valuesToProto(values)}, nil
}

func (s *GRPCServer) PutValues(ctx context.Context, req *pb.PutValuesRequest) (*pb.PutValuesResponse, error) {
	values, err := serverEntriesFromProto(req.GetValues())
	if err != nil {
		return nil, err
	}
	var opts *PutOptions
	if req.GetCasVersion() != "" || len(req.GetCasVersions()) > 0 {
		opts = &PutOptions{
			CASVersion:  req.GetCasVersion(),
			CASVersions: req.GetCasVersions(),
		}
	}
	if err := s.Impl.PutValues(ctx, req.GetId(), values, opts); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.PutValuesResponse{}, nil
}

func (s *GRPCServer) DeleteValues(ctx context.Context, req *pb.DeleteValuesRequest) (*pb.DeleteValuesResponse, error) {
	if err := s.Impl.DeleteValues(ctx, req.GetId(), req.GetPaths()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.DeleteValuesResponse{}, nil
}

// --- Tree-level versioning ---

func (s *GRPCServer) ListTreeVersions(ctx context.Context, req *pb.ListTreeVersionsRequest) (*pb.ListTreeVersionsResponse, error) {
	versions, err := s.Impl.ListTreeVersions(ctx, req.GetId())
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.ListTreeVersionsResponse{Versions: versions}, nil
}

func (s *GRPCServer) GetTreeVersion(ctx context.Context, req *pb.GetTreeVersionRequest) (*pb.GetTreeVersionResponse, error) {
	values, err := s.Impl.GetTreeVersion(ctx, req.GetId(), req.GetVersion(), req.GetPaths())
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.GetTreeVersionResponse{Values: valuesToProto(values)}, nil
}

func (s *GRPCServer) RollbackTree(ctx context.Context, req *pb.RollbackTreeRequest) (*pb.RollbackTreeResponse, error) {
	if err := s.Impl.RollbackTree(ctx, req.GetId(), req.GetVersion()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.RollbackTreeResponse{}, nil
}

func (s *GRPCServer) DeleteTreeVersion(ctx context.Context, req *pb.DeleteTreeVersionRequest) (*pb.DeleteTreeVersionResponse, error) {
	if err := s.Impl.DeleteTreeVersion(ctx, req.GetId(), req.GetVersion()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.DeleteTreeVersionResponse{}, nil
}

// --- Value-level versioning ---

func (s *GRPCServer) ListValueVersions(ctx context.Context, req *pb.ListValueVersionsRequest) (*pb.ListValueVersionsResponse, error) {
	versions, err := s.Impl.ListValueVersions(ctx, req.GetId(), req.GetPath())
	if err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.ListValueVersionsResponse{Versions: versions}, nil
}

func (s *GRPCServer) GetValueVersion(ctx context.Context, req *pb.GetValueVersionRequest) (*pb.GetValueVersionResponse, error) {
	v, found, err := s.Impl.GetValueVersion(ctx, req.GetId(), req.GetPath(), req.GetVersion())
	if err != nil {
		return nil, errorToStatus(err)
	}
	if !found {
		return &pb.GetValueVersionResponse{Found: false}, nil
	}
	valJSON, err := json.Marshal(v.Val)
	if err != nil {
		return nil, err
	}
	var metaJSON []byte
	if v.Metadata != nil {
		metaJSON, err = json.Marshal(v.Metadata)
		if err != nil {
			return nil, err
		}
	}
	return &pb.GetValueVersionResponse{
		Found:        true,
		ValueJson:    valJSON,
		MetadataJson: metaJSON,
	}, nil
}

func (s *GRPCServer) RollbackValue(ctx context.Context, req *pb.RollbackValueRequest) (*pb.RollbackValueResponse, error) {
	if err := s.Impl.RollbackValue(ctx, req.GetId(), req.GetPath(), req.GetVersion()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.RollbackValueResponse{}, nil
}

func (s *GRPCServer) DeleteValueVersion(ctx context.Context, req *pb.DeleteValueVersionRequest) (*pb.DeleteValueVersionResponse, error) {
	if err := s.Impl.DeleteValueVersion(ctx, req.GetId(), req.GetPath(), req.GetVersion()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.DeleteValueVersionResponse{}, nil
}

// --- Encryption ---

func (s *GRPCServer) InitEncryption(ctx context.Context, req *pb.InitEncryptionRequest) (*pb.InitEncryptionResponse, error) {
	if err := s.Impl.InitEncryption(ctx, req.GetPassphrase()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.InitEncryptionResponse{}, nil
}

func (s *GRPCServer) RotateEncryption(ctx context.Context, req *pb.RotateEncryptionRequest) (*pb.RotateEncryptionResponse, error) {
	if err := s.Impl.RotateEncryption(ctx, req.GetOldPassphrase(), req.GetNewPassphrase()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.RotateEncryptionResponse{}, nil
}

// --- Access control ---

func (s *GRPCServer) GrantAccess(ctx context.Context, req *pb.GrantAccessRequest) (*pb.GrantAccessResponse, error) {
	perms := permissionsFromProto(req.GetPermissions())
	if err := s.Impl.GrantAccess(ctx, req.GetId(), req.GetUser(), perms); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.GrantAccessResponse{}, nil
}

func (s *GRPCServer) RevokeAccess(ctx context.Context, req *pb.RevokeAccessRequest) (*pb.RevokeAccessResponse, error) {
	if err := s.Impl.RevokeAccess(ctx, req.GetId(), req.GetUser(), req.GetPaths()); err != nil {
		return nil, errorToStatus(err)
	}
	return &pb.RevokeAccessResponse{}, nil
}

func (s *GRPCServer) ListAccess(ctx context.Context, req *pb.ListAccessRequest) (*pb.ListAccessResponse, error) {
	access, err := s.Impl.ListAccess(ctx, req.GetId())
	if err != nil {
		return nil, errorToStatus(err)
	}
	result := make(map[string]*pb.UserPermissions, len(access))
	for user, perms := range access {
		result[user] = &pb.UserPermissions{
			Permissions: permissionsToProto(perms),
		}
	}
	return &pb.ListAccessResponse{Access: result}, nil
}

// serverEntriesFromProto converts proto TreeEntry messages into a map on
// the server side.
func serverEntriesFromProto(entries []*configpb.TreeEntry) (map[string]config.Value, error) {
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
