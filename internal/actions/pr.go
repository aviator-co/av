package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aviator-co/av/internal/utils/errutils"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/editor"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/browser"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/sanitize"
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/aviator-co/av/internal/utils/templateutils"
	"github.com/fatih/color"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
)

type CreatePullRequestOpts struct {
	// The HEAD branch to create a pull request for.
	BranchName string
	// The pull request title.
	Title string
	// The pull request body (description).
	Body string
	// If true, create the pull request as a GitHub draft PR.
	Draft bool
	// If true, do not push the branch to GitHub
	NoPush bool
	// If true, force push the branch to GitHub
	ForcePush bool
	// If true, create a PR even if we think one already exists
	Force bool
	// If true, open an editor for editing the title and body
	Edit bool
}

type CreatePullRequestResult struct {
	// True if the pull request was created
	Created bool
	// The (updated) branch metadata.
	Branch meta.Branch
	// The pull request object that was returned from GitHub
	Pull *gh.PullRequest
}

// getPRMetadata constructs the PRMetadata for the current state of the branch.
// TODO:
// The way we pass/load all the relevant data here is not great(tm). The
// `parent` argument is optional because sometimes it's loadeed by the
// calling function and sometimes not. :shrug: It can also be nil if the
// branch doesn't have a parent (i.e., the branch is a stack root).
func getPRMetadata(
	tx meta.ReadTx,
	branch meta.Branch,
	parent *meta.Branch,
) (PRMetadata, error) {
	trunk, _ := meta.Trunk(tx, branch.Name)
	prMeta := PRMetadata{
		Parent:     branch.Parent.Name,
		ParentHead: branch.Parent.Head,
		Trunk:      trunk,
	}
	if parent == nil && branch.Parent.Name != "" {
		p, _ := tx.Branch(branch.Parent.Name)
		parent = &p
	}
	if parent != nil && parent.PullRequest != nil {
		prMeta.ParentPull = parent.PullRequest.Number
	}
	return prMeta, nil
}

type errPullRequestClosed struct {
	*gh.PullRequest
}

func (e errPullRequestClosed) Error() string {
	return fmt.Sprintf("pull request #%d is %s", e.Number, e.State)
}

// getExistingOpenPR returns an existing pull request for the given branch if
// any exist and are open.
func getExistingOpenPR(
	ctx context.Context,
	client *gh.Client,
	repoMeta meta.Repository,
	branchMeta meta.Branch,
	baseRefName string,
) (*gh.PullRequest, error) {
	if branchMeta.PullRequest != nil {
		logrus.WithField("pr", branchMeta.PullRequest.Number).
			Debug("querying data for existing PR from GitHub")
		pr, err := client.PullRequest(ctx, branchMeta.PullRequest.ID)
		if err != nil {
			return nil, errors.WrapIf(err, "querying existing pull request")
		}
		if pr.State != githubv4.PullRequestStateOpen {
			return nil, errPullRequestClosed{pr}
		}
		return pr, nil
	}
	logrus.WithField("branch", branchMeta.Name).Debug("querying existing open PRs from GitHub")
	existing, err := client.GetPullRequests(ctx, gh.GetPullRequestsInput{
		Owner:       repoMeta.Owner,
		Repo:        repoMeta.Name,
		HeadRefName: branchMeta.Name,
		BaseRefName: baseRefName,
		States:      []githubv4.PullRequestState{githubv4.PullRequestStateOpen},
	})
	if err != nil {
		return nil, errors.WrapIf(err, "querying existing pull requests")
	}
	if len(existing.PullRequests) > 1 {
		return nil, errors.Errorf("multiple existing PRs found for %q", branchMeta.Name)
	} else if len(existing.PullRequests) == 1 {
		return &existing.PullRequests[0], nil
	}
	return nil, nil
}

// CreatePullRequest creates a pull request on GitHub for the current branch, if
// one doesn't already exist.
func CreatePullRequest(
	ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	opts CreatePullRequestOpts,
) (_ *CreatePullRequestResult, reterr error) {
	if opts.BranchName == "" {
		logrus.Panicf("internal invariant error: CreatePullRequest called with empty branch name")
	}

	repoMeta, ok := tx.Repository()
	if !ok {
		return nil, ErrRepoNotInitialized
	}
	branchMeta, _ := tx.Branch(opts.BranchName)

	var existingPR *gh.PullRequest
	if !opts.Force {
		var err error
		existingPR, err = getExistingOpenPR(ctx, client, repoMeta, branchMeta, opts.BranchName)
		if closed, ok := errutils.As[errPullRequestClosed](err); ok {
			_, _ = fmt.Fprint(os.Stderr,
				colors.Failure("Existing pull request for branch "),
				colors.UserInput(opts.BranchName),
				colors.Failure(" is "), colors.UserInput(closed.State),
				colors.Failure(": "), colors.UserInput(closed.Permalink),
				"\n",
			)
			_, _ = fmt.Fprint(os.Stderr,
				colors.Faint("  - use "), colors.CliCmd("av pr create --force"),
				colors.Faint(" to create a new pull request for this branch\n"),
			)
			return nil, err
		} else if err != nil {
			return nil, errors.WrapIf(err, "failed to get existing pull request")
		}
	} else {
		// If we're forcing creation of a new PR, we always want to force push.
		// This prevents errors where we try to use `--force-with-lease` but the
		// remote branch has been deleted.
		opts.ForcePush = true
	}

	verb := "Creating"
	if existingPR != nil {
		verb = "Updating"
	}
	_, _ = fmt.Fprint(os.Stderr,
		verb, " pull request for branch ", colors.UserInput(opts.BranchName), ":",
		"\n",
	)
	if !opts.NoPush || opts.ForcePush {
		pushFlags := []string{"push"}

		if opts.ForcePush {
			pushFlags = append(pushFlags, "--force")
		} else {
			// Use --force-with-lease to allow pushing branches that have been
			// rebased but don't overwrite changes if we don't expect them to
			// be there.
			pushFlags = append(pushFlags, "--force-with-lease")
		}

		// NOTE: This assumes that the user use the default push strategy (simple). It would
		// be rare to use the upstream strategy.
		pushFlags = append(pushFlags, "origin", opts.BranchName)
		logrus.Debug("pushing latest changes")

		_, _ = fmt.Fprint(os.Stderr,
			"  - pushing to ", color.CyanString("origin/%s", opts.BranchName),
			"\n",
		)
		if _, err := repo.Git(pushFlags...); err != nil {
			return nil, errors.WrapIf(err, "failed to push")
		}
		if err := repo.BranchSetConfig(opts.BranchName, "av-pushed-remote", "origin"); err != nil {
			return nil, err
		}
		if err := repo.BranchSetConfig(opts.BranchName, "av-pushed-ref", fmt.Sprintf("refs/heads/%s", opts.BranchName)); err != nil {
			return nil, err
		}
	} else {
		_, _ = fmt.Fprint(os.Stderr,
			"  - skipping push to GitHub",
			"\n",
		)
	}

	// figure this out based on whether or not we're on a stacked branch
	parentState := branchMeta.Parent
	if parentState.Name == "" {
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return nil, errors.WrapIf(err, "failed to determine default branch")
		}
		parentState = meta.BranchState{
			Name:  defaultBranch,
			Trunk: true,
		}
		branchMeta.Parent = parentState
	}
	prCompareRef := parentState.Name
	var parentMeta meta.Branch
	if !parentState.Trunk {
		// check if the base branch also has an associated PR
		var ok bool
		parentMeta, ok = tx.Branch(parentState.Name)
		if !ok {
			return nil, errors.Errorf("failed to read branch metadata for %q", parentState.Name)
		}
		if parentMeta.PullRequest == nil {
			// TODO:
			//     We should automagically create PRs for every branch in the stack
			return nil, errors.Errorf(
				"base branch %q does not have an associated pull request "+
					"(create one by checking out the branch and running `av pr create`)",
				parentState.Name,
			)
		}
	} else {
		logrus.WithField("base", parentState.Name).Debug("base branch is a trunk branch")
		prCompareRef = "origin/" + parentState.Name
	}

	commitsList, err := repo.Git("rev-list", "--reverse", fmt.Sprintf("%s..%s", prCompareRef, opts.BranchName))
	if err != nil {
		return nil, errors.WrapIf(err, "failed to determine commits to include in PR")
	}
	if commitsList == "" {
		return nil, errors.Errorf("no commits between %q and %q", prCompareRef, opts.BranchName)
	}

	if existingPR != nil {
		// If there's an existing PR, use that as the new PR title and body. If --edit is
		// used, an editor is opened later.
		if opts.Title == "" {
			opts.Title = existingPR.Title
		}
		if opts.Body == "" {
			opts.Body = existingPR.Body
		}
	}

	if opts.Edit || opts.Body == "" || opts.Title == "" {
		var commits []git.CommitInfo
		for _, commitHash := range strings.Split(commitsList, "\n") {
			commit, err := repo.CommitInfo(git.CommitInfoOpts{Rev: commitHash})
			if err != nil {
				return nil, errors.WrapIff(err, "failed to get commit info for %q", commitHash)
			}
			commits = append(commits, *commit)
		}

		// If a saved pull request description exists, use that.
		saveFile := filepath.Join(
			repo.AvTmpDir(),
			fmt.Sprintf("av-pr-%s.md", sanitize.FileName(opts.BranchName)),
		)
		if _, err := os.Stat(saveFile); err == nil {
			contents, err := os.ReadFile(saveFile)
			if err != nil {
				logrus.WithError(err).Warn("failed to read saved pull request description")
			} else {
				title, body := stringutils.ParseSubjectBody(string(contents))
				if opts.Title == "" {
					opts.Title = title
				}
				if opts.Body == "" {
					opts.Body = body
				}
			}
		}

		// Try to populate the editor text using contextual information from the
		// repository and commits included in this pull request.
		if opts.Title == "" {
			opts.Title = commits[0].Subject
		}
		// Reasonable defaults for body:
		// 1. Try and find a pull request template
		if opts.Body == "" {
			opts.Body = readDefaultPullRequestTemplate(repo)
		}
		// 2. Use the commit message from the first PR
		if opts.Body == "" {
			opts.Body = commits[0].Body
		}

		editorText := templateutils.MustString(prBodyTemplate, prBodyTemplateData{
			Branch:  opts.BranchName,
			Title:   opts.Title,
			Body:    opts.Body,
			Commits: commits,
		})

		res, err := editor.Launch(repo, editor.Config{
			Text:           editorText,
			TmpFilePattern: "pr-*.av.md",
			CommentPrefix:  "%%",
		})
		if err != nil {
			if res != "" {
				savePRDescriptionToTemporaryFile(saveFile, res)
			}
			return nil, errors.WrapIf(err, "text editor failed")

		}
		opts.Title, opts.Body = stringutils.ParseSubjectBody(res)
		if opts.Title == "" {
			return nil, errors.New("aborting pull request due to empty message")
		}
		// The tailing new line is needed for compare with `PRMetadataCommentEnd` during the metadata parsing.
		opts.Body += "\n"

		defer func() {
			// If we created the PR successfully, just make sure to clean up any
			// lingering files.
			if reterr == nil {
				_ = os.Remove(saveFile)
				return
			}

			// Otherwise, save what the user entered to a file so that it's not
			// lost forever (and we can re-use it if they try again).
			savePRDescriptionToTemporaryFile(saveFile, res)
		}()
	}

	prMeta, err := getPRMetadata(tx, branchMeta, &parentMeta)
	if err != nil {
		return nil, err
	}

	draft := opts.Draft
	if !config.Av.PullRequest.NoWIPDetection && strings.Contains(opts.Title, "WIP") {
		draft = true
	}

	pull, didCreatePR, err := ensurePR(ctx, client, repoMeta, ensurePROpts{
		baseRefName: parentState.Name,
		headRefName: opts.BranchName,
		title:       opts.Title,
		body:        opts.Body,
		meta:        prMeta,
		draft:       draft,
		existingPR:  existingPR,
	})
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("  - failed to create pull request: "), err, "\n",
		)
		return nil, errors.WrapIf(err, "failed to create PR")
	}

	branchMeta.PullRequest = &meta.PullRequest{
		Number:    pull.Number,
		ID:        pull.ID,
		Permalink: pull.Permalink,
	}
	// It's possible that a new PR is created with the same branch. Reset the MergeCommit.
	branchMeta.MergeCommit = ""
	var action string
	if didCreatePR {
		action = "created"
	} else {
		action = "synchronized"
	}
	_, _ = fmt.Fprint(os.Stderr,
		"  - ", action, " pull request ",
		colors.UserInput(pull.Permalink), "\n",
	)

	if didCreatePR && config.Av.PullRequest.OpenBrowser {
		if err := browser.Open(pull.Permalink); err != nil {
			_, _ = fmt.Fprint(os.Stderr,
				"  - couldn't open browser ",
				colors.UserInput(err),
				" for pull request link ",
				colors.UserInput(pull.Permalink),
			)
		}
	}

	tx.SetBranch(branchMeta)
	return &CreatePullRequestResult{didCreatePR, branchMeta, pull}, nil
}

func savePRDescriptionToTemporaryFile(saveFile string, contents string) {
	if err := os.WriteFile(saveFile, []byte(contents), 0644); err != nil {
		logrus.WithError(err).
			Error("failed to write pull request description to temporary file")
		return
	}
	_, _ = fmt.Fprint(os.Stderr,
		"  - saved pull request description to ", colors.UserInput(saveFile),
		" (it will be automatically re-used if you try again)\n",
	)
}

type prBodyTemplateData struct {
	Branch  string
	Title   string
	Body    string
	Commits []git.CommitInfo
}

var templateFuncs = template.FuncMap{"trimSpace": strings.TrimSpace}

var prBodyTemplate = template.Must(
	template.New("prBody").
		Funcs(templateFuncs).
		Parse(`%% Creating pull request for branch '{{ .Branch }}'
%% Lines starting with '%%' will be ignored and an empty message aborts the
%% creation of the pull request.

%% Pull request title (single line)
{{ .Title }}

%% Pull request body (multiple lines)
{{ .Body }}

%% This branch includes the following commits:
{{- range $c := .Commits }}
%%     {{ $c.ShortHash }}    {{ $c.Subject }}
{{- if trimSpace $c.Body }}
{{- range $line := $c.BodyWithPrefix "%%         " }}
{{ trimSpace $line }}
{{- end }}
{{- end }}
{{- end }}
`),
)

func readDefaultPullRequestTemplate(repo *git.Repo) string {
	for _, dir := range []string{"", ".github", "data"} {
		for _, f := range []string{
			"PULL_REQUEST_TEMPLATE.md",
			"pull_request_template.md",
		} {
			tpl := filepath.Join(repo.Dir(), dir, f)
			data, err := os.ReadFile(tpl)
			if err != nil {
				continue
			}
			return string(data)
		}
	}
	return ""
}

type ensurePROpts struct {
	baseRefName string
	headRefName string
	title       string
	body        string
	meta        PRMetadata
	draft       bool
	existingPR  *gh.PullRequest
}

// ensurePR returns the pull request for the given input, creating a new
// pull request if one doesn't exist. It returns the pull request, a boolean
// indicating whether or not the pull request was created, and an error if one
// occurred.
func ensurePR(
	ctx context.Context,
	client *gh.Client,
	repoMeta meta.Repository,
	opts ensurePROpts,
) (*gh.PullRequest, bool, error) {
	if opts.existingPR != nil {
		newBody := AddPRMetadata(opts.body, opts.meta)
		updatedPR, err := client.UpdatePullRequest(ctx, githubv4.UpdatePullRequestInput{
			PullRequestID: opts.existingPR.ID,
			Title:         gh.Ptr(githubv4.String(opts.title)),
			Body:          gh.Ptr(githubv4.String(newBody)),
			BaseRefName:   gh.Ptr(githubv4.String(opts.baseRefName)),
		})
		if err != nil {
			return nil, false, errors.WithStack(err)
		}
		return updatedPR, false, nil
	}
	pull, err := client.CreatePullRequest(ctx, githubv4.CreatePullRequestInput{
		RepositoryID: githubv4.ID(repoMeta.ID),
		BaseRefName:  githubv4.String(opts.baseRefName),
		HeadRefName:  githubv4.String(opts.headRefName),
		Title:        githubv4.String(opts.title),
		Body:         gh.Ptr(githubv4.String(AddPRMetadata(opts.body, opts.meta))),
		Draft:        gh.Ptr(githubv4.Boolean(opts.draft)),
	})
	if err != nil {
		return nil, false, errors.WithStack(err)
	}
	return pull, true, nil
}

type UpdatePullRequestResult struct {
	// True if the pull request information changed (e.g., a new pull request
	// was found or if the pull request changed state)
	Changed bool
	// The pull request object that was returned from GitHub
	Pull *gh.PullRequest
}

// UpdatePullRequestState fetches the latest pull request information from GitHub
// and writes the relevant branch metadata.
func UpdatePullRequestState(
	ctx context.Context,
	client *gh.Client,
	tx meta.WriteTx,
	branchName string,
) (*UpdatePullRequestResult, error) {
	repoMeta, ok := tx.Repository()
	if !ok {
		return nil, ErrRepoNotInitialized
	}
	branch, _ := tx.Branch(branchName)

	_, _ = fmt.Fprint(os.Stderr,
		"  - fetching latest pull request information for ", colors.UserInput(branchName),
		"\n",
	)
	page, err := client.GetPullRequests(ctx, gh.GetPullRequestsInput{
		Owner:       repoMeta.Owner,
		Repo:        repoMeta.Name,
		HeadRefName: branchName,
	})
	if err != nil {
		return nil, errors.WrapIf(err, "querying GitHub pull requests. Make sure GitHub token is set or refresh.\nSee: https://docs.aviator.co/aviator-cli#getting-started")
	}

	if len(page.PullRequests) == 0 {
		// branch has no pull request
		if branch.PullRequest != nil {
			// This should never happen?
			logrus.WithFields(logrus.Fields{
				"branch": branch.Name,
				"pull":   branch.PullRequest.Permalink,
			}).Error("GitHub reported no pull requests for branch but local metadata has pull request")
			return nil, errors.New(
				"GitHub reported no pull requests for branch but local metadata has pull request",
			)
		}

		return &UpdatePullRequestResult{false, nil}, nil
	}

	// The latest info for the pull request that we have stored in local metadata
	// (we can use this to check if the pull was closed/merged)
	var currentPull *gh.PullRequest
	// The current open pull request (if any)
	var openPull *gh.PullRequest
	for i := range page.PullRequests {
		pull := &page.PullRequests[i]
		if branch.PullRequest != nil && pull.ID == branch.PullRequest.ID {
			currentPull = pull
		}
		if pull.State != githubv4.PullRequestStateOpen {
			continue
		}
		// GH only allows one open pull for a given (head, base) pair, but
		// we only support one open pull per head branch (the workflow of
		// opening a pull from a head branch into multiple base branches is
		// rare). This probably isn't necessary but better to be defensive
		// here.
		if openPull != nil {
			return nil, errors.Errorf(
				"multiple open pull requests for branch %q (#%d into %q and #%d into %q)",
				branchName,
				openPull.Number, openPull.BaseRefName,
				pull.Number, pull.BaseRefName,
			)
		}
		openPull = pull
	}

	changed := false
	var oldId string
	if branch.PullRequest != nil {
		oldId = branch.PullRequest.ID
	}

	var newPull *gh.PullRequest
	if openPull != nil {
		if oldId != openPull.ID {
			_, _ = fmt.Fprint(os.Stderr,
				"  - found new pull request for ", colors.UserInput(branchName),
				": ", colors.UserInput(openPull.Permalink),
				"\n",
			)
			changed = true
		}
		branch.PullRequest = &meta.PullRequest{
			ID:        openPull.ID,
			Number:    openPull.Number,
			Permalink: openPull.Permalink,
			State:     openPull.State,
		}
		newPull = openPull
	} else {
		// openPull is nil so the PR should be merged or closed
		if currentPull != nil {
			branch.MergeCommit = currentPull.GetMergeCommit()
			branch.PullRequest = &meta.PullRequest{
				ID:        currentPull.ID,
				Number:    currentPull.Number,
				Permalink: currentPull.Permalink,
				State:     currentPull.State,
			}
		} else {
			// openPull and currentPull is nil
			if branch.PullRequest != nil {
				_, _ = fmt.Fprint(os.Stderr,
					"  - ", colors.Failure("ERROR:"),
					" pull request for ", colors.UserInput(branchName),
					" could not be found on GitHub: ", colors.UserInput(branch.PullRequest.Permalink),
					" (removing reference from local state)\n",
				)
				changed = true
			}
			branch.PullRequest = nil
		}
		newPull = currentPull
	}

	tx.SetBranch(branch)
	return &UpdatePullRequestResult{changed, newPull}, nil
}

type PRMetadata struct {
	Parent     string `json:"parent"`
	ParentHead string `json:"parentHead"`
	ParentPull int64  `json:"parentPull,omitempty"`
	Trunk      string `json:"trunk"`
}

const PRMetadataCommentStart = "<!-- av pr metadata"

const PRMetadataCommentHelpText = "This information is embedded by the av CLI when creating PRs to track the status of stacks when using Aviator. Please do not delete or edit this section of the PR.\n"
const PRMetadataCommentEnd = "-->"

// extractContent parses the given input and looks for the start and end
// strings. It returns the content between the start and end strings and the
// remaining input. If the start or end strings are not found, the content is
// empty and the input is returned as-is.
func extractContent(input string, start string, end string) (content string, output string) {
	startIndex := strings.Index(input, start)
	if startIndex == -1 {
		return "", input
	}
	contentIndex := startIndex + len(start)
	endIndex := strings.Index(input[contentIndex:], end)
	if endIndex == -1 {
		return "", input
	}

	content = strings.TrimSpace(input[contentIndex : contentIndex+endIndex])
	preContent := strings.TrimSpace(input[:startIndex])
	postContent := strings.TrimSpace(input[contentIndex+endIndex+len(end):])
	output = preContent
	if postContent != "" {
		output += "\n" + postContent
	}
	return
}

func ParsePRBody(input string) (body string, prMeta PRMetadata, retErr error) {
	metadata, body := extractContent(input, PRMetadataCommentStart, PRMetadataCommentEnd)
	metadataContent, _ := extractContent(metadata, "```", "```")
	if err := json.Unmarshal([]byte(metadataContent), &prMeta); err != nil {
		retErr = errors.WrapIff(err, "decoding PR metadata")
		return
	}

	return
}

func ReadPRMetadata(body string) (PRMetadata, error) {
	_, prMeta, err := ParsePRBody(body)
	return prMeta, err
}

func AddPRMetadata(body string, prMeta PRMetadata) string {
	body, _, err := ParsePRBody(body)
	if err != nil {
		// No existing metadata comment, so add one.
		logrus.WithError(err).Debug("could not parse PR metadata (assuming it doesn't exist)")
		body += "\n\n"
	}

	sb := strings.Builder{}
	sb.WriteString(body)

	sb.WriteString("\n\n")
	sb.WriteString(PRMetadataCommentStart)
	sb.WriteString("\n")
	sb.WriteString(PRMetadataCommentHelpText)
	sb.WriteString("```\n")

	// Note: Encoder.Encode implicitly adds a newline at the end of the JSON
	// which is important here so that the ``` below appears on its own line.
	if err := json.NewEncoder(&sb).Encode(prMeta); err != nil {
		// shouldn't ever happen since we're encoding a simple struct to a buffer
		panic(errors.WrapIff(err, "encoding PR metadata"))
	}
	sb.WriteString("```\n")
	sb.WriteString(PRMetadataCommentEnd)
	sb.WriteString("\n")

	return sb.String()
}
