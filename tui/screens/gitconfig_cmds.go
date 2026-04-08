package screens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
)

type gitcfgDetectionMsg struct {
	hasGit    bool
	remoteURL string
}

type gitcfgScopesMsg struct {
	scopes config.TokenScopes
	err    error
}

type gitcfgCollabDetectionMsg struct {
	state collabState
}

type gitcfgSigningDetectionMsg struct {
	localActive        bool
	globalActive       bool
	gpgKeys            []string
	sshKeys            []string
	gpgAvailable       bool
	sshKeygenAvailable bool
}

type gitcfgKeyGenMsg struct{ err error }

type gitcfgKeyDeleteMsg struct {
	err error
}

type gitcfgExecMsg struct {
	err          error
	newRemoteURL string
	successMsg   string
}

type gitcfgSigningDoneMsg struct {
	err        error
	successMsg string
}

type gitcfgGithubPushDoneMsg struct {
	err        error
	successMsg string
}

func detectGitConfigCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		checkCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		checkCmd.Dir = dir
		checkOut, checkErr := checkCmd.Output()
		if checkErr != nil || strings.TrimSpace(string(checkOut)) != "true" {
			return gitcfgDetectionMsg{hasGit: false}
		}

		var remoteURL string
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err == nil {
			remoteURL = strings.TrimSpace(string(out))
		}
		return gitcfgDetectionMsg{hasGit: true, remoteURL: remoteURL}
	}
}

func fetchTokenScopesCmd() tea.Cmd {
	return func() tea.Msg {
		scopes, err := config.FetchTokenScopes(context.Background())
		return gitcfgScopesMsg{scopes: scopes, err: err}
	}
}

func detectCollabStateCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		var s collabState

		cmd := exec.Command("git", "branch", "--list", "dev")
		cmd.Dir = dir
		out, err := cmd.Output()
		s.hasBranchDev = err == nil && strings.TrimSpace(string(out)) != ""

		_, err = os.Stat(filepath.Join(dir, "CONTRIBUTING.md"))
		s.hasContributing = err == nil

		_, err = os.Stat(filepath.Join(dir, ".github", "PULL_REQUEST_TEMPLATE.md"))
		s.hasPRTemplate = err == nil

		entries, err := os.ReadDir(filepath.Join(dir, ".github", "ISSUE_TEMPLATE"))
		s.hasIssueTemplates = err == nil && len(entries) > 0

		return gitcfgCollabDetectionMsg{state: s}
	}
}

func detectGitSigningCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		var msg gitcfgSigningDetectionMsg

		localCmd := exec.Command("git", "config", "--local", "commit.gpgsign")
		localCmd.Dir = dir
		if out, err := localCmd.Output(); err == nil {
			msg.localActive = strings.TrimSpace(string(out)) == "true"
		}

		globalCmd := exec.Command("git", "config", "--global", "commit.gpgsign")
		if out, err := globalCmd.Output(); err == nil {
			msg.globalActive = strings.TrimSpace(string(out)) == "true"
		}

		if _, err := exec.LookPath("gpg"); err == nil {
			msg.gpgAvailable = true
			gpgCmd := exec.Command("gpg", "--list-secret-keys", "--with-colons")
			if out, err := gpgCmd.Output(); err == nil {
				isPrimary := false
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "sec:") {
						isPrimary = true
					} else if strings.HasPrefix(line, "ssb:") {
						isPrimary = false
					} else if strings.HasPrefix(line, "fpr:") && isPrimary {
						parts := strings.Split(line, ":")
						if len(parts) >= 10 && parts[9] != "" {
							msg.gpgKeys = append(msg.gpgKeys, parts[9])
							isPrimary = false
						}
					}
				}
			}
		}

		if _, err := exec.LookPath("ssh-keygen"); err == nil {
			msg.sshKeygenAvailable = true
		}

		sshDir, _ := os.UserHomeDir()
		sshDir = filepath.Join(sshDir, ".ssh")
		entries, err := os.ReadDir(sshDir)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if strings.HasSuffix(name, ".pub") && name != "known_hosts.pub" {
					msg.sshKeys = append(msg.sshKeys, filepath.Join(sshDir, name))
				}
			}
		}

		return msg
	}
}


func runKeyGenCmd(format string) tea.Cmd {
	var binary string
	var args []string
	if format == "ssh" {
		binary = "ssh-keygen"
		args = []string{"-t", "ed25519"}
	} else {
		binary = "gpg"
		args = []string{"--gen-key"}
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return func() tea.Msg {
			return gitcfgKeyGenMsg{err: fmt.Errorf("%s not found in PATH — please install it first", binary)}
		}
	}
	return tea.ExecProcess(exec.Command(path, args...), func(err error) tea.Msg {
		return gitcfgKeyGenMsg{err: err}
	})
}

func zeroAndRemove(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	zeros := make([]byte, info.Size())
	_, _ = f.Write(zeros)
	_ = f.Sync()
	f.Close()
	return os.Remove(path)
}

func execGitKeyDeleteCmd(format, key string) tea.Cmd {
	return func() tea.Msg {
		switch format {
		case "gpg":
			cmd1 := exec.Command("gpg", "--batch", "--yes", "--delete-secret-keys", key)
			if err := cmd1.Run(); err != nil {
				return gitcfgKeyDeleteMsg{err: fmt.Errorf("failed to delete GPG secret key: %w", err)}
			}
			cmd2 := exec.Command("gpg", "--batch", "--yes", "--delete-keys", key)
			_ = cmd2.Run()
		case "ssh":
			pubPath := key
			privPath := strings.TrimSuffix(key, ".pub")
			if err := zeroAndRemove(pubPath); err != nil {
				return gitcfgKeyDeleteMsg{err: fmt.Errorf("failed to delete SSH public key: %w", err)}
			}
			if err := zeroAndRemove(privPath); err != nil {
				return gitcfgKeyDeleteMsg{err: fmt.Errorf("failed to delete SSH private key: %w", err)}
			}
		}
		return gitcfgKeyDeleteMsg{}
	}
}

func execGithubCreateRepoCmd(name string, private bool, dir string) tea.Cmd {
	return func() tea.Msg {
		tok := config.GithubToken()
		if tok == "" {
			return gitcfgExecMsg{err: fmt.Errorf("GitHub token not found")}
		}

		bodyData := map[string]any{
			"name":    name,
			"private": private,
		}
		reqBody, _ := json.Marshal(bodyData)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.github.com/user/repos", bytes.NewReader(reqBody))
		if err != nil {
			return gitcfgExecMsg{err: fmt.Errorf("could not create GitHub API request: %w", err)}
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		httpClient := &http.Client{Timeout: 15 * time.Second}
		resp, err := httpClient.Do(req)
		if err != nil {
			return gitcfgExecMsg{err: fmt.Errorf("could not create repo on GitHub: %w", err)}
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusUnprocessableEntity {
			return gitcfgExecMsg{err: fmt.Errorf("GitHub repo '%s' already exists", name)}
		}
		if resp.StatusCode != http.StatusCreated {
			return gitcfgExecMsg{err: fmt.Errorf("failed to create repo (HTTP %d): %s", resp.StatusCode, string(bodyBytes))}
		}

		var result struct {
			SSHUrl string `json:"ssh_url"`
		}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return gitcfgExecMsg{err: fmt.Errorf("failed to parse GitHub response")}
		}

		sshUrl := result.SSHUrl
		if sshUrl == "" {
			return gitcfgExecMsg{err: fmt.Errorf("no SSH URL in GitHub response")}
		}

		chk := exec.Command("git", "remote", "get-url", "origin")
		chk.Dir = dir
		if err := chk.Run(); err == nil {
			cmd := exec.Command("git", "remote", "set-url", "origin", sshUrl)
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("repo created but failed to set remote: %w", err)}
			}
			return gitcfgExecMsg{newRemoteURL: sshUrl, successMsg: "GitHub repo created and remote updated successfully."}
		}
		cmd := exec.Command("git", "remote", "add", "origin", sshUrl)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return gitcfgExecMsg{err: fmt.Errorf("repo created but failed to add remote: %w", err)}
		}
		return gitcfgExecMsg{newRemoteURL: sshUrl, successMsg: "GitHub repo created and remote added successfully."}
	}
}

func execGitRemoteSetCmd(dir, url string) tea.Cmd {
	return func() tea.Msg {
		chk := exec.Command("git", "remote", "get-url", "origin")
		chk.Dir = dir
		if err := chk.Run(); err == nil {
			cmd := exec.Command("git", "remote", "set-url", "origin", url)
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				return gitcfgExecMsg{err: err}
			}
			return gitcfgExecMsg{newRemoteURL: url, successMsg: "Remote updated successfully."}
		}
		cmd := exec.Command("git", "remote", "add", "origin", url)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return gitcfgExecMsg{err: err}
		}
		return gitcfgExecMsg{newRemoteURL: url, successMsg: "Remote added successfully."}
	}
}

func execGitCollabCmd(dir string, plan collabPlan) tea.Cmd {
	return func() tea.Msg {
		var done []string

		if plan.createBranch {
			branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			branchCmd.Dir = dir
			out, err := branchCmd.Output()
			if err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not determine current branch: %w", err)}
			}
			currentBranch := strings.TrimSpace(string(out))

			createCmd := exec.Command("git", "checkout", "-b", "dev")
			createCmd.Dir = dir
			if err := createCmd.Run(); err != nil {
				checkoutCmd := exec.Command("git", "checkout", "dev")
				checkoutCmd.Dir = dir
				if err2 := checkoutCmd.Run(); err2 != nil {
					return gitcfgExecMsg{err: fmt.Errorf("could not create or switch to dev branch: %w", err2)}
				}
			}

			if currentBranch != "HEAD" && currentBranch != "dev" {
				backCmd := exec.Command("git", "checkout", currentBranch)
				backCmd.Dir = dir
				if err := backCmd.Run(); err != nil {
					return gitcfgExecMsg{err: fmt.Errorf("could not restore branch %q: %w", currentBranch, err)}
				}
			}
			done = append(done, "dev branch")
		}

		if plan.writeContributing {
			content := contributingContent()
			if err := os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte(content), 0644); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not write CONTRIBUTING.md: %w", err)}
			}
			done = append(done, "CONTRIBUTING.md")
		}

		ghDir := filepath.Join(dir, ".github")

		if plan.writePRTemplate {
			if err := os.MkdirAll(ghDir, 0755); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not create .github directory: %w", err)}
			}
			if err := os.WriteFile(filepath.Join(ghDir, "PULL_REQUEST_TEMPLATE.md"), []byte(prTemplateContent()), 0644); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not write PR template: %w", err)}
			}
			done = append(done, "PR template")
		}

		if plan.writeIssueTemplates {
			issueDir := filepath.Join(ghDir, "ISSUE_TEMPLATE")
			if err := os.MkdirAll(issueDir, 0755); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not create ISSUE_TEMPLATE directory: %w", err)}
			}
			if err := os.WriteFile(filepath.Join(issueDir, "bug_report.md"), []byte(bugReportContent()), 0644); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not write bug report template: %w", err)}
			}
			if err := os.WriteFile(filepath.Join(issueDir, "feature_request.md"), []byte(featureRequestContent()), 0644); err != nil {
				return gitcfgExecMsg{err: fmt.Errorf("could not write feature request template: %w", err)}
			}
			done = append(done, "Issue templates")
		}

		if len(done) == 0 {
			return gitcfgExecMsg{successMsg: "Nothing to do — everything was already in place."}
		}
		return gitcfgExecMsg{successMsg: strings.Join(done, ", ") + " set up successfully."}
	}
}

func execGithubPushKeyCmd(format, key, title string) tea.Cmd {
	return func() tea.Msg {
		tok := config.GithubToken()
		if tok == "" {
			return gitcfgGithubPushDoneMsg{successMsg: "Key not pushed to GitHub (no token)"}
		}

		var endpoint string
		var reqBody []byte

		if format == "ssh" {
			endpoint = "https://api.github.com/user/keys"
			content, err := os.ReadFile(key)
			if err != nil {
				return gitcfgGithubPushDoneMsg{err: fmt.Errorf("could not read SSH public key: %w", err)}
			}
			bodyData := map[string]string{
				"title": title,
				"key":   strings.TrimSpace(string(content)),
			}
			reqBody, _ = json.Marshal(bodyData)
		} else {
			endpoint = "https://api.github.com/user/gpg_keys"
			cmd := exec.Command("gpg", "--armor", "--export", key)
			content, err := cmd.Output()
			if err != nil {
				return gitcfgGithubPushDoneMsg{err: fmt.Errorf("could not export GPG key: %w", err)}
			}
			bodyData := map[string]string{
				"name":               title,
				"armored_public_key": strings.TrimSpace(string(content)),
			}
			reqBody, _ = json.Marshal(bodyData)
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(reqBody))
		if err != nil {
			return gitcfgGithubPushDoneMsg{err: fmt.Errorf("could not create GitHub API request: %w", err)}
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return gitcfgGithubPushDoneMsg{err: fmt.Errorf("could not push key to GitHub: %w", err)}
		}
		defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusUnprocessableEntity {
			return gitcfgGithubPushDoneMsg{err: fmt.Errorf("failed to push key (HTTP %d)", resp.StatusCode)}
		}

		successMsg := fmt.Sprintf("Key pushed to GitHub (%s).", strings.ToUpper(format))
		if resp.StatusCode == http.StatusUnprocessableEntity {
			successMsg = fmt.Sprintf("Key already exists on GitHub (%s).", strings.ToUpper(format))
		}
		return gitcfgGithubPushDoneMsg{successMsg: successMsg}
	}
}

func execGitSigningCmd(dir, format, scope, key string) tea.Cmd {
	return func() tea.Msg {
		scopes := []string{}
		switch scope {
		case "local":
			scopes = []string{"--local"}
		case "global":
			scopes = []string{"--global"}
		case "both":
			scopes = []string{"--local", "--global"}
		}

		for _, s := range scopes {
			cmd := exec.Command("git", "config", s, "commit.gpgsign", "true")
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				return gitcfgSigningDoneMsg{err: fmt.Errorf("could not enable commit.gpgsign (%s): %w", s, err)}
			}
			cmd2 := exec.Command("git", "config", s, "user.signingkey", key)
			cmd2.Dir = dir
			if err := cmd2.Run(); err != nil {
				return gitcfgSigningDoneMsg{err: fmt.Errorf("could not set user.signingkey (%s): %w", s, err)}
			}
			if format == "ssh" {
				cmd3 := exec.Command("git", "config", s, "gpg.format", "ssh")
				cmd3.Dir = dir
				if err := cmd3.Run(); err != nil {
					return gitcfgSigningDoneMsg{err: fmt.Errorf("could not set gpg.format (%s): %w", s, err)}
				}
			}
		}

		scopeLabel := scope
		return gitcfgSigningDoneMsg{successMsg: fmt.Sprintf("Commit signing enabled (%s, %s).", strings.ToUpper(format), scopeLabel)}
	}
}
