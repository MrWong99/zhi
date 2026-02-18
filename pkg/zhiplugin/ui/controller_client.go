package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// controllerGRPCClient implements Controller by calling the
// UIControllerServiceClient. It runs on the plugin side, connected to
// the host via the GRPCBroker.
type controllerGRPCClient struct {
	client pb.UIControllerServiceClient
}

func (c *controllerGRPCClient) LoadTree(ctx context.Context) (*config.Tree, error) {
	resp, err := c.client.LoadTree(ctx, &pb.CtrlLoadTreeRequest{})
	if err != nil {
		return nil, err
	}
	return treeFromProto(resp.GetTree()), nil
}

func (c *controllerGRPCClient) SetValue(ctx context.Context, path string, value config.Value) error {
	valJSON, err := json.Marshal(value.Val)
	if err != nil {
		return err
	}
	var metaJSON []byte
	if value.Metadata != nil {
		metaJSON, err = json.Marshal(value.Metadata)
		if err != nil {
			return err
		}
	}
	_, err = c.client.SetValue(ctx, &pb.CtrlSetValueRequest{
		Path:         path,
		ValueJson:    valJSON,
		MetadataJson: metaJSON,
	})
	return err
}

func (c *controllerGRPCClient) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	resp, err := c.client.Validate(ctx, &pb.CtrlValidateRequest{})
	if err != nil {
		return nil, err
	}
	results := make([]config.ValidationResult, 0, len(resp.GetResults()))
	for _, m := range resp.GetResults() {
		r := config.ValidationResult{
			Severity: config.Severity(m.GetSeverity()),
			Message:  m.GetMessage(),
		}
		if len(m.GetMetadataJson()) > 0 {
			if err := json.Unmarshal(m.GetMetadataJson(), &r.Metadata); err != nil {
				return nil, err
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func (c *controllerGRPCClient) SaveTree(ctx context.Context) error {
	_, err := c.client.SaveTree(ctx, &pb.CtrlSaveTreeRequest{})
	return err
}

func (c *controllerGRPCClient) ExportTemplates(ctx context.Context) ([]ExportTemplate, error) {
	resp, err := c.client.ExportTemplates(ctx, &pb.CtrlExportTemplatesRequest{})
	if err != nil {
		return nil, err
	}
	templates := make([]ExportTemplate, 0, len(resp.GetTemplates()))
	for _, m := range resp.GetTemplates() {
		templates = append(templates, ExportTemplate{
			Name:     m.GetName(),
			Template: m.GetTemplate(),
			Format:   m.GetFormat(),
			Output:   m.GetOutput(),
			Prefix:   m.GetPrefix(),
		})
	}
	return templates, nil
}

func (c *controllerGRPCClient) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	resp, err := c.client.Export(ctx, &pb.CtrlExportRequest{
		TemplatePath: req.TemplatePath,
		Format:       req.Format,
		OutputPath:   req.OutputPath,
		Prefix:       req.Prefix,
		DryRun:       req.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &ExportResult{
		Name:       resp.GetName(),
		Content:    resp.GetContent(),
		OutputPath: resp.GetOutputPath(),
	}, nil
}

func (c *controllerGRPCClient) Apply(ctx context.Context, target string, handler func(ApplyEvent)) (*ApplyResult, error) {
	stream, err := c.client.Apply(ctx, &pb.CtrlApplyRequest{Target: target})
	if err != nil {
		return nil, err
	}

	var result *ApplyResult
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if resp.GetDone() {
			result = &ApplyResult{
				ExitCode: int(resp.GetExitCode()),
				Error:    resp.GetError(),
			}
			break
		}
		if handler != nil {
			handler(ApplyEvent{
				Line:   resp.GetLine(),
				Stream: resp.GetStream(),
			})
		}
	}
	if result == nil {
		result = &ApplyResult{}
	}
	return result, nil
}

func (c *controllerGRPCClient) ListComponents(ctx context.Context) ([]ComponentInfo, error) {
	resp, err := c.client.ListComponents(ctx, &pb.CtrlListComponentsRequest{})
	if err != nil {
		return nil, err
	}
	components := make([]ComponentInfo, 0, len(resp.GetComponents()))
	for _, m := range resp.GetComponents() {
		components = append(components, ComponentInfo{
			Name:         m.GetName(),
			Description:  m.GetDescription(),
			Enabled:      m.GetEnabled(),
			Mandatory:    m.GetMandatory(),
			Paths:        m.GetPaths(),
			Dependencies: m.GetDependencies(),
		})
	}
	return components, nil
}

func (c *controllerGRPCClient) EnableComponent(ctx context.Context, name string) ([]string, error) {
	resp, err := c.client.EnableComponent(ctx, &pb.CtrlEnableComponentRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return resp.GetEnabled(), nil
}

func (c *controllerGRPCClient) DisableComponent(ctx context.Context, name string) error {
	_, err := c.client.DisableComponent(ctx, &pb.CtrlDisableComponentRequest{Name: name})
	return err
}

func (c *controllerGRPCClient) WorkspaceName(ctx context.Context) (string, error) {
	resp, err := c.client.WorkspaceName(ctx, &pb.CtrlWorkspaceNameRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetName(), nil
}

func (c *controllerGRPCClient) SearchMarketplace(ctx context.Context, query MarketplaceQuery) (*MarketplaceResults, error) {
	if query.Page < 0 || query.Page > math.MaxInt32 {
		return nil, fmt.Errorf("page value %d out of int32 range", query.Page)
	}
	if query.PerPage < 0 || query.PerPage > math.MaxInt32 {
		return nil, fmt.Errorf("perPage value %d out of int32 range", query.PerPage)
	}
	resp, err := c.client.SearchMarketplace(ctx, &pb.CtrlSearchMarketplaceRequest{
		Query:    query.Query,
		Type:     query.Type,
		Sort:     query.Sort,
		Verified: query.Verified,
		Page:     int32(query.Page),
		PerPage:  int32(query.PerPage),
	})
	if err != nil {
		return nil, err
	}
	results := &MarketplaceResults{Total: int(resp.GetTotal())}
	for _, m := range resp.GetResults() {
		results.Results = append(results.Results, marketplaceEntryFromProto(m))
	}
	return results, nil
}

func (c *controllerGRPCClient) GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*MarketplaceDetail, error) {
	resp, err := c.client.GetMarketplaceDetail(ctx, &pb.CtrlGetMarketplaceDetailRequest{
		PluginType: pluginType,
		Publisher:  publisher,
		Name:       name,
	})
	if err != nil {
		return nil, err
	}
	detail := &MarketplaceDetail{
		LongDescription: resp.GetLongDescription(),
		License:         resp.GetLicense(),
		Homepage:        resp.GetHomepage(),
		Repository:      resp.GetRepository(),
		Keywords:        resp.GetKeywords(),
	}
	if resp.GetEntry() != nil {
		detail.MarketplaceEntry = marketplaceEntryFromProto(resp.GetEntry())
	}
	for _, v := range resp.GetVersions() {
		detail.Versions = append(detail.Versions, versionEntryFromProto(v))
	}
	for _, r := range resp.GetRatings() {
		detail.Ratings = append(detail.Ratings, ratingEntryFromProto(r))
	}
	for _, d := range resp.GetDependencies() {
		detail.Dependencies = append(detail.Dependencies, DependencyEntry{
			Name:      d.GetName(),
			Type:      d.GetType(),
			Publisher: d.GetPublisher(),
			Required:  d.GetRequired(),
		})
	}
	return detail, nil
}

func (c *controllerGRPCClient) InstallPlugin(ctx context.Context, ref string) (*InstallResult, error) {
	resp, err := c.client.InstallPlugin(ctx, &pb.CtrlInstallPluginRequest{Ref: ref})
	if err != nil {
		return nil, err
	}
	return &InstallResult{
		Name:        resp.GetName(),
		Type:        resp.GetType(),
		Version:     resp.GetVersion(),
		PrevVersion: resp.GetPrevVersion(),
		Digest:      resp.GetDigest(),
		Verified:    resp.GetVerified(),
		RuntimeDeps: resp.GetRuntimeDeps(),
	}, nil
}

func (c *controllerGRPCClient) UninstallPlugin(ctx context.Context, name string, pluginType string) error {
	_, err := c.client.UninstallPlugin(ctx, &pb.CtrlUninstallPluginRequest{
		Name:       name,
		PluginType: pluginType,
	})
	return err
}

func (c *controllerGRPCClient) ListInstalledPlugins(ctx context.Context) ([]InstalledPlugin, error) {
	resp, err := c.client.ListInstalledPlugins(ctx, &pb.CtrlListInstalledPluginsRequest{})
	if err != nil {
		return nil, err
	}
	plugins := make([]InstalledPlugin, 0, len(resp.GetPlugins()))
	for _, m := range resp.GetPlugins() {
		plugins = append(plugins, installedPluginFromProto(m))
	}
	return plugins, nil
}

func (c *controllerGRPCClient) CheckUpdates(ctx context.Context) ([]PluginUpdate, error) {
	resp, err := c.client.CheckUpdates(ctx, &pb.CtrlCheckUpdatesRequest{})
	if err != nil {
		return nil, err
	}
	updates := make([]PluginUpdate, 0, len(resp.GetUpdates()))
	for _, m := range resp.GetUpdates() {
		updates = append(updates, PluginUpdate{
			Name:           m.GetName(),
			Type:           m.GetType(),
			CurrentVersion: m.GetCurrentVersion(),
			LatestVersion:  m.GetLatestVersion(),
			Verified:       m.GetVerified(),
		})
	}
	return updates, nil
}

func (c *controllerGRPCClient) UpdatePlugin(ctx context.Context, name string, version string) (*InstallResult, error) {
	resp, err := c.client.UpdatePlugin(ctx, &pb.CtrlUpdatePluginRequest{
		Name:    name,
		Version: version,
	})
	if err != nil {
		return nil, err
	}
	return &InstallResult{
		Name:        resp.GetName(),
		Type:        resp.GetType(),
		Version:     resp.GetVersion(),
		PrevVersion: resp.GetPrevVersion(),
		Digest:      resp.GetDigest(),
		Verified:    resp.GetVerified(),
		RuntimeDeps: resp.GetRuntimeDeps(),
	}, nil
}

func (c *controllerGRPCClient) RatePlugin(ctx context.Context, pluginType, publisher, name string, rating Rating) error {
	_, err := c.client.RatePlugin(ctx, &pb.CtrlRatePluginRequest{
		PluginType: pluginType,
		Publisher:  publisher,
		Name:       name,
		Score:      int32(rating.Score),
		Comment:    rating.Comment,
	})
	return err
}

func (c *controllerGRPCClient) StoreAuthMethods(ctx context.Context) ([]StoreAuthMethod, error) {
	resp, err := c.client.StoreAuthMethods(ctx, &pb.CtrlStoreAuthMethodsRequest{})
	if err != nil {
		return nil, err
	}
	methods := make([]StoreAuthMethod, 0, len(resp.GetMethods()))
	for _, m := range resp.GetMethods() {
		fields := make([]StoreAuthField, 0, len(m.GetFields()))
		for _, f := range m.GetFields() {
			fields = append(fields, StoreAuthField{
				Name:        f.GetName(),
				Description: f.GetDescription(),
				Required:    f.GetRequired(),
				Secret:      f.GetSecret(),
			})
		}
		methods = append(methods, StoreAuthMethod{
			Type:        m.GetType(),
			Description: m.GetDescription(),
			Fields:      fields,
			Interactive: m.GetInteractive(),
		})
	}
	return methods, nil
}

func (c *controllerGRPCClient) StoreLogin(ctx context.Context, method string, credentials map[string]string) (*StoreSession, error) {
	resp, err := c.client.StoreLogin(ctx, &pb.CtrlStoreLoginRequest{
		Method:      method,
		Credentials: credentials,
	})
	if err != nil {
		return nil, err
	}
	return &StoreSession{
		SessionID: resp.GetSessionId(),
		Status:    StoreSessionStatus(resp.GetStatus()),
		ExpiresAt: resp.GetExpiresAt(),
		Metadata:  resp.GetMetadata(),
	}, nil
}

func (c *controllerGRPCClient) StoreLoginInteractive(ctx context.Context, method string, params map[string]string) (*StoreInteractiveChallenge, error) {
	resp, err := c.client.StoreLoginInteractive(ctx, &pb.CtrlStoreLoginInteractiveRequest{
		Method: method,
		Params: params,
	})
	if err != nil {
		return nil, err
	}
	return &StoreInteractiveChallenge{
		ChallengeID: resp.GetChallengeId(),
		AuthURL:     resp.GetAuthUrl(),
		ExpiresAt:   resp.GetExpiresAt(),
	}, nil
}

func (c *controllerGRPCClient) StoreLoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*StoreSession, error) {
	resp, err := c.client.StoreLoginInteractiveCallback(ctx, &pb.CtrlStoreLoginInteractiveCallbackRequest{
		ChallengeId:    challengeID,
		CallbackParams: callbackParams,
	})
	if err != nil {
		return nil, err
	}
	return &StoreSession{
		SessionID: resp.GetSessionId(),
		Status:    StoreSessionStatus(resp.GetStatus()),
		ExpiresAt: resp.GetExpiresAt(),
		Metadata:  resp.GetMetadata(),
	}, nil
}

func (c *controllerGRPCClient) StoreAuthStatus(ctx context.Context) (*StoreSession, error) {
	resp, err := c.client.StoreAuthStatus(ctx, &pb.CtrlStoreAuthStatusRequest{})
	if err != nil {
		return nil, err
	}
	return &StoreSession{
		SessionID: resp.GetSessionId(),
		Status:    StoreSessionStatus(resp.GetStatus()),
		ExpiresAt: resp.GetExpiresAt(),
		Metadata:  resp.GetMetadata(),
	}, nil
}

func (c *controllerGRPCClient) StoreLogout(ctx context.Context) error {
	_, err := c.client.StoreLogout(ctx, &pb.CtrlStoreLogoutRequest{})
	return err
}
