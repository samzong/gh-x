package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/samzong/gh-x/internal/version"
)

func TestRootCommandVersionFlag(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute(--version) failed: %v", err)
	}

	want := "x version " + version.Format()
	if got := strings.TrimSpace(out.String()); got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}
