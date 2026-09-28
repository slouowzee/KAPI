package gitconfig

type GitConfig struct {
	InitLocal      bool
	HasExistingGit bool

	UniversalGitignore bool
	InitialCommit      bool

	RemoteURL  string
	RemoteHost string
	// HasExistingRemote is set when RemoteURL was detected on an existing
	// repository: nothing must be created, added or pushed for it.
	HasExistingRemote bool

	RemotePrivate bool
	// RemoteHTTPS selects the HTTPS clone URL instead of SSH for a new GitHub repo.
	RemoteHTTPS bool
	RepoName    string
	Collab      bool
	CI          string
}

// CreatesGithubRepo reports whether a new GitHub repository must be created.
// RemoteHost is also inferred from a user-supplied URL, so it alone cannot
// tell "create" from "use this existing remote".
func (c GitConfig) CreatesGithubRepo() bool {
	return c.RemoteHost == "github" && c.RemoteURL == "" && !c.HasExistingRemote
}
