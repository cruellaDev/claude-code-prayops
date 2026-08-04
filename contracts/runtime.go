package contracts

import "time"

type RuntimeManifest struct {
	SchemaVersion   int             `json:"schemaVersion"`
	RuntimeVersion  string          `json:"runtimeVersion"`
	Repository      string          `json:"repository"`
	AssetTemplate   string          `json:"assetTemplate"`
	ChecksumAsset   string          `json:"checksumAsset"`
	SupportedTarget []RuntimeTarget `json:"supported"`
}

type RuntimeTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type RuntimeInstallMetadata struct {
	RuntimeVersion string    `json:"runtimeVersion"`
	PluginVersion  string    `json:"pluginVersion"`
	InstalledAt    time.Time `json:"installedAt"`
	Asset          string    `json:"asset"`
	SHA256         string    `json:"sha256"`
}
