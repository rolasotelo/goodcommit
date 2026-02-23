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

//go:embed builtin_manifests/description.manifest.json
var builtinDescriptionManifest []byte

//go:embed builtin_manifests/body.manifest.json
var builtinBodyManifest []byte

//go:embed builtin_manifests/conventional-title.manifest.json
var builtinConventionalTitleManifest []byte

var builtinRegistry = map[string]builtinDefinition{
	"builtin/types": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-types"},
		ManifestRaw:   builtinTypesManifest,
	},
	"builtin/description": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-description"},
		ManifestRaw:   builtinDescriptionManifest,
	},
	"builtin/body": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-body"},
		ManifestRaw:   builtinBodyManifest,
	},
	"builtin/conventional-title": {
		DefaultSource: SourceConfig{Type: "path", Path: "./cmd/goodcommit-plugin-conventional-title"},
		ManifestRaw:   builtinConventionalTitleManifest,
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
