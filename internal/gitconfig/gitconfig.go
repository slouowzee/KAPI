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
