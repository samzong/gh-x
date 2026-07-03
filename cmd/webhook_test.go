package cmd

import (
	"encoding/json"
	"reflect"
	"strings"
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
	rows := repoHookRows("samzong/gh-x", nil)
	want := []string{"samzong/gh-x\t-\t-\t-\t-"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}

	rows = repoHookRows("samzong/adit", []repoWebhook{{
		ID:     1,
		Active: true,
		Events: []string{"push"},
		Config: webhookConfig{URL: "https://example.com"},
	}})
	want = []string{"samzong/adit\t1\ttrue\tpush\thttps://example.com"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestWebhookEventsOrDefault(t *testing.T) {
	got := webhookEventsOrDefault(nil)
	want := []string{"*"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}

	got = webhookEventsOrDefault([]string{"push", "pull_request"})
	want = []string{"push", "pull_request"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestHooksMatchingURL(t *testing.T) {
	hooks := []repoWebhook{
		{ID: 1, Config: webhookConfig{URL: "https://example.com/a"}},
		{ID: 2, Config: webhookConfig{URL: "https://example.com/b"}},
		{ID: 3, Config: webhookConfig{URL: "https://example.com/a"}},
	}

	got := hooksMatchingURL(hooks, "https://example.com/a")
	want := []repoWebhook{hooks[0], hooks[2]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hooks = %v, want %v", got, want)
	}
}

func TestUpdateWebhookRequestForKeepsSecretUnlessRequested(t *testing.T) {
	request := createWebhookRequest{
		Active: true,
		Events: []string{"*"},
		Config: webhookConfig{
			URL:         "https://example.com",
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      "secret",
		},
	}

	payload, err := json.Marshal(updateWebhookRequestFor(request, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "config") {
		t.Fatalf("payload = %s, want no config", payload)
	}

	payload, err = json.Marshal(updateWebhookRequestFor(request, true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"secret":"secret"`) {
		t.Fatalf("payload = %s, want secret", payload)
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
