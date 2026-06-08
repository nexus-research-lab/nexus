package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (s *Service) runCommand(ctx context.Context, workDir string, command ...string) (string, error) {
	return s.runCommandWithEnv(ctx, workDir, nil, command...)
}

func (s *Service) runCommandWithEnv(ctx context.Context, workDir string, extraEnv []string, command ...string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("命令不能为空")
	}
	if s.commandRunner != nil {
		return s.commandRunner(ctx, workDir, extraEnv, command...)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, extraEnv...)
	if strings.TrimSpace(s.config.SkillsAPIURL) != "" {
		cmd.Env = append(cmd.Env, "SKILLS_API_URL="+strings.TrimSpace(s.config.SkillsAPIURL))
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (s *Service) cloneGitRepository(ctx context.Context, repositoryURL string, destination string, options gitCloneOptions) (string, error) {
	attemptOutputs := make([]string, 0, gitCloneMaxAttempts)
	var lastErr error
	for attempt := 1; attempt <= gitCloneMaxAttempts; attempt++ {
		output, runErr := s.runGitCloneAttempt(ctx, repositoryURL, destination, options)
		if runErr == nil {
			return output, nil
		}
		lastErr = runErr
		attemptOutputs = append(attemptOutputs, formatGitAttemptOutput(attempt, output, runErr))
		if ctx.Err() != nil || attempt == gitCloneMaxAttempts || !isTransientGitCloneError(output, runErr) {
			break
		}
		_ = os.RemoveAll(destination)
		delay := time.NewTimer(time.Duration(attempt) * 300 * time.Millisecond)
		select {
		case <-ctx.Done():
			delay.Stop()
			return strings.Join(attemptOutputs, "\n"), ctx.Err()
		case <-delay.C:
		}
	}
	return strings.Join(attemptOutputs, "\n"), lastErr
}

func (s *Service) runGitCloneAttempt(ctx context.Context, repositoryURL string, destination string, options gitCloneOptions) (string, error) {
	branch := strings.TrimSpace(options.Branch)
	branchWasExplicit := branch != ""
	if branch == "" {
		branch = s.resolveGitDefaultBranch(ctx, repositoryURL, options)
	}

	output, runErr := s.runGitCloneCommand(ctx, repositoryURL, destination, branch, options)
	if runErr == nil || branchWasExplicit || branch != "" {
		return output, runErr
	}

	_ = os.RemoveAll(destination)
	fallbackOutput, fallbackErr := s.runGitCloneCommand(ctx, repositoryURL, destination, "master", options)
	if strings.TrimSpace(output) == "" {
		return fallbackOutput, fallbackErr
	}
	if strings.TrimSpace(fallbackOutput) == "" {
		return output, fallbackErr
	}
	return strings.TrimSpace(output + "\n" + fallbackOutput), fallbackErr
}

func (s *Service) runGitCloneCommand(ctx context.Context, repositoryURL string, destination string, branch string, options gitCloneOptions) (string, error) {
	command := []string{"git", "-c", "http.version=HTTP/1.1", "clone", "--depth", "1", "--single-branch"}
	if strings.TrimSpace(branch) != "" {
		command = append(command, "--branch", strings.TrimSpace(branch))
	}
	command = append(command, "--", repositoryURL, destination)
	return s.runCommandWithEnv(ctx, "", gitCommandEnv(options), command...)
}

func (s *Service) resolveGitDefaultBranch(ctx context.Context, repositoryURL string, options gitCloneOptions) string {
	output, err := s.runCommandWithEnv(ctx, "", gitCommandEnv(options), "git", "ls-remote", "--symref", "--", repositoryURL, "HEAD")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ref: refs/heads/") {
			continue
		}
		branch := strings.TrimPrefix(line, "ref: refs/heads/")
		if before, _, ok := strings.Cut(branch, "\t"); ok {
			branch = before
		}
		if before, _, ok := strings.Cut(branch, " "); ok {
			branch = before
		}
		return strings.TrimSpace(branch)
	}
	return ""
}

func gitCommandEnv(options gitCloneOptions) []string {
	env := []string{"GIT_TERMINAL_PROMPT=0"}
	if options.CleanGlobalConfig {
		env = append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	}
	return env
}

func shouldUseCleanGitConfigForRepository(repositoryURL string, manifest externalManifest) bool {
	normalizedURL := strings.ToLower(strings.TrimSpace(repositoryURL))
	if !strings.HasPrefix(normalizedURL, "https://github.com/") && !strings.HasPrefix(normalizedURL, "https://www.github.com/") {
		return false
	}
	switch strings.TrimSpace(manifest.SourceKind) {
	case externalSourceKindClaudePlugins,
		externalSourceKindSkillsSh,
		externalSourceKindHermesIndex,
		externalSourceKindBrowseSh,
		externalSourceKindWellKnown:
		return true
	default:
		return false
	}
}

func formatGitAttemptOutput(attempt int, output string, err error) string {
	message := strings.TrimSpace(output)
	if message == "" && err != nil {
		message = err.Error()
	}
	return fmt.Sprintf("attempt %d/%d: %s", attempt, gitCloneMaxAttempts, message)
}

func isTransientGitCloneError(output string, err error) bool {
	text := strings.ToLower(output + " " + fmt.Sprint(err))
	transientMarkers := []string{
		"early eof",
		"eof",
		"could not fetch",
		"rpc failed",
		"connection closed",
		"remote end hung up unexpectedly",
		"http/2 stream",
		"http 5",
		"connection reset",
		"connection refused",
		"connection timed out",
		"operation timed out",
		"tls handshake timeout",
		"ssl_error_syscall",
		"ssl_connect",
		"gnutls",
		"gnutls_handshake",
		"the tls connection was non-properly terminated",
		"temporary failure",
		"temporarily unavailable",
	}
	for _, marker := range transientMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
