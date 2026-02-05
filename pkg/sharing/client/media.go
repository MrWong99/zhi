// Package client provides an OCI artifact client for pulling and pushing
// zhi plugin artifacts using oras-go v2.
package client

// OCI media types for zhi plugin artifacts.
const (
	MediaTypePluginConfig  = "application/vnd.zhi.plugin.config.v1+json"
	MediaTypePluginBinary  = "application/vnd.zhi.plugin.binary.v1"
	MediaTypePluginRuntime = "application/vnd.zhi.plugin.runtime.v1+tar.gz"
	MediaTypePluginReadme  = "application/vnd.zhi.plugin.readme.v1+markdown"
	MediaTypePluginLicense = "application/vnd.zhi.plugin.license.v1"
)

// OCI media types for zhi workspace artifacts.
const (
	MediaTypeWorkspaceConfig = "application/vnd.zhi.workspace.config.v1+json"
	MediaTypeWorkspaceBundle = "application/vnd.zhi.workspace.bundle.v1+tar.gz"
	MediaTypeWorkspaceReadme = "application/vnd.zhi.workspace.readme.v1+markdown"
)
