// SPDX-License-Identifier: MIT
package asset

import (
	"fmt"
	"path"
	"strings"
)

// ID is a namespaced resource identifier, for example minecraft:block/stone.
// It names an asset, rather than a filesystem path or a game runtime ID.
type ID string

func ParseID(value string) (ID, error) {
	if !strings.Contains(value, ":") {
		value = "minecraft:" + value
	}
	namespace, name, ok := strings.Cut(value, ":")
	if !ok || namespace == "" || name == "" || strings.ContainsAny(namespace, "/\\") || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid resource ID %q", value)
	}
	return ID(value), nil
}

func (id ID) Namespace() string { namespace, _, _ := strings.Cut(string(id), ":"); return namespace }
func (id ID) Path() string      { _, name, _ := strings.Cut(string(id), ":"); return name }

// ExportName follows mc-assets: omit the vanilla namespace, retain other ones.
func (id ID) ExportName() string {
	if id.Namespace() == "minecraft" {
		return id.Path()
	}
	return string(id)
}

// TextureName matches mc-assets atlas keys: block/stone becomes stone, while
// entity/bed/red remains entity/bed/red. Other namespaces stay explicit.
func (id ID) TextureName() string {
	name := id.Path()
	name = strings.TrimPrefix(strings.TrimPrefix(name, "block/"), "blocks/")
	if id.Namespace() != "minecraft" {
		name = id.Namespace() + ":" + name
	}
	return name
}
