package actions

import (
	"context"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func NewGetRemoteStackedPRModel(
	repo meta.Repository,
	ghClient *gh.Client,
	initialBranchName string,
	onDone func([]RemotePRInfo) tea.Cmd,
) tea.Model {
	return &GetRemoteStackedPRModel{
		repo:              repo,
		ghClient:          ghClient,
		spinner:           spinner.New(spinner.WithSpinner(spinner.Dot)),
		initialBranchName: initialBranchName,
		onDone:            onDone,
	}
}

type RemotePRInfo struct {
	Name        string
	Parent      meta.BranchState
	PullRequest meta.PullRequest
	MergeCommit string
	Title       string
}

type GetRemoteStackedPRModel struct {
	repo              meta.Repository
	ghClient          *gh.Client
	spinner           spinner.Model
	initialBranchName string
	onDone            func([]RemotePRInfo) tea.Cmd
	done              bool
	failed            bool
	prs               []RemotePRInfo
}

func (m *GetRemoteStackedPRModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		ctx := context.Background()
		nextPRNumber := int64(0)
		for {
			var pr *gh.PullRequest
			if nextPRNumber == 0 {
				// Initial branch is searched based on the branch name.
				page, err := m.ghClient.GetPullRequests(ctx, gh.GetPullRequestsInput{
					Owner:       m.repo.Owner,
					Repo:        m.repo.Name,
					HeadRefName: m.initialBranchName,
				})
				if err != nil {
					m.failed = true
					return err
				}
				if len(page.PullRequests) == 0 {
					m.failed = true
					return errors.New("cannot find PR for branch " + m.initialBranchName)
				}
				if len(page.PullRequests) > 1 {
					m.failed = true
					return errors.New("multiple PRs found for branch " + m.initialBranchName)
				}
				pr = &page.PullRequests[0]
			} else {
				// Otherwise, we can fetch based on the PR number.
				var err error
				pr, err = m.ghClient.GetPullRequestByNumber(ctx, gh.GetPullRequestByNumberInput{
					Owner:  m.repo.Owner,
					Repo:   m.repo.Name,
					Number: nextPRNumber,
				})
				if err != nil {
					m.failed = true
					return errors.Wrapf(err, "failed to get PR %d", nextPRNumber)
				}
			}
			prMeta, err := ReadPRMetadata(pr.Body)
			if errors.Is(err, ErrNoPRMetadata) {
				prMeta = PRMetadata{
					Parent:     pr.BaseBranchName(),
					Trunk:      pr.BaseBranchName(),
					ParentHead: "",
					ParentPull: 0,
				}
			} else if err != nil {
				m.failed = true
				return errors.Wrapf(err, "failed to read metadata for PR %d", pr.Number)
			}
			remotePRInfo := RemotePRInfo{
				Name: strings.TrimPrefix(pr.HeadRefName, "refs/heads/"),
				Parent: meta.BranchState{
					Name:                     prMeta.Parent,
					Trunk:                    prMeta.Trunk == prMeta.Parent,
					BranchingPointCommitHash: prMeta.ParentHead,
				},
				PullRequest: meta.PullRequest{
					ID:        pr.ID,
					Number:    pr.Number,
					Permalink: pr.Permalink,
					State:     pr.State,
				},
				MergeCommit: pr.GetMergeCommit(),
				Title:       pr.Title,
			}
			m.prs = append(m.prs, remotePRInfo)
			if remotePRInfo.Parent.Trunk {
				break
			}
			nextPRNumber = prMeta.ParentPull
		}
		m.done = true
		return uiutils.SimpleCommandMsg{Cmd: m.onDone(m.prs)}
	})
}

func (m *GetRemoteStackedPRModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *GetRemoteStackedPRModel) View() string {
	if m.done {
		return colors.SuccessStyle.Render("✓ Found adoptable branches on remote.")
	}
	message := ""
	if len(m.prs) == 1 {
		message = " (found 1 PR)"
	} else if len(m.prs) > 1 {
		message = fmt.Sprintf(" (found %d PRs)", len(m.prs))
	}
	message = "Finding adoptable branches on remote" + message + "..."
	if m.failed {
		return colors.FailureStyle.Render("✗ " + message)
	}
	var lines []string
	lines = append(lines, colors.ProgressStyle.Render(m.spinner.View()+message), "")
	for _, prInfo := range m.prs {
		lines = append(lines, fmt.Sprintf("  * %s (%s)", prInfo.Title, prInfo.PullRequest.Permalink))
	}
	return strings.Join(lines, "\n")
}
