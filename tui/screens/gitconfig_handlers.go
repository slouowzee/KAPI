package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m GitConfigModel) handleCollabChecklist(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MENU
		m.menuCursor = GITCFG_ACTION_COLLAB
	case "enter":
		m.collabQIndex = 0
		m.collabCursor = 0
		m.step = GITCFG_STEP_COLLAB_QUESTIONS
	}
	return m, nil
}

func (m GitConfigModel) handleCollabQuestion(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.collabQIndex = 0
		m.collabCursor = 0
		for i := range m.collabQuestions {
			m.collabQuestions[i].answer = nil
		}
		m.step = GITCFG_STEP_COLLAB_CHECKLIST
	case "up", "k":
		if m.collabCursor > 0 {
			m.collabCursor--
		}
	case "down", "j":
		if m.collabCursor < 1 {
			m.collabCursor++
		}
	case "enter":
		answer := m.collabCursor == 0 // 0 = Yes
		m.collabQuestions[m.collabQIndex].answer = &answer
		m.collabQIndex++
		m.collabCursor = 0

		if m.collabQIndex >= len(m.collabQuestions) {
			plan := planFromAnswers(m.collabQuestions)
			m.step = GITCFG_STEP_EXECUTING
			m.execMsg = "Setting up collaborative architecture..."
			return m, execGitCollabCmd(m.dir, plan)
		}
	}
	return m, nil
}

func (m GitConfigModel) handleRemoteMenu(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MENU
	case "up", "k":
		if m.remoteMenuCursor > 0 {
			m.remoteMenuCursor--
		}
	case "down", "j":
		if m.remoteMenuCursor < 2 {
			m.remoteMenuCursor++
		}
	case "enter":
		if m.remoteMenuCursor == 0 || m.remoteMenuCursor == 1 {
			if !m.scopes.Repo {
				return m, nil
			}
			m.remoteIsPrivate = (m.remoteMenuCursor == 0)
			m.remoteRepoName = filepath.Base(m.dir)
			m.remoteNamePos = len([]rune(m.remoteRepoName))
			m.step = GITCFG_STEP_REMOTE_NAME_INPUT
		} else {
			m.step = GITCFG_STEP_REMOTE_INPUT
			m.inputURL = m.remoteURL
			m.inputURLPos = len([]rune(m.inputURL))
		}
	}
	return m, nil
}

func (m GitConfigModel) handleRemoteNameInput(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	runes := []rune(m.remoteRepoName)
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_REMOTE_MENU
	case "enter":
		name := strings.TrimSpace(m.remoteRepoName)
		if name != "" {
			m.step = GITCFG_STEP_EXECUTING
			m.execMsg = "Creating GitHub repository..."
			return m, execGithubCreateRepoCmd(name, m.remoteIsPrivate, m.dir)
		}
	case "left":
		if m.remoteNamePos > 0 {
			m.remoteNamePos--
		}
	case "right":
		if m.remoteNamePos < len(runes) {
			m.remoteNamePos++
		}
	case "backspace":
		if m.remoteNamePos > 0 {
			m.remoteRepoName = string(append(runes[:m.remoteNamePos-1], runes[m.remoteNamePos:]...))
			m.remoteNamePos--
		}
	case "delete":
		if m.remoteNamePos < len(runes) {
			m.remoteRepoName = string(append(runes[:m.remoteNamePos], runes[m.remoteNamePos+1:]...))
		}
	default:
		if len(msg.Runes) > 0 {
			m.remoteRepoName = string(append(runes[:m.remoteNamePos], append(msg.Runes, runes[m.remoteNamePos:]...)...))
			m.remoteNamePos += len(msg.Runes)
		}
	}
	return m, nil
}

func (m GitConfigModel) handleURLInput(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	runes := []rune(m.inputURL)
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_REMOTE_MENU
	case "enter":
		url := strings.TrimSpace(m.inputURL)
		if url != "" {
			m.step = GITCFG_STEP_EXECUTING
			m.execMsg = "Applying remote..."
			return m, execGitRemoteSetCmd(m.dir, url)
		}
	case "left":
		if m.inputURLPos > 0 {
			m.inputURLPos--
		}
	case "right":
		if m.inputURLPos < len(runes) {
			m.inputURLPos++
		}
	case "backspace":
		if m.inputURLPos > 0 {
			m.inputURL = string(append(runes[:m.inputURLPos-1], runes[m.inputURLPos:]...))
			m.inputURLPos--
		}
	case "delete":
		if m.inputURLPos < len(runes) {
			m.inputURL = string(append(runes[:m.inputURLPos], runes[m.inputURLPos+1:]...))
		}
	default:
		if len(msg.Runes) > 0 {
			m.inputURL = string(append(runes[:m.inputURLPos], append(msg.Runes, runes[m.inputURLPos:]...)...))
			m.inputURLPos += len(msg.Runes)
		}
	}
	return m, nil
}

func (m GitConfigModel) handleCIMenu(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MENU
		m.menuCursor = GITCFG_ACTION_CI
	case "up", "k":
		if m.ciCursor > 0 {
			m.ciCursor--
		}
	case "down", "j":
		if m.ciCursor < len(ciOptions)-1 {
			m.ciCursor++
		}
	case "enter":
		switch m.ciCursor {
		case 0:
			m.ciChoice = ciChoiceGitHub
		case 1:
			m.ciChoice = ciChoiceGitLab
		case 2:
			m.ciChoice = ciChoiceNone
		}
		m.lastErr = nil
		m.lastMsg = fmt.Sprintf("%s workflows queued for generation.", ciOptions[m.ciCursor])
		m.step = GITCFG_STEP_MENU
		m.menuCursor = GITCFG_ACTION_CI
	}
	return m, nil
}

func (m GitConfigModel) handleSigningStatus(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MENU
		m.menuCursor = GITCFG_ACTION_SIGNING
	case "enter":
		m.signingFormatCursor = 0
		m.step = GITCFG_STEP_SIGNING_FORMAT
	}
	return m, nil
}

func (m GitConfigModel) handleSigningFormat(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_SIGNING_STATUS
	case "up", "k":
		if m.signingFormatCursor > 0 {
			m.signingFormatCursor--
		}
	case "down", "j":
		if m.signingFormatCursor < len(signingFormats)-1 {
			m.signingFormatCursor++
		}
	case "enter":
		formatAvail := []bool{m.signingGPGAvailable, m.signingSSHKeygenAvail}
		if !formatAvail[m.signingFormatCursor] {
			return m, nil
		}
		if m.signingFormatCursor == 0 {
			m.signingFormat = "gpg"
		} else {
			m.signingFormat = "ssh"
		}
		m.signingScopeCursor = 0
		m.step = GITCFG_STEP_SIGNING_SCOPE
	}
	return m, nil
}

func (m GitConfigModel) handleSigningScope(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_SIGNING_FORMAT
	case "up", "k":
		if m.signingScopeCursor > 0 {
			m.signingScopeCursor--
		}
	case "down", "j":
		if m.signingScopeCursor < len(signingScopes)-1 {
			m.signingScopeCursor++
		}
	case "enter":
		switch m.signingScopeCursor {
		case 0:
			m.signingScope = "local"
		case 1:
			m.signingScope = "global"
		case 2:
			m.signingScope = "both"
		}
		m.signingKeyCursor = 0
		m.step = GITCFG_STEP_SIGNING_KEY
	}
	return m, nil
}

func (m GitConfigModel) handleSigningKey(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	keys := m.signingKeys()
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_SIGNING_SCOPE
	case "up", "k":
		if m.signingKeyCursor > 0 {
			m.signingKeyCursor--
		}
	case "down", "j":
		if m.signingKeyCursor < len(keys)-1 {
			m.signingKeyCursor++
		}
	case "enter":
		if len(keys) == 0 {
			break
		}
		selected := keys[m.signingKeyCursor]
		if selected == signingGenSentinel {
			return m, runKeyGenCmd(m.signingFormat)
		}
		m.signingKey = selected
		if (m.signingFormat == "ssh" && m.scopes.WritePublicKey) || (m.signingFormat == "gpg" && m.scopes.WriteGPGKey) {
			m.step = GITCFG_STEP_SIGNING_PUSH_GITHUB
			m.execMsg = "Configuring signing..."
			return m, execGitSigningCmd(m.dir, m.signingFormat, m.signingScope, m.signingKey)
		}
		m.step = GITCFG_STEP_EXECUTING
		m.execMsg = "Configuring commit signing..."
		return m, execGitSigningCmd(m.dir, m.signingFormat, m.signingScope, m.signingKey)
	}
	return m, nil
}

func (m GitConfigModel) handleManageFormat(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MENU
	case "up", "k":
		if m.manageFormatCursor > 0 {
			m.manageFormatCursor--
		}
	case "down", "j":
		if m.manageFormatCursor < len(signingFormats)-1 {
			m.manageFormatCursor++
		}
	case "enter":
		formatAvail := []bool{m.signingGPGAvailable, m.signingSSHKeygenAvail}
		if !formatAvail[m.manageFormatCursor] {
			return m, nil
		}
		if m.manageFormatCursor == 0 {
			m.manageFormat = "gpg"
		} else {
			m.manageFormat = "ssh"
		}
		m.manageListCursor = 0
		m.step = GITCFG_STEP_MANAGE_LIST
	}
	return m, nil
}

func (m GitConfigModel) manageKeysList() []string {
	var keys []string
	if m.manageFormat == "ssh" {
		keys = append(keys, m.signingSSHKeys...)
	} else {
		keys = append(keys, m.signingGPGKeys...)
	}
	keys = append(keys, signingGenSentinel)
	return keys
}

func (m GitConfigModel) handleManageList(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	keys := m.manageKeysList()
	switch msg.String() {
	case "esc":
		m.clearMsg()
		m.step = GITCFG_STEP_MANAGE_FORMAT
	case "up", "k":
		if m.manageListCursor > 0 {
			m.manageListCursor--
		}
	case "down", "j":
		if m.manageListCursor < len(keys)-1 {
			m.manageListCursor++
		}
	case "x", "delete":
		if len(keys) == 0 {
			break
		}
		selected := keys[m.manageListCursor]
		if selected != signingGenSentinel {
			m.manageKeyToDelete = selected
			m.manageDeleteCursor = 0
			m.step = GITCFG_STEP_MANAGE_CONFIRM_DELETE
		}
	case "enter":
		if len(keys) == 0 {
			break
		}
		selected := keys[m.manageListCursor]
		if selected == signingGenSentinel {
			return m, runKeyGenCmd(m.manageFormat)
		}
		if (m.manageFormat == "ssh" && m.scopes.WritePublicKey) || (m.manageFormat == "gpg" && m.scopes.WriteGPGKey) {
			m.step = GITCFG_STEP_MANAGE_PUSH_GITHUB
			m.execMsg = "Pushing key to GitHub..."
			return m, execGithubPushKeyCmd(m.manageFormat, selected, "KAPI Manage Key")
		}
		m.lastErr = fmt.Errorf("missing GitHub scope to push key")
		m.lastMsg = "Missing GitHub scope (requires write:public_key or write:gpg_key)"
	}
	return m, nil
}

func (m GitConfigModel) handleManageConfirmDelete(msg tea.KeyMsg) (GitConfigModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.step = GITCFG_STEP_MANAGE_LIST
	case "up", "k":
		if m.manageDeleteCursor > 0 {
			m.manageDeleteCursor--
		}
	case "down", "j":
		if m.manageDeleteCursor < 1 {
			m.manageDeleteCursor++
		}
	case "enter":
		if m.manageDeleteCursor == 0 {
			m.step = GITCFG_STEP_MANAGE_LIST
			return m, nil
		}
		m.step = GITCFG_STEP_EXECUTING
		m.execMsg = "Deleting key..."
		return m, execGitKeyDeleteCmd(m.manageFormat, m.manageKeyToDelete)
	}
	return m, nil
}
