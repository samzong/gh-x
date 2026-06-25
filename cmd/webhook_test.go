package cmd

import (
	"reflect"
	"testing"
)

func TestExpandRepoTargetsDeduplicatesExplicitRepos(t *testing.T) {
	repos, err := expandRepoTargets([]string{"samzong/gh-x", "samzong/gh-x", "cli/cli"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"samzong/gh-x", "cli/cli"}
	if !reflect.DeepEqual(repos, want) {
		t.Fatalf("repos = %v, want %v", repos, want)
	}
}

func TestRepoHookRows(t *testing.T) {
	rows := repoHookRows("samzong/gh-x", "")
	want := []string{"samzong/gh-x\t-\t-\t-\t-"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}

	rows = repoHookRows("samzong/adit", "1\ttrue\tpush\thttps://example.com")
	want = []string{"samzong/adit\t1\ttrue\tpush\thttps://example.com"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestRepoListArgsOptions(t *testing.T) {
	args, err := repoListArgs("samzong", repoListOptions{source: true, private: true})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"repo", "list", "samzong", "--limit", "1000", "--source", "--visibility", "private", "--json", "nameWithOwner", "-q", ".[].nameWithOwner"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}

	if _, err := repoListArgs("samzong", repoListOptions{source: true, fork: true}); err == nil {
		t.Fatal("expected source/fork conflict")
	}
	if _, err := repoListArgs("samzong", repoListOptions{private: true, public: true}); err == nil {
		t.Fatal("expected private/public conflict")
	}
}
