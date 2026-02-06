package ui

import (
	"encoding/json"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	cfgpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// treeToProto serialises a config.TreeReader into proto TreeEntry messages.
func treeToProto(tree config.TreeReader) []*cfgpb.TreeEntry {
	paths := tree.List()
	entries := make([]*cfgpb.TreeEntry, 0, len(paths))
	for _, p := range paths {
		v, ok := tree.Get(p)
		if !ok {
			continue
		}
		valJSON, err := json.Marshal(v.Val)
		if err != nil {
			continue
		}
		var metaJSON []byte
		if v.Metadata != nil {
			metaJSON, _ = json.Marshal(v.Metadata)
		}
		entries = append(entries, &cfgpb.TreeEntry{
			Path:         p,
			ValueJson:    valJSON,
			MetadataJson: metaJSON,
		})
	}
	return entries
}

// treeFromProto reconstructs a config.Tree from proto TreeEntry messages.
func treeFromProto(entries []*cfgpb.TreeEntry) *config.Tree {
	t := config.NewTree()
	for _, e := range entries {
		v, err := valueFromProto(e.GetValueJson(), e.GetMetadataJson())
		if err != nil {
			continue
		}
		// Paths were validated at Set time; re-validation is acceptable.
		_ = t.Set(e.GetPath(), &v)
	}
	return t
}

// valueFromProto reconstructs a config.Value from JSON-encoded bytes.
func valueFromProto(valJSON, metaJSON []byte) (config.Value, error) {
	var v config.Value
	if len(valJSON) > 0 {
		if err := json.Unmarshal(valJSON, &v.Val); err != nil {
			return config.Value{}, err
		}
	}
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &v.Metadata); err != nil {
			return config.Value{}, err
		}
	}
	return v, nil
}

// marketplaceEntryFromProto converts a proto marketplace entry to the Go type.
func marketplaceEntryFromProto(m *pb.CtrlMarketplaceEntryMsg) MarketplaceEntry {
	return MarketplaceEntry{
		Name:          m.GetName(),
		Publisher:     m.GetPublisher(),
		Type:          m.GetType(),
		Description:   m.GetDescription(),
		LatestVersion: m.GetLatestVersion(),
		Rating:        m.GetRating(),
		RatingCount:   int(m.GetRatingCount()),
		Downloads:     int(m.GetDownloads()),
		Verified:      m.GetVerified(),
		Installed:     m.GetInstalled(),
		InstalledVer:  m.GetInstalledVer(),
		UpdateAvail:   m.GetUpdateAvail(),
		Platforms:     m.GetPlatforms(),
	}
}

// marketplaceEntryToProto converts a Go marketplace entry to the proto type.
func marketplaceEntryToProto(e MarketplaceEntry) *pb.CtrlMarketplaceEntryMsg {
	return &pb.CtrlMarketplaceEntryMsg{
		Name:          e.Name,
		Publisher:     e.Publisher,
		Type:          e.Type,
		Description:   e.Description,
		LatestVersion: e.LatestVersion,
		Rating:        e.Rating,
		RatingCount:   int32(e.RatingCount),
		Downloads:     int32(e.Downloads),
		Verified:      e.Verified,
		Installed:     e.Installed,
		InstalledVer:  e.InstalledVer,
		UpdateAvail:   e.UpdateAvail,
		Platforms:     e.Platforms,
	}
}

// versionEntryFromProto converts a proto version entry to the Go type.
func versionEntryFromProto(v *pb.CtrlVersionEntryMsg) VersionEntry {
	return VersionEntry{
		Version:   v.GetVersion(),
		CreatedAt: time.Unix(v.GetCreatedAtUnix(), 0),
		Digest:    v.GetDigest(),
		Platforms: v.GetPlatforms(),
	}
}

// versionEntryToProto converts a Go version entry to the proto type.
func versionEntryToProto(v VersionEntry) *pb.CtrlVersionEntryMsg {
	return &pb.CtrlVersionEntryMsg{
		Version:       v.Version,
		CreatedAtUnix: v.CreatedAt.Unix(),
		Digest:        v.Digest,
		Platforms:     v.Platforms,
	}
}

// ratingEntryFromProto converts a proto rating entry to the Go type.
func ratingEntryFromProto(r *pb.CtrlRatingEntryMsg) RatingEntry {
	return RatingEntry{
		Score:     int(r.GetScore()),
		Comment:   r.GetComment(),
		Author:    r.GetAuthor(),
		CreatedAt: time.Unix(r.GetCreatedAtUnix(), 0),
	}
}

// ratingEntryToProto converts a Go rating entry to the proto type.
func ratingEntryToProto(r RatingEntry) *pb.CtrlRatingEntryMsg {
	return &pb.CtrlRatingEntryMsg{
		Score:         int32(r.Score),
		Comment:       r.Comment,
		Author:        r.Author,
		CreatedAtUnix: r.CreatedAt.Unix(),
	}
}

// installedPluginFromProto converts a proto installed plugin to the Go type.
func installedPluginFromProto(m *pb.CtrlInstalledPluginMsg) InstalledPlugin {
	return InstalledPlugin{
		Name:        m.GetName(),
		Type:        m.GetType(),
		Version:     m.GetVersion(),
		Source:      m.GetSource(),
		InstalledAt: time.Unix(m.GetInstalledAtUnix(), 0),
		Digest:      m.GetDigest(),
		Verified:    m.GetVerified(),
		UpdateAvail: m.GetUpdateAvail(),
	}
}

// installedPluginToProto converts a Go installed plugin to the proto type.
func installedPluginToProto(p InstalledPlugin) *pb.CtrlInstalledPluginMsg {
	return &pb.CtrlInstalledPluginMsg{
		Name:            p.Name,
		Type:            p.Type,
		Version:         p.Version,
		Source:          p.Source,
		InstalledAtUnix: p.InstalledAt.Unix(),
		Digest:          p.Digest,
		Verified:        p.Verified,
		UpdateAvail:     p.UpdateAvail,
	}
}
