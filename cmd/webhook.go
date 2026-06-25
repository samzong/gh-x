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

type webhookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	InsecureSSL string `json:"insecure_ssl"`
	Secret      string `json:"secret,omitempty"`
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
	Short: "List or add repository webhooks in batches",
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
	Short: "Add a webhook to repositories",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWebhookAdd(cmd.Context(), args[0], args[1:])
	},
}

func init() {
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.source, "source", false, "Show only non-fork repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.fork, "fork", false, "Show only fork repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.private, "private", false, "Show only private repositories for owner/org targets")
	webhookCmd.PersistentFlags().BoolVar(&webhookRepos.public, "public", false, "Show only public repositories for owner/org targets")
	webhookAddCmd.Flags().StringArrayVarP(&webhookEvents, "event", "e", nil, "Webhook event to subscribe to (default push)")
	webhookAddCmd.Flags().StringVar(&webhookSecretEnv, "secret-env", "", "Environment variable containing the webhook secret")
	webhookCmd.AddCommand(webhookListCmd, webhookAddCmd)
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
	events := webhookEvents
	if len(events) == 0 {
		events = []string{"push"}
	}

	secret := ""
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
		if err := createRepoWebhook(ctx, repo, request); err != nil {
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

func expandRepoTargets(targets []string) ([]string, error) {
	return expandRepoTargetsWithOptions(targets, repoListOptions{})
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
	stdout, stderr, err := gh.Exec("api", fmt.Sprintf("repos/%s/hooks", repo), "--jq", `.[] | "\(.id)\t\(.active)\t\(.events | join(","))\t\(.config.url)"`)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, err
	}
	return repoHookRows(repo, stdout.String()), nil
}

func repoHookRows(repo, output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return []string{fmt.Sprintf("%s\t-\t-\t-\t-", repo)}
	}

	var rows []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			rows = append(rows, fmt.Sprintf("%s\t%s", repo, line))
		}
	}
	return rows
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

func ghAPIInput(ctx context.Context, endpoint string, body any) (string, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "gh", "api", "-X", "POST", endpoint, "--input", "-")
	command.Stdin = bytes.NewReader(payload)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	return stdout.String(), stderr.String(), err
}
