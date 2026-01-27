// Package zhiplugin provides shared constants and types used across all zhi
// plugin types (configuration, storage, provisioning, etc.).
package zhiplugin

import goplugin "github.com/hashicorp/go-plugin"

// Handshake is the handshake configuration that host and plugin must share.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ZHI_PLUGIN",
	MagicCookieValue: "zhiplugin-v1",
}
