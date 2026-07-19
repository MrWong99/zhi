package ui

import (
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	cfgpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// treeToProto delegates to config.TreeToProto.
func treeToProto(tree config.TreeReader) ([]*cfgpb.TreeEntry, error) {
	return config.TreeToProto(tree)
}

// treeFromProto delegates to config.TreeFromProto.
func treeFromProto(entries []*cfgpb.TreeEntry) (*config.Tree, error) {
	return config.TreeFromProto(entries)
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
