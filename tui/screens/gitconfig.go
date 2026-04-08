package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
)

type GitConfigModel struct {
	width  int
	height int
	dir    string

	detecting bool
	step      gitcfgStep

	remoteURL string

	menuItems  []string
	menuCursor int

	inputURL    string
	inputURLPos int

	remoteMenuCursor int
	remoteRepoName   string
	remoteNamePos    int
	remoteIsPrivate  bool

	collabDetecting bool
	collabState     collabState
	collabQuestions []collabQuestion
	collabQIndex    int
	collabCursor    int

	ciCursor int
	ciChoice string

	signingDetecting      bool
	signingLocalActive    bool
	signingGlobalActive   bool
	signingGPGKeys        []string
	signingSSHKeys        []string
	signingGPGAvailable   bool
	signingSSHKeygenAvail bool
	signingFormat         string
	signingScope          string
	signingKey            string
	signingFormatCursor   int
	signingScopeCursor    int
	signingKeyCursor      int
	signingReturnToKey    bool

	manageFormat       string
	manageFormatCursor int
	manageListCursor   int
	manageKeyToDelete  string
	manageDeleteCursor int
	manageIsActiveFlow bool

	lastMsg string
	lastErr error
	execMsg string

	scopes        config.TokenScopes
	scopesFetched bool

	backPressed bool
}

func NewGitConfig(width, height int, dir string) GitConfigModel {
	return GitConfigModel{
		width:     width,
		height:    height,
		dir:       dir,
		step:      GITCFG_STEP_DETECTING,
		detecting: true,
	}
}

func (m *GitConfigModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m GitConfigModel) IsBack() bool { return m.backPressed }
func (m GitConfigModel) IsInputMode() bool {
	return m.step == GITCFG_STEP_REMOTE_INPUT || m.step == GITCFG_STEP_REMOTE_NAME_INPUT
}
func (m GitConfigModel) CIChoice() string { return m.ciChoice }

func (m *GitConfigModel) ConsumeBack() { m.backPressed = false }

func (m GitConfigModel) Init() tea.Cmd {
	return tea.Batch(detectGitConfigCmd(m.dir), fetchTokenScopesCmd())
}

func (m GitConfigModel) Update(msg tea.Msg) (GitConfigModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case gitcfgExecMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
		} else {
			m.lastErr = nil
			m.lastMsg = msg.successMsg
			if msg.newRemoteURL != "" {
				m.remoteURL = msg.newRemoteURL
				m.menuItems = buildGitCfgMenu(m.remoteURL)
			}
		}
		m.step = GITCFG_STEP_MENU

	case gitcfgSigningDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
			m.step = GITCFG_STEP_MENU
		} else {
			m.lastErr = nil
			m.lastMsg = msg.successMsg
			if m.step == GITCFG_STEP_SIGNING_PUSH_GITHUB {
				m.execMsg = "Pushing key to GitHub..."
				return m, execGithubPushKeyCmd(m.signingFormat, m.signingKey, "KAPI Signing Key")
			}
			m.step = GITCFG_STEP_MENU
		}

	case gitcfgGithubPushDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
		} else {
			m.lastErr = nil
			if m.lastMsg != "" {
				m.lastMsg = m.lastMsg + " " + msg.successMsg
			} else {
				m.lastMsg = msg.successMsg
			}
		}
		if m.manageIsActiveFlow {
			m.step = GITCFG_STEP_MANAGE_LIST
		} else {
			m.step = GITCFG_STEP_MENU
		}

	case gitcfgDetectionMsg:
		m.detecting = false
		if !msg.hasGit {
			m.step = GITCFG_STEP_NO_GIT
			return m, nil
		}
		m.remoteURL = msg.remoteURL
		m.menuItems = buildGitCfgMenu(m.remoteURL)
		m.step = GITCFG_STEP_MENU

	case gitcfgScopesMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
		}
		m.scopes = msg.scopes
		m.scopesFetched = true
		m.menuItems = buildGitCfgMenu(m.remoteURL)

	case gitcfgCollabDetectionMsg:
		m.collabDetecting = false
		m.collabState = msg.state
		m.collabQuestions = buildCollabQuestions(msg.state)
		m.step = GITCFG_STEP_COLLAB_CHECKLIST

	case gitcfgSigningDetectionMsg:
		m.signingDetecting = false
		m.signingLocalActive = msg.localActive
		m.signingGlobalActive = msg.globalActive
		m.signingGPGKeys = msg.gpgKeys
		m.signingSSHKeys = msg.sshKeys
		m.signingGPGAvailable = msg.gpgAvailable
		m.signingSSHKeygenAvail = msg.sshKeygenAvailable
		if m.manageIsActiveFlow {
			if m.signingReturnToKey {
				m.signingReturnToKey = false
				m.manageListCursor = len(m.signingKeys()) - 1
				m.step = GITCFG_STEP_MANAGE_LIST
			} else {
				m.step = GITCFG_STEP_MANAGE_FORMAT
			}
		} else {
			if m.signingReturnToKey {
				m.signingReturnToKey = false
				m.signingKeyCursor = 0
				m.step = GITCFG_STEP_SIGNING_KEY
			} else {
				m.step = GITCFG_STEP_SIGNING_STATUS
			}
		}

	case gitcfgKeyGenMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
			if m.manageIsActiveFlow {
				m.step = GITCFG_STEP_MANAGE_LIST
			} else {
				m.step = GITCFG_STEP_SIGNING_KEY
			}
			return m, nil
		}
		m.signingDetecting = true
		m.signingReturnToKey = true
		if m.manageIsActiveFlow {
			m.step = GITCFG_STEP_MANAGE_DETECTING
		} else {
			m.step = GITCFG_STEP_SIGNING_DETECTING
		}
		return m, detectGitSigningCmd(m.dir)

	case gitcfgKeyDeleteMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = msg.err.Error()
			m.step = GITCFG_STEP_MANAGE_LIST
			return m, nil
		}
		m.lastMsg = "Key deleted successfully."
		m.lastErr = nil
		m.signingDetecting = true
		m.signingReturnToKey = true
		m.step = GITCFG_STEP_MANAGE_DETECTING
		return m, detectGitSigningCmd(m.dir)

	case tea.KeyMsg:
		if m.detecting || m.collabDetecting || m.signingDetecting {
			break
		}
		switch m.step {
		case GITCFG_STEP_REMOTE_MENU:
			return m.handleRemoteMenu(msg)
		case GITCFG_STEP_REMOTE_NAME_INPUT:
			return m.handleRemoteNameInput(msg)
		case GITCFG_STEP_REMOTE_INPUT:
			return m.handleURLInput(msg)
		case GITCFG_STEP_CONFIRM_CI:
			return m.handleCIMenu(msg)
		case GITCFG_STEP_COLLAB_CHECKLIST:
			return m.handleCollabChecklist(msg)
		case GITCFG_STEP_COLLAB_QUESTIONS:
			return m.handleCollabQuestion(msg)
		case GITCFG_STEP_SIGNING_STATUS:
			return m.handleSigningStatus(msg)
		case GITCFG_STEP_SIGNING_FORMAT:
			return m.handleSigningFormat(msg)
		case GITCFG_STEP_SIGNING_SCOPE:
			return m.handleSigningScope(msg)
		case GITCFG_STEP_SIGNING_KEY:
			return m.handleSigningKey(msg)
		case GITCFG_STEP_MANAGE_FORMAT:
			return m.handleManageFormat(msg)
		case GITCFG_STEP_MANAGE_LIST:
			return m.handleManageList(msg)
		case GITCFG_STEP_MANAGE_CONFIRM_DELETE:
			return m.handleManageConfirmDelete(msg)
		default:
			switch msg.String() {
			case "esc":
				switch m.step {
				case GITCFG_STEP_MENU, GITCFG_STEP_NO_GIT:
					m.backPressed = true
				default:
					m.clearMsg()
					m.step = GITCFG_STEP_MENU
				}
			case "up", "k":
				if m.step == GITCFG_STEP_MENU && m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.step == GITCFG_STEP_MENU && m.menuCursor < len(m.menuItems)-1 {
					m.menuCursor++
				}
			case "enter":
				return m.handleEnter()
			}
		}
	}

	return m, nil
}

func buildGitCfgMenu(remoteURL string) []string {
	remoteLabel := "Add remote URL"
	if remoteURL != "" {
		remoteLabel = "Change remote URL"
	}
	return []string{
		remoteLabel,
		"Prepare collaborative setup",
		"Generate CI/CD workflows",
		"Commit signing",
		"Manage signing keys",
		"Back",
	}
}

func (m GitConfigModel) isActionDisabled(action int) bool {
	if !m.scopesFetched {
		return false
	}
	switch action {
	case GITCFG_ACTION_SIGNING:
		return !m.scopes.WritePublicKey && !m.scopes.WriteGPGKey
	}
	return false
}

func (m GitConfigModel) handleEnter() (GitConfigModel, tea.Cmd) {
	switch m.step {
	case GITCFG_STEP_MENU:
		if m.isActionDisabled(m.menuCursor) {
			return m, nil
		}
		switch m.menuCursor {
		case GITCFG_ACTION_REMOTE:
			m.lastMsg = ""
			m.lastErr = nil
			m.step = GITCFG_STEP_REMOTE_MENU
			m.remoteMenuCursor = 0
			m.inputURL = m.remoteURL
			m.inputURLPos = len([]rune(m.inputURL))
		case GITCFG_ACTION_COLLAB:
			m.lastMsg = ""
			m.lastErr = nil
			m.collabDetecting = true
			m.step = GITCFG_STEP_COLLAB_DETECTING
			return m, detectCollabStateCmd(m.dir)
		case GITCFG_ACTION_CI:
			m.lastMsg = ""
			m.lastErr = nil
			m.step = GITCFG_STEP_CONFIRM_CI
			m.ciCursor = 0
		case GITCFG_ACTION_SIGNING:
			m.lastMsg = ""
			m.lastErr = nil
			m.signingDetecting = true
			m.manageIsActiveFlow = false
			m.step = GITCFG_STEP_SIGNING_DETECTING
			return m, detectGitSigningCmd(m.dir)
		case GITCFG_ACTION_MANAGE_KEYS:
			m.lastMsg = ""
			m.lastErr = nil
			m.signingDetecting = true
			m.manageIsActiveFlow = true
			m.step = GITCFG_STEP_MANAGE_DETECTING
			return m, detectGitSigningCmd(m.dir)
		case GITCFG_ACTION_BACK:
			m.backPressed = true
		}
	}
	return m, nil
}

func (m *GitConfigModel) clearMsg() {
	m.lastMsg = ""
	m.lastErr = nil
}

func (m GitConfigModel) signingKeys() []string {
	var keys []string
	if m.signingFormat == "ssh" {
		keys = append(keys, m.signingSSHKeys...)
	} else {
		keys = append(keys, m.signingGPGKeys...)
	}
	keys = append(keys, signingGenSentinel)
	return keys
}
