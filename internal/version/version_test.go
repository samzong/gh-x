package version

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	mockInfo := func(ver string, ok bool) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			if !ok {
				return nil, false
			}
			return &debug.BuildInfo{
				Main: debug.Module{Version: ver},
			}, true
		}
	}

	cases := []struct {
		name string
		raw  string
		read func() (*debug.BuildInfo, bool)
		want string
	}{
		{
			name: "linker injected overrides build info",
			raw:  "v1.2.3",
			read: mockInfo("v0.0.1", true),
			want: "1.2.3",
		},
		{
			name: "linker injected prerelease",
			raw:  "v1.2.3-rc.1",
			read: mockInfo("(devel)", true),
			want: "1.2.3-rc.1",
		},
		{
			name: "fallback to build info module version",
			raw:  "dev",
			read: mockInfo("v0.4.0-beta.2", true),
			want: "0.4.0-beta.2",
		},
		{
			name: "empty raw falls back to build info",
			raw:  "",
			read: mockInfo("v0.5.0", true),
			want: "0.5.0",
		},
		{
			name: "devel build info returns dev",
			raw:  "dev",
			read: mockInfo("(devel)", true),
			want: "dev",
		},
		{
			name: "missing build info returns dev",
			raw:  "dev",
			read: mockInfo("", false),
			want: "dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.raw, tc.read); got != tc.want {
				t.Fatalf("resolveVersion(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = origVersion, origCommit, origDate
	})

	Version = "v0.3.0-alpha.1"
	Commit = "abc1234"
	Date = "2026-11-25T00:00:00Z"

	want := "0.3.0-alpha.1 (commit abc1234, built at 2026-11-25T00:00:00Z)"
	if got := Format(); got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}
