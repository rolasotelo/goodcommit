package pluginruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func ReadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return ParseManifest(raw)
}

func ReadManifestWithRaw(path string) (Manifest, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, nil, err
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return Manifest{}, nil, err
	}
	return m, raw, nil
}

func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, err
	}
	if err := normalizeAndValidateProtocol(&m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func SupportsProtocol(m Manifest, protocol string) bool {
	for _, v := range m.ProtocolVersions {
		if v == protocol {
			return true
		}
	}
	return false
}

func normalizeAndValidateProtocol(m *Manifest) error {
	if len(m.ProtocolVersions) == 0 {
		m.ProtocolVersions = []string{ProtocolVersionV1}
	}
	if SupportsProtocol(*m, ProtocolVersionV1) {
		return nil
	}
	return fmt.Errorf("manifest %s does not support protocol version %s", m.ID, ProtocolVersionV1)
}
