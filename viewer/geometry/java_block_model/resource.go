// SPDX-License-Identifier: MIT
package java_block_model

import (
	"fmt"
	"path"
	"strings"
)

// ResourceID is a namespaced resource identifier, for example minecraft:block/stone.
// It names an asset, rather than a filesystem path or a game runtime ID.
type ResourceID string

func ParseResourceID(value string) (ResourceID, error) {
	if !strings.Contains(value, ":") {
		value = "minecraft:" + value
	}
	namespace, name, ok := strings.Cut(value, ":")
	if !ok || namespace == "" || name == "" || strings.ContainsAny(namespace, "/\\") || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid resource ID %q", value)
	}
	return ResourceID(value), nil
}

func (id ResourceID) Namespace() string {
	namespace, _, _ := strings.Cut(string(id), ":")
	return namespace
}
func (id ResourceID) Path() string { _, name, _ := strings.Cut(string(id), ":"); return name }

// UnmarshalText normalizes resource names once at the JSON boundary, including map keys.
func (id *ResourceID) UnmarshalText(text []byte) error {
	value, err := ParseResourceID(string(text))
	if err != nil {
		return err
	}
	*id = value
	return nil
}
