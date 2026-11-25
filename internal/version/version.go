package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func ResolvedVersion() string {
	return resolveVersion(Version, debug.ReadBuildInfo)
}

func resolveVersion(raw string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if raw != "" && raw != "dev" {
		return strings.TrimPrefix(raw, "v")
	}
	if readBuildInfo == nil {
		return "dev"
	}
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}

func Format() string {
	return fmt.Sprintf("%s (commit %s, built at %s)", ResolvedVersion(), Commit, Date)
}
