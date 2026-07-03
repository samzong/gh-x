package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/tabwriter"

	gh "github.com/cli/go-gh/v2"
	"github.com/spf13/cobra"
)

type createWebhookRequest struct {
	Name   string        `json:"name"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config webhookConfig `json:"config"`
}

type updateWebhookRequest struct {
	Active bool           `json:"active"`
	Events []string       `json:"events"`
	Config *webhookConfig `json:"config,omitempty"`
}

type webhookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	InsecureSSL string `json:"insecure_ssl"`
	Secret      string `json:"secret,omitempty"`
}

type repoWebhook struct {
	ID     int64         `json:"id"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config webhookConfig `json:"config"`
}

var (
	webhookEvents    []string
	webhookRepos     repoListOptions
	webhookSecretEnv string
)

type repoHookListResult struct {
	repo string
	rows []string
	err  error
}

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "List, add, or delete repository webhooks in batches",
}

var webhookListCmd = &cobra.Command{
	Use:   "list <owner/repo|owner>...",
	Short: "List webhooks for repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWebhookList(args)
	},
}

var webhookAddCmd = &cobra.Command{
	Use:   "add <url> <owner/repo|owner>...",
	Short: "Add or update a webhook on repositories",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWebhookAdd(cmd.Context(), args[0], args[1:])
	},
}

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <url> <owner/repo|owner>...",
	Short: "Delete webhooks matching a URL from repositories",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWebhookDelete(cmd.Context(), args[0], args[1:])
	},
}

func init() {
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.source, "source", false, "Show only non-fork repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.fork, "fork", false, "Show only fork repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.private, "private", false, "Show only private repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.public, "public", false, "Show only public repositories for owner/org targets")
	webhookAddCmd.Flags().StringArrayVarP(&webhookEvents, "event", "e", nil, "Webhook event to subscribe to (default all events)")
	webhookAddCmd.Flags().StringVar(&webhookSecretEnv, "secret-env", "", "Environment variable containing the webhook secret")
	webhookCmd.AddCommand(webhookListCmd, webhookAddCmd, webhookDeleteCmd)
	rootCmd.AddCommand(webhookCmd)
}

func runWebhookList(targets []string) error {
	repos, err := expandRepoTargetsWithOptions(targets, webhookRepos)
	if err != nil {
		return err
	}

	hadFailures := false
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "repo\tid\tactive\tevents\turl")
	for _, result := range listRepoHooks(repos) {
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s (list failed): %s\n", result.repo, result.err)
			hadFailures = true
			continue
		}
		for _, row := range result.rows {
			fmt.Fprintln(writer, row)
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if hadFailures {
		return errors.New("one or more repositories failed")
	}
	return nil
}

func runWebhookAdd(ctx context.Context, hookURL string, targets []string) error {
	events := webhookEventsOrDefault(webhookEvents)

	secret := ""
	updateConfig := webhookSecretEnv != ""
	if webhookSecretEnv != "" {
		secret = strings.TrimSpace(os.Getenv(webhookSecretEnv))
		if secret == "" {
			return fmt.Errorf("%s is empty", webhookSecretEnv)
		}
	}

	repos, err := expandRepoTargetsWithOptions(targets, webhookRepos)
	if err != nil {
		return err
	}

	request := createWebhookRequest{
		Name:   "web",
		Active: true,
		Events: events,
		Config: webhookConfig{
			URL:         hookURL,
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      secret,
		},
	}

	hadFailures := false
	for _, repo := range repos {
		if err := upsertRepoWebhook(ctx, repo, request, updateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s (add failed): %s\n", repo, err)
			hadFailures = true
			continue
		}
		fmt.Printf("✓ %s\n", repo)
	}

	if hadFailures {
		return errors.New("one or more repositories failed")
	}
	return nil
}

func runWebhookDelete(ctx context.Context, hookURL string, targets []string) error {
	repos, err := expandRepoTargetsWithOptions(targets, webhookRepos)
	if err != nil {
		return err
	}

	hadFailures := false
	for _, repo := range repos {
		deleted, err := deleteRepoWebhooksByURL(ctx, repo, hookURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s (delete failed): %s\n", repo, err)
			hadFailures = true
			continue
		}
		if deleted == 0 {
			fmt.Printf("- %s (not found)\n", repo)
			continue
		}
		fmt.Printf("✓ %s (%d)\n", repo, deleted)
	}

	if hadFailures {
		return errors.New("one or more repositories failed")
	}
	return nil
}

func expandRepoTargets(targets []string) ([]string, error) {
	return expandRepoTargetsWithOptions(targets, repoListOptions{})
}

func webhookEventsOrDefault(events []string) []string {
	if len(events) == 0 {
		return []string{"*"}
	}
	return events
}

func expandRepoTargetsWithOptions(targets []string, options repoListOptions) ([]string, error) {
	if _, err := repoListArgs("", options); err != nil {
		return nil, err
	}

	var repos []string
	seen := make(map[string]bool)
	add := func(repo string) {
		if seen[repo] {
			return
		}
		seen[repo] = true
		repos = append(repos, repo)
	}

	for _, target := range targets {
		if strings.Contains(target, "/") {
			add(target)
			continue
		}

		ownerRepos, err := listReposWithOptions(target, options)
		if err != nil {
			return nil, err
		}
		for _, repo := range ownerRepos {
			add(repo)
		}
	}

	return repos, nil
}

func listRepoHooks(repos []string) []repoHookListResult {
	results := make([]repoHookListResult, len(repos))
	jobs := make(chan int)
	workerCount := min(intFromEnv("WEBHOOK_PARALLEL", 8), len(repos))

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for i := range jobs {
			repo := repos[i]
			rows, err := repoHookRowsForRepo(repo)
			results[i] = repoHookListResult{repo: repo, rows: rows, err: err}
		}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range repos {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func repoHookRowsForRepo(repo string) ([]string, error) {
	hooks, err := repoHooksForRepo(repo)
	if err != nil {
		return nil, err
	}
	return repoHookRows(repo, hooks), nil
}

func repoHookRows(repo string, hooks []repoWebhook) []string {
	if len(hooks) == 0 {
		return []string{fmt.Sprintf("%s\t-\t-\t-\t-", repo)}
	}

	var rows []string
	for _, hook := range hooks {
		rows = append(rows, fmt.Sprintf("%s\t%d\t%t\t%s\t%s", repo, hook.ID, hook.Active, strings.Join(hook.Events, ","), hook.Config.URL))
	}
	return rows
}

func repoHooksForRepo(repo string) ([]repoWebhook, error) {
	stdout, stderr, err := gh.Exec("api", fmt.Sprintf("repos/%s/hooks", repo))
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, err
	}

	var hooks []repoWebhook
	if err := json.Unmarshal([]byte(stdout.String()), &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

func hooksMatchingURL(hooks []repoWebhook, hookURL string) []repoWebhook {
	var matches []repoWebhook
	for _, hook := range hooks {
		if hook.Config.URL == hookURL {
			matches = append(matches, hook)
		}
	}
	return matches
}

func upsertRepoWebhook(ctx context.Context, repo string, request createWebhookRequest, updateConfig bool) error {
	hooks, err := repoHooksForRepo(repo)
	if err != nil {
		return err
	}

	matches := hooksMatchingURL(hooks, request.Config.URL)
	switch len(matches) {
	case 0:
		return createRepoWebhook(ctx, repo, request)
	case 1:
		return updateRepoWebhook(ctx, repo, matches[0].ID, updateWebhookRequestFor(request, updateConfig))
	default:
		return fmt.Errorf("%d webhooks match %s", len(matches), request.Config.URL)
	}
}

func updateWebhookRequestFor(request createWebhookRequest, includeConfig bool) updateWebhookRequest {
	update := updateWebhookRequest{
		Active: request.Active,
		Events: request.Events,
	}
	if includeConfig {
		update.Config = &request.Config
	}
	return update
}

func createRepoWebhook(ctx context.Context, repo string, request createWebhookRequest) error {
	_, stderr, err := ghAPIInput(ctx, fmt.Sprintf("repos/%s/hooks", repo), request)
	if err != nil {
		if msg := strings.TrimSpace(stderr); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

func updateRepoWebhook(ctx context.Context, repo string, hookID int64, request updateWebhookRequest) error {
	_, stderr, err := ghAPIInputMethod(ctx, "PATCH", fmt.Sprintf("repos/%s/hooks/%d", repo, hookID), request)
	if err != nil {
		if msg := strings.TrimSpace(stderr); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

func deleteRepoWebhooksByURL(ctx context.Context, repo, hookURL string) (int, error) {
	hooks, err := repoHooksForRepo(repo)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, hook := range hooksMatchingURL(hooks, hookURL) {
		if err := deleteRepoWebhook(ctx, repo, hook.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func deleteRepoWebhook(ctx context.Context, repo string, hookID int64) error {
	_, stderr, err := ghAPIMethod(ctx, "DELETE", fmt.Sprintf("repos/%s/hooks/%d", repo, hookID))
	if err != nil {
		if msg := strings.TrimSpace(stderr); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

func ghAPIInput(ctx context.Context, endpoint string, body any) (string, string, error) {
	return ghAPIInputMethod(ctx, "POST", endpoint, body)
}

func ghAPIInputMethod(ctx context.Context, method, endpoint string, body any) (string, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "gh", "api", "-X", method, endpoint, "--input", "-")
	command.Stdin = bytes.NewReader(payload)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	return stdout.String(), stderr.String(), err
}

func ghAPIMethod(ctx context.Context, method, endpoint string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "gh", "api", "-X", method, endpoint)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}
