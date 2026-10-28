// SPDX-License-Identifier: MIT
package legal

import _ "embed"

//go:embed notices.txt
var Sources string

//go:embed mc-assets-license.txt
var MCAssetsLicense string

func Text() string { return Sources + "\n\nmc-assets license:\n\n" + MCAssetsLicense }
