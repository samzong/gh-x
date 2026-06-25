package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	gh "github.com/cli/go-gh/v2"
	"github.com/spf13/cobra"
)

type syncResult struct {
	message string
	failed  bool
}

type repoListOptions struct {
	source  bool
	fork    bool
	private bool
	public  bool
}

var cloneCmd = &cobra.Command{
	Use:   "clone <user1> [org2] ...",
	Short: "Clone or update all repos for one or more users/orgs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClone(cmd.Context(), args)
	},
}

func runClone(ctx context.Context, users []string) error {
	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}

	parallel := parallelFromEnv()
	hadFailures := false

	for _, user := range users {
		targetDir := filepath.Join(baseDir, user)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}

		fmt.Printf("=== %s ===\n", user)

		repos, err := listRepos(user)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			hadFailures = true
			continue
		}
		if len(repos) == 0 {
			continue
		}

		jobs := make(chan string)
		results := make(chan syncResult)

		workerCount := min(parallel, len(repos))

		var wg sync.WaitGroup
		worker := func() {
			defer wg.Done()
			for repo := range jobs {
				results <- syncRepo(ctx, repo, targetDir)
			}
		}

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go worker()
		}

		go func() {
			for _, repo := range repos {
				jobs <- repo
			}
			close(jobs)
		}()

		go func() {
			wg.Wait()
			close(results)
		}()

		for result := range results {
			fmt.Println(result.message)
			if result.failed {
				hadFailures = true
			}
		}
	}

	if hadFailures {
		return errors.New("one or more repositories failed to sync")
	}
	return nil
}

func listRepos(user string) ([]string, error) {
	return listReposWithOptions(user, repoListOptions{})
}

func listReposWithOptions(user string, options repoListOptions) ([]string, error) {
	args, err := repoListArgs(user, options)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("✗ %s (list failed): %s", user, msg)
		}
		return nil, fmt.Errorf("✗ %s (list failed)", user)
	}

	var repos []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			repos = append(repos, line)
		}
	}
	return repos, nil
}

func repoListArgs(user string, options repoListOptions) ([]string, error) {
	if options.source && options.fork {
		return nil, errors.New("--source and --fork cannot be used together")
	}
	if options.private && options.public {
		return nil, errors.New("--private and --public cannot be used together")
	}

	args := []string{"repo", "list", user, "--limit", "1000"}
	if options.source {
		args = append(args, "--source")
	}
	if options.fork {
		args = append(args, "--fork")
	}
	if options.private {
		args = append(args, "--visibility", "private")
	}
	if options.public {
		args = append(args, "--visibility", "public")
	}
	return append(args, "--json", "nameWithOwner", "-q", ".[].nameWithOwner"), nil
}

func syncRepo(ctx context.Context, repo, targetDir string) syncResult {
	repoName := path.Base(repo)
	repoPath := filepath.Join(targetDir, repoName)
	if info, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil && info.IsDir() {
		if err := runGit(ctx, repoPath, "pull", "--ff-only", "--quiet"); err != nil {
			return syncResult{message: fmt.Sprintf("✗ %s (pull failed)", repo), failed: true}
		}
		return syncResult{message: fmt.Sprintf("✓ %s", repo)}
	}

	_ = os.RemoveAll(repoPath)
	if _, _, err := gh.Exec("repo", "clone", repo, repoPath); err != nil {
		return syncResult{message: fmt.Sprintf("✗ %s (clone failed)", repo), failed: true}
	}
	return syncResult{message: fmt.Sprintf("★ %s", repo)}
}

func runGit(ctx context.Context, repoPath string, args ...string) error {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func parallelFromEnv() int {
	return intFromEnv("CLONE_PARALLEL", 4)
}

func intFromEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
