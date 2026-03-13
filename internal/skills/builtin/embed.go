package builtin

import "embed"

//go:embed step-signal
var StepSignalFS embed.FS

//go:embed all:*
var AllBuiltinFS embed.FS
