// Command argosd is the Argos CMDB daemon entry point.
package main

import (
	"log/slog"
	"os"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("argosd starting", "version", version)
}
