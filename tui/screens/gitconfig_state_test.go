package screens

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
)

func gcKey(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func gcSend(m GitConfigModel, keys ...string) (GitConfigModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(gcKey(k))
	}
	return m, cmd
}

func newMenuModel(scopes config.TokenScopes) GitConfigModel {
	m := GitConfigModel{step: GITCFG_STEP_MENU, scopesFetched: true, scopes: scopes, dir: "/tmp/kapi-gitconfig-test"}
	m.menuItems = buildGitCfgMenu("")
	return m
}

// --- top-level menu -------------------------------------------------------

func TestBuildGitCfgMenu_RemoteLabel(t *testing.T) {
	tests := []struct {
		remoteURL string
		want      string
	}{
		{remoteURL: "", want: "Add remote URL"},
		{remoteURL: "git@github.com:me/app.git", want: "Change remote URL"},
	}
	for _, tt := range tests {
		items := buildGitCfgMenu(tt.remoteURL)
		if items[GITCFG_ACTION_REMOTE] != tt.want {
			t.Errorf("buildGitCfgMenu(%q)[remote] = %q, want %q", tt.remoteURL, items[GITCFG_ACTION_REMOTE], tt.want)
		}
	}
}

func TestGitConfigMenu_EnterDispatchesToTheRightStep(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		wantStep   gitcfgStep
		wantCmd    bool
		wantManage bool
	}{
		{name: "remote", cursor: GITCFG_ACTION_REMOTE, wantStep: GITCFG_STEP_REMOTE_MENU, wantCmd: false},
		{name: "collab", cursor: GITCFG_ACTION_COLLAB, wantStep: GITCFG_STEP_COLLAB_DETECTING, wantCmd: true},
		{name: "ci", cursor: GITCFG_ACTION_CI, wantStep: GITCFG_STEP_CONFIRM_CI, wantCmd: false},
		{name: "signing", cursor: GITCFG_ACTION_SIGNING, wantStep: GITCFG_STEP_SIGNING_DETECTING, wantCmd: true},
		{name: "manage keys", cursor: GITCFG_ACTION_MANAGE_KEYS, wantStep: GITCFG_STEP_MANAGE_DETECTING, wantCmd: true, wantManage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMenuModel(config.TokenScopes{Repo: true, WriteSSHSigningKey: true, WriteGPGKey: true})
			m.menuCursor = tt.cursor
			m.lastMsg = "stale"
			updated, cmd := m.Update(gcKey("enter"))
			if updated.step != tt.wantStep {
				t.Errorf("step = %v, want %v", updated.step, tt.wantStep)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("cmd != nil = %v, want %v", cmd != nil, tt.wantCmd)
			}
			if updated.manageIsActiveFlow != tt.wantManage {
				t.Errorf("manageIsActiveFlow = %v, want %v", updated.manageIsActiveFlow, tt.wantManage)
			}
			if updated.lastMsg != "" {
				t.Errorf("lastMsg = %q, want cleared", updated.lastMsg)
			}
		})
	}
}

func TestGitConfigMenu_BackAndCursorClamp(t *testing.T) {
	m := newMenuModel(config.TokenScopes{})
	m.menuCursor = GITCFG_ACTION_BACK

	updated, _ := m.Update(gcKey("enter"))
	if !updated.backPressed {
		t.Error("selecting Back should set backPressed")
	}

	m.menuCursor = 0
	updated, _ = m.Update(gcKey("up"))
	if updated.menuCursor != 0 {
		t.Errorf("cursor clamped at top = %d, want 0", updated.menuCursor)
	}

	m.menuCursor = len(m.menuItems) - 1
	updated, _ = m.Update(gcKey("down"))
	if updated.menuCursor != len(m.menuItems)-1 {
		t.Errorf("cursor clamped at bottom = %d, want %d", updated.menuCursor, len(m.menuItems)-1)
	}
}

func TestGitConfigMenu_SigningDisabledWithoutScopes(t *testing.T) {
	m := newMenuModel(config.TokenScopes{}) // no scopes at all
	m.menuCursor = GITCFG_ACTION_SIGNING

	updated, cmd := m.Update(gcKey("enter"))

	if updated.step != GITCFG_STEP_MENU || cmd != nil {
		t.Errorf("step = %v, cmd = %v, want to stay on the menu with no command", updated.step, cmd)
	}
}

func TestGitConfigMenu_BeforeScopesFetchedNothingIsDisabled(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_MENU, scopesFetched: false, dir: "/tmp/kapi-gitconfig-test"}
	m.menuItems = buildGitCfgMenu("")
	m.menuCursor = GITCFG_ACTION_SIGNING

	updated, _ := m.Update(gcKey("enter"))

	if updated.step != GITCFG_STEP_SIGNING_DETECTING {
		t.Errorf("step = %v, want signing detecting while scopes are still unknown", updated.step)
	}
}

func TestGitConfigModel_BusyStepsIgnoreKeys(t *testing.T) {
	for _, busy := range []struct {
		name  string
		model GitConfigModel
	}{
		{name: "detecting", model: GitConfigModel{step: GITCFG_STEP_DETECTING, detecting: true}},
		{name: "collab detecting", model: GitConfigModel{step: GITCFG_STEP_COLLAB_DETECTING, collabDetecting: true}},
		{name: "signing detecting", model: GitConfigModel{step: GITCFG_STEP_SIGNING_DETECTING, signingDetecting: true}},
	} {
		t.Run(busy.name, func(t *testing.T) {
			updated, cmd := busy.model.Update(gcKey("esc"))
			if updated.backPressed || cmd != nil || updated.step != busy.model.step {
				t.Errorf("a busy step must ignore key input, got step=%v back=%v cmd=%v", updated.step, updated.backPressed, cmd)
			}
		})
	}
}

// --- remote flow ------------------------------------------------------

func TestHandleRemoteMenu_GithubOptionsNeedRepoScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes config.TokenScopes
		cursor int
		want   gitcfgStep
	}{
		{name: "private without repo scope", scopes: config.TokenScopes{}, cursor: 0, want: GITCFG_STEP_REMOTE_MENU},
		{name: "public without repo scope", scopes: config.TokenScopes{}, cursor: 1, want: GITCFG_STEP_REMOTE_MENU},
		{name: "private with repo scope", scopes: config.TokenScopes{Repo: true}, cursor: 0, want: GITCFG_STEP_REMOTE_NAME_INPUT},
		{name: "existing url never needs the scope", scopes: config.TokenScopes{}, cursor: 2, want: GITCFG_STEP_REMOTE_INPUT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GitConfigModel{step: GITCFG_STEP_REMOTE_MENU, scopes: tt.scopes, remoteMenuCursor: tt.cursor, dir: "/tmp/x/my-app"}
			updated, _ := m.handleRemoteMenu(gcKey("enter"))
			if updated.step != tt.want {
				t.Errorf("step = %v, want %v", updated.step, tt.want)
			}
		})
	}
}

func TestHandleRemoteMenu_PrefillsRepoNameFromDir(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_REMOTE_MENU, scopes: config.TokenScopes{Repo: true}, remoteMenuCursor: 1, dir: "/home/me/projects/my-app"}
	updated, _ := m.handleRemoteMenu(gcKey("enter"))
	if updated.remoteRepoName != "my-app" || updated.remoteIsPrivate {
		t.Errorf("remoteRepoName = %q, remoteIsPrivate = %v, want my-app / false", updated.remoteRepoName, updated.remoteIsPrivate)
	}
}

func TestHandleURLInput_EmptyURLDoesNotSubmit(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_REMOTE_INPUT, inputURL: "   "}
	updated, cmd := m.handleURLInput(gcKey("enter"))
	if updated.step != GITCFG_STEP_REMOTE_INPUT || cmd != nil {
		t.Errorf("a blank URL must not submit, got step=%v cmd=%v", updated.step, cmd)
	}
}

func TestHandleURLInput_SubmitsTrimmedURL(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_REMOTE_INPUT, inputURL: "  https://example.com/a.git  "}
	updated, cmd := m.handleURLInput(gcKey("enter"))
	if updated.step != GITCFG_STEP_EXECUTING || cmd == nil {
		t.Errorf("step = %v, cmd = %v, want executing with a command", updated.step, cmd)
	}
}

// --- CI menu ------------------------------------------------------------

func TestHandleCIMenu_SelectsChoiceAndReturnsToMenu(t *testing.T) {
	tests := []struct {
		cursor int
		want   string
	}{
		{cursor: 0, want: ciChoiceGitHub},
		{cursor: 1, want: ciChoiceGitLab},
		{cursor: 2, want: ciChoiceNone},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_CONFIRM_CI, ciCursor: tt.cursor}
		updated, _ := m.handleCIMenu(gcKey("enter"))
		if updated.CIChoice() != tt.want {
			t.Errorf("CIChoice() = %q, want %q", updated.CIChoice(), tt.want)
		}
		if updated.step != GITCFG_STEP_MENU || updated.menuCursor != GITCFG_ACTION_CI {
			t.Errorf("step = %v menuCursor = %d, want back on the menu at the CI row", updated.step, updated.menuCursor)
		}
	}
}

// --- collab flow ----------------------------------------------------------

func TestBuildCollabQuestions_MirrorsDetectedState(t *testing.T) {
	qs := buildCollabQuestions(collabState{hasBranchDev: true, hasContributing: false, hasPRTemplate: true, hasIssueTemplates: false})
	want := []bool{true, false, true, false}
	for i, q := range qs {
		if q.exists != want[i] {
			t.Errorf("question %d (%s) exists = %v, want %v", i, q.label, q.exists, want[i])
		}
	}
}

func TestHandleCollabQuestion_BuildsPlanFromAnswersAndExecutes(t *testing.T) {
	m := GitConfigModel{
		step:            GITCFG_STEP_COLLAB_QUESTIONS,
		collabQuestions: buildCollabQuestions(collabState{}),
		dir:             "/tmp/kapi-gitconfig-test",
	}

	// yes, no, yes, no
	answers := []string{"up", "enter", "down", "enter", "up", "enter", "down", "enter"}
	var cmd tea.Cmd
	for i := 0; i < len(answers); i += 2 {
		m, cmd = gcSend(m, answers[i], answers[i+1])
	}

	if m.step != GITCFG_STEP_EXECUTING || cmd == nil {
		t.Fatalf("step = %v, cmd = %v, want executing with a command once every question is answered", m.step, cmd)
	}

	plan := planFromAnswers(m.collabQuestions)
	want := collabPlan{createBranch: true, writeContributing: false, writePRTemplate: true, writeIssueTemplates: false}
	if plan != want {
		t.Errorf("plan = %+v, want %+v", plan, want)
	}
}

func TestHandleCollabQuestion_EscResetsAllAnswers(t *testing.T) {
	answer := true
	m := GitConfigModel{
		step:         GITCFG_STEP_COLLAB_QUESTIONS,
		collabQIndex: 2,
		collabQuestions: []collabQuestion{
			{label: "a", answer: &answer},
			{label: "b", answer: &answer},
		},
	}
	updated, _ := m.handleCollabQuestion(gcKey("esc"))

	if updated.step != GITCFG_STEP_COLLAB_CHECKLIST || updated.collabQIndex != 0 {
		t.Errorf("step = %v qIndex = %d, want back at the checklist reset to 0", updated.step, updated.collabQIndex)
	}
	for i, q := range updated.collabQuestions {
		if q.answer != nil {
			t.Errorf("question %d answer = %v, want cleared", i, *q.answer)
		}
	}
}

// --- signing flow -----------------------------------------------------

func TestHandleSigningFormat_UnavailableFormatIsANoOp(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_SIGNING_FORMAT, signingGPGAvailable: false, signingSSHKeygenAvail: true, signingFormatCursor: 0}
	updated, _ := m.handleSigningFormat(gcKey("enter"))
	if updated.step != GITCFG_STEP_SIGNING_FORMAT || updated.signingFormat != "" {
		t.Errorf("step = %v format = %q, want no transition for an unavailable format", updated.step, updated.signingFormat)
	}
}

func TestHandleSigningFormat_AvailableFormatAdvances(t *testing.T) {
	tests := []struct {
		cursor int
		want   string
	}{
		{cursor: 0, want: "gpg"},
		{cursor: 1, want: "ssh"},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_SIGNING_FORMAT, signingGPGAvailable: true, signingSSHKeygenAvail: true, signingFormatCursor: tt.cursor}
		updated, _ := m.handleSigningFormat(gcKey("enter"))
		if updated.step != GITCFG_STEP_SIGNING_SCOPE || updated.signingFormat != tt.want {
			t.Errorf("step = %v format = %q, want SIGNING_SCOPE / %q", updated.step, updated.signingFormat, tt.want)
		}
	}
}

func TestSigningKeys_GPGvsSSHAndGenerateSentinel(t *testing.T) {
	m := GitConfigModel{signingFormat: "ssh", signingSSHKeys: []string{"a.pub"}, signingGPGKeys: []string{"KEYID"}}
	keys := m.signingKeys()
	if len(keys) != 2 || keys[0] != "a.pub" || keys[1] != signingGenSentinel {
		t.Errorf("signingKeys() = %v, want [a.pub, sentinel]", keys)
	}
}

func TestHandleSigningKey_GenerateSentinelRunsKeygen(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_SIGNING_KEY, signingFormat: "ssh", signingSSHKeys: nil, signingKeyCursor: 0}
	updated, cmd := m.handleSigningKey(gcKey("enter"))
	if cmd == nil || updated.step != GITCFG_STEP_SIGNING_KEY {
		t.Errorf("step = %v, cmd = %v, want the keygen command with no step change yet", updated.step, cmd)
	}
}

func TestHandleSigningKey_PushesToGithubOnlyWithScope(t *testing.T) {
	tests := []struct {
		name   string
		format string
		scopes config.TokenScopes
		want   gitcfgStep
	}{
		{name: "ssh with scope", format: "ssh", scopes: config.TokenScopes{WriteSSHSigningKey: true}, want: GITCFG_STEP_SIGNING_PUSH_GITHUB},
		{name: "ssh without scope", format: "ssh", scopes: config.TokenScopes{}, want: GITCFG_STEP_EXECUTING},
		{name: "gpg with scope", format: "gpg", scopes: config.TokenScopes{WriteGPGKey: true}, want: GITCFG_STEP_SIGNING_PUSH_GITHUB},
		{name: "gpg scope does not cover ssh", format: "ssh", scopes: config.TokenScopes{WriteGPGKey: true}, want: GITCFG_STEP_EXECUTING},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GitConfigModel{
				step: GITCFG_STEP_SIGNING_KEY, signingFormat: tt.format, scopes: tt.scopes,
				signingSSHKeys: []string{"key.pub"}, signingGPGKeys: []string{"KEYID"}, dir: "/tmp/x",
			}
			updated, cmd := m.handleSigningKey(gcKey("enter"))
			if updated.step != tt.want || cmd == nil {
				t.Errorf("step = %v, cmd = %v, want %v with a command", updated.step, cmd, tt.want)
			}
		})
	}
}

// --- manage keys flow -------------------------------------------------

func TestHandleManageList_DeleteEntersConfirmation(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_MANAGE_LIST, manageFormat: "ssh", signingSSHKeys: []string{"a.pub", "b.pub"}, manageListCursor: 1}
	updated, _ := m.handleManageList(gcKey("delete"))
	if updated.step != GITCFG_STEP_MANAGE_CONFIRM_DELETE || updated.manageKeyToDelete != "b.pub" {
		t.Errorf("step = %v keyToDelete = %q, want confirm delete on b.pub", updated.step, updated.manageKeyToDelete)
	}
}

func TestHandleManageList_CannotDeleteTheGenerateRow(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_MANAGE_LIST, manageFormat: "ssh", signingSSHKeys: nil, manageListCursor: 0}
	updated, _ := m.handleManageList(gcKey("delete"))
	if updated.step != GITCFG_STEP_MANAGE_LIST {
		t.Errorf("step = %v, the generate row must not be deletable", updated.step)
	}
}

func TestHandleManageList_EnterWithoutScopeReportsError(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_MANAGE_LIST, manageFormat: "gpg", signingGPGKeys: []string{"KEYID"}, manageListCursor: 0, scopes: config.TokenScopes{}}
	updated, cmd := m.handleManageList(gcKey("enter"))
	if cmd != nil || updated.step != GITCFG_STEP_MANAGE_LIST || updated.lastErr == nil {
		t.Errorf("step = %v cmd = %v err = %v, want to stay put with an error and no command", updated.step, cmd, updated.lastErr)
	}
}

func TestHandleManageConfirmDelete_CancelVsConfirm(t *testing.T) {
	tests := []struct {
		name    string
		cursor  int
		want    gitcfgStep
		wantCmd bool
	}{
		{name: "cancel", cursor: 0, want: GITCFG_STEP_MANAGE_LIST, wantCmd: false},
		{name: "confirm", cursor: 1, want: GITCFG_STEP_EXECUTING, wantCmd: true},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_MANAGE_CONFIRM_DELETE, manageDeleteCursor: tt.cursor, manageFormat: "ssh", manageKeyToDelete: "a.pub"}
		updated, cmd := m.handleManageConfirmDelete(gcKey("enter"))
		if updated.step != tt.want || (cmd != nil) != tt.wantCmd {
			t.Errorf("%s: step = %v cmd!=nil = %v, want %v / %v", tt.name, updated.step, cmd != nil, tt.want, tt.wantCmd)
		}
	}
}

// --- top-level async message handling ----------------------------------

func TestUpdate_GitDetection_NoGitVsHasGit(t *testing.T) {
	tests := []struct {
		name      string
		hasGit    bool
		remoteURL string
		want      gitcfgStep
	}{
		{name: "no git", hasGit: false, want: GITCFG_STEP_NO_GIT},
		{name: "git with remote", hasGit: true, remoteURL: "git@github.com:me/app.git", want: GITCFG_STEP_MENU},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_DETECTING, detecting: true}
		updated, _ := m.Update(gitcfgDetectionMsg{hasGit: tt.hasGit, remoteURL: tt.remoteURL})
		if updated.step != tt.want || updated.detecting {
			t.Errorf("step = %v detecting = %v, want %v / false", updated.step, updated.detecting, tt.want)
		}
	}
}

func TestUpdate_ExecMsg_SuccessUpdatesRemoteAndError(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_EXECUTING}

	updated, _ := m.Update(gitcfgExecMsg{err: errors.New("boom")})
	if updated.step != GITCFG_STEP_MENU || updated.lastErr == nil {
		t.Errorf("failure: step = %v err = %v, want back on the menu with an error", updated.step, updated.lastErr)
	}

	updated, _ = m.Update(gitcfgExecMsg{newRemoteURL: "git@github.com:me/app.git", successMsg: "done"})
	if updated.step != GITCFG_STEP_MENU || updated.remoteURL != "git@github.com:me/app.git" || updated.lastErr != nil {
		t.Errorf("success: step=%v url=%q err=%v, want the remote applied with no error", updated.step, updated.remoteURL, updated.lastErr)
	}
	if updated.menuItems[GITCFG_ACTION_REMOTE] != "Change remote URL" {
		t.Errorf("menu not rebuilt after the remote URL changed: %v", updated.menuItems)
	}
}

func TestUpdate_SigningDoneMsg_PushStepChainsToGithubPush(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_SIGNING_PUSH_GITHUB, signingFormat: "ssh", signingKey: "key.pub"}
	updated, cmd := m.Update(gitcfgSigningDoneMsg{successMsg: "signing configured"})
	if updated.step != GITCFG_STEP_SIGNING_PUSH_GITHUB || cmd == nil {
		t.Errorf("step = %v cmd = %v, want to stay on the push step while the push command runs", updated.step, cmd)
	}
}

func TestUpdate_SigningDoneMsg_ErrorReturnsToMenu(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_SIGNING_KEY}
	updated, cmd := m.Update(gitcfgSigningDoneMsg{err: errors.New("nope")})
	if updated.step != GITCFG_STEP_MENU || cmd != nil || updated.lastErr == nil {
		t.Errorf("step = %v cmd = %v err = %v, want back on the menu with the error surfaced", updated.step, cmd, updated.lastErr)
	}
}

func TestUpdate_GithubPushDoneMsg_ReturnsToManageOrMenu(t *testing.T) {
	tests := []struct {
		name       string
		manageFlow bool
		want       gitcfgStep
	}{
		{name: "from the signing flow", manageFlow: false, want: GITCFG_STEP_MENU},
		{name: "from the manage-keys flow", manageFlow: true, want: GITCFG_STEP_MANAGE_LIST},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_SIGNING_PUSH_GITHUB, manageIsActiveFlow: tt.manageFlow, lastMsg: "Commit signing enabled."}
		updated, _ := m.Update(gitcfgGithubPushDoneMsg{successMsg: "Key pushed to GitHub."})
		if updated.step != tt.want {
			t.Errorf("step = %v, want %v", updated.step, tt.want)
		}
		if updated.lastMsg != "Commit signing enabled. Key pushed to GitHub." {
			t.Errorf("lastMsg = %q, want both messages joined", updated.lastMsg)
		}
	}
}

func TestUpdate_GithubPushDoneMsg_ErrorDoesNotJoinMessages(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_SIGNING_PUSH_GITHUB, lastMsg: "Commit signing enabled."}
	updated, _ := m.Update(gitcfgGithubPushDoneMsg{err: errors.New("rate limited")})
	if updated.lastErr == nil || updated.lastMsg != "rate limited" {
		t.Errorf("lastErr = %v lastMsg = %q, want the error surfaced alone", updated.lastErr, updated.lastMsg)
	}
}

func TestUpdate_KeyGenMsg_SuccessRedetectsSigningState(t *testing.T) {
	tests := []struct {
		name       string
		manageFlow bool
		want       gitcfgStep
	}{
		{name: "signing flow", manageFlow: false, want: GITCFG_STEP_SIGNING_DETECTING},
		{name: "manage flow", manageFlow: true, want: GITCFG_STEP_MANAGE_DETECTING},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_SIGNING_KEY, manageIsActiveFlow: tt.manageFlow, dir: "/tmp/x"}
		updated, cmd := m.Update(gitcfgKeyGenMsg{})
		if updated.step != tt.want || cmd == nil || !updated.signingReturnToKey {
			t.Errorf("step = %v cmd = %v returnToKey = %v, want %v with a command and returnToKey set", updated.step, cmd, updated.signingReturnToKey, tt.want)
		}
	}
}

func TestUpdate_KeyGenMsg_ErrorGoesBackWithoutRedetecting(t *testing.T) {
	tests := []struct {
		name       string
		manageFlow bool
		want       gitcfgStep
	}{
		{name: "signing flow", manageFlow: false, want: GITCFG_STEP_SIGNING_KEY},
		{name: "manage flow", manageFlow: true, want: GITCFG_STEP_MANAGE_LIST},
	}
	for _, tt := range tests {
		m := GitConfigModel{step: GITCFG_STEP_SIGNING_KEY, manageIsActiveFlow: tt.manageFlow}
		updated, cmd := m.Update(gitcfgKeyGenMsg{err: errors.New("no entropy")})
		if updated.step != tt.want || cmd != nil || updated.lastErr == nil {
			t.Errorf("step = %v cmd = %v err = %v, want %v with no command and an error", updated.step, cmd, updated.lastErr, tt.want)
		}
	}
}

func TestUpdate_SigningDetectionMsg_FourBranches(t *testing.T) {
	tests := []struct {
		name         string
		manageFlow   bool
		returnToKey  bool
		want         gitcfgStep
		wantSSHKeys  []string
		wantCursorAt int // -1 = don't check
	}{
		{name: "fresh signing flow", manageFlow: false, returnToKey: false, want: GITCFG_STEP_SIGNING_STATUS, wantCursorAt: -1},
		{name: "back to key picker after keygen", manageFlow: false, returnToKey: true, want: GITCFG_STEP_SIGNING_KEY, wantCursorAt: -1},
		{name: "fresh manage flow", manageFlow: true, returnToKey: false, want: GITCFG_STEP_MANAGE_FORMAT, wantCursorAt: -1},
		{name: "back to manage list after keygen", manageFlow: true, returnToKey: true, want: GITCFG_STEP_MANAGE_LIST, wantSSHKeys: []string{"a.pub", "b.pub"}, wantCursorAt: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GitConfigModel{
				step: GITCFG_STEP_SIGNING_DETECTING, manageIsActiveFlow: tt.manageFlow, signingReturnToKey: tt.returnToKey,
				signingFormat: "ssh", manageFormat: "ssh",
			}
			msg := gitcfgSigningDetectionMsg{sshKeys: tt.wantSSHKeys}
			updated, _ := m.Update(msg)
			if updated.step != tt.want || updated.signingReturnToKey {
				t.Errorf("step = %v returnToKey = %v, want %v / false", updated.step, updated.signingReturnToKey, tt.want)
			}
			if tt.wantCursorAt >= 0 && updated.manageListCursor != tt.wantCursorAt {
				t.Errorf("manageListCursor = %d, want %d (last real key before the generate row)", updated.manageListCursor, tt.wantCursorAt)
			}
		})
	}
}

func TestUpdate_KeyDeleteMsg_SuccessVsError(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_MANAGE_CONFIRM_DELETE}

	updated, cmd := m.Update(gitcfgKeyDeleteMsg{err: errors.New("permission denied")})
	if updated.step != GITCFG_STEP_MANAGE_LIST || cmd != nil || updated.lastErr == nil {
		t.Errorf("failure: step=%v cmd=%v err=%v, want back on the list with the error and no command", updated.step, cmd, updated.lastErr)
	}

	updated, cmd = m.Update(gitcfgKeyDeleteMsg{})
	if updated.step != GITCFG_STEP_MANAGE_DETECTING || cmd == nil || updated.lastErr != nil {
		t.Errorf("success: step=%v cmd=%v err=%v, want a redetect command and no error", updated.step, cmd, updated.lastErr)
	}
}

// --- input mode ---------------------------------------------------------

func TestGitConfigModel_IsInputMode(t *testing.T) {
	for step := GITCFG_STEP_DETECTING; step <= GITCFG_STEP_MANAGE_PUSH_GITHUB; step++ {
		m := GitConfigModel{step: step}
		want := step == GITCFG_STEP_REMOTE_INPUT || step == GITCFG_STEP_REMOTE_NAME_INPUT
		if got := m.IsInputMode(); got != want {
			t.Errorf("step %v: IsInputMode() = %v, want %v", step, got, want)
		}
	}
}
