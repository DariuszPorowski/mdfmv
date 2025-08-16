package mdfmv

import (
	"fmt"
	"runtime"
)

// BuildInfo represents CLI build metadata.
type BuildInfo struct {
	GoVersion string `json:"goVersion"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
}

// The following variables are intended to be overridden via -ldflags at build time.
var (
	version = "dev"
	commit  = "unknown" //nolint:gochecknoglobals // set via -ldflags
	date    = "unknown" //nolint:gochecknoglobals // set via -ldflags
)

// build is constructed from ldflags-overridable variables.
var build = func() BuildInfo { //nolint:gochecknoglobals // exposed as package-level build info
	return BuildInfo{
		GoVersion: runtime.Version(),
		Version:   version,
		Commit:    commit,
		Date:      date,
	}
}()

func (b BuildInfo) String(t string) string {
	return fmt.Sprintf("%s has version %s built with %s from %s on %s",
		t, b.Version, b.GoVersion, b.Commit, b.Date)
}
