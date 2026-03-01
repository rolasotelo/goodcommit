package pluginruntime

import (
	_ "embed"
	"fmt"
)

type builtinDefinition struct {
	DefaultSource SourceConfig
	ManifestRaw   []byte
}

//go:embed builtin_manifests/types.manifest.json
var builtinTypesManifest []byte

//go:embed builtin_manifests/logo.manifest.json
var builtinLogoManifest []byte

//go:embed builtin_manifests/scopes.manifest.json
var builtinScopesManifest []byte

//go:embed builtin_manifests/description.manifest.json
var builtinDescriptionManifest []byte

//go:embed builtin_manifests/why.manifest.json
var builtinWhyManifest []byte

//go:embed builtin_manifests/body.manifest.json
var builtinBodyManifest []byte

//go:embed builtin_manifests/breaking.manifest.json
var builtinBreakingManifest []byte

//go:embed builtin_manifests/breakingmsg.manifest.json
var builtinBreakingMsgManifest []byte

//go:embed builtin_manifests/coauthors.manifest.json
var builtinCoauthorsManifest []byte

//go:embed builtin_manifests/conventional-title.manifest.json
var builtinConventionalTitleManifest []byte

//go:embed builtin_manifests/signedoffby.manifest.json
var builtinSignedOffByManifest []byte

var builtinRegistry = map[string]builtinDefinition{
	"builtin/logo": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-logo"},
		ManifestRaw:   builtinLogoManifest,
	},
	"builtin/types": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-types"},
		ManifestRaw:   builtinTypesManifest,
	},
	"builtin/scopes": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-scopes"},
		ManifestRaw:   builtinScopesManifest,
	},
	"builtin/description": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-description"},
		ManifestRaw:   builtinDescriptionManifest,
	},
	"builtin/why": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-why"},
		ManifestRaw:   builtinWhyManifest,
	},
	"builtin/body": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-body"},
		ManifestRaw:   builtinBodyManifest,
	},
	"builtin/breaking": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-breaking"},
		ManifestRaw:   builtinBreakingManifest,
	},
	"builtin/breakingmsg": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-breakingmsg"},
		ManifestRaw:   builtinBreakingMsgManifest,
	},
	"builtin/coauthors": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-coauthors"},
		ManifestRaw:   builtinCoauthorsManifest,
	},
	"builtin/conventional-title": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-conventional-title"},
		ManifestRaw:   builtinConventionalTitleManifest,
	},
	"builtin/signedoffby": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-signedoffby"},
		ManifestRaw:   builtinSignedOffByManifest,
	},
}

func builtinByID(id string) (builtinDefinition, bool) {
	d, ok := builtinRegistry[id]
	return d, ok
}

func builtinManifest(id string) (Manifest, string, error) {
	def, ok := builtinByID(id)
	if !ok {
		return Manifest{}, "", fmt.Errorf("unknown builtin plugin %q", id)
	}
	m, err := ParseManifest(def.ManifestRaw)
	if err != nil {
		return Manifest{}, "", fmt.Errorf("parse builtin manifest %s: %w", id, err)
	}
	return m, BytesSHA256(def.ManifestRaw), nil
}
