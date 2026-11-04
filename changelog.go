// Package vedabasetui holds the files built into the program from the
// repository root.
package vedabasetui

import _ "embed"

// Changelog is CHANGELOG.md, shown by the app as "What's new" after an update.
//
//go:embed CHANGELOG.md
var Changelog string
