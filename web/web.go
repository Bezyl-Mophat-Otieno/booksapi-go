// Package web holds the static payments UI, embedded into the binary so it is
// served the same way regardless of the working directory.
package web

import "embed"

//go:embed index.html styles.css app.js
var Files embed.FS
