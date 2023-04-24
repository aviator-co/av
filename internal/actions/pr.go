package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aviator-co/av/internal/editor"
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/aviator-co/av/internal/utils/templateutils"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/browser"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/fatih/color"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
)

type CreatePullRequestOpts struct {
	BranchName string
	Title      string
	Body       string
	//LabelNames      []string

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
func getPRMetadata(repo *git.Repo, branch meta.Branch, parent *meta.Branch) (PRMetadata, error) {
	trunk, err := meta.Trunk(repo, branch.Name)
	if err != nil {
		return PRMetadata{}, err
	}
	prMeta := PRMetadata{
		Parent:     branch.Parent.Name,
		ParentHead: branch.Parent.Head,
		Trunk:      trunk,
	}
	if parent == nil && branch.Parent.Name != "" {
		p, _ := meta.ReadBranch(repo, branch.Parent.Name)
		parent = &p
	}
	if parent != nil && parent.PullRequest != nil {
		prMeta.ParentPull = parent.PullRequest.Number
	}
	return prMeta, nil
}

func getExistingOpenPR(ctx context.Context, client *gh.Client, repoMeta meta.Repository, branchMeta meta.Branch, baseRefName string) (*gh.PullRequest, error) {
	if branchMeta.PullRequest != nil {
		pr, err := client.PullRequest(ctx, branchMeta.PullRequest.ID)
		if err != nil {
			return nil, errors.WrapIf(err, "querying existing pull request")
		}
		if pr.State != githubv4.PullRequestStateOpen {
			// This is already closed. Create a new one.
			return nil, nil
		}
		return pr, nil
	}
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
func CreatePullRequest(ctx context.Context, repo *git.Repo, client *gh.Client, opts CreatePullRequestOpts) (*CreatePullRequestResult, error) {
	if opts.BranchName == "" {
		logrus.Panicf("internal invariant error: CreatePullRequest called with empty branch name")
	}

	repoMeta, err := meta.ReadRepository(repo)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprint(os.Stderr,
		"Creating pull request for branch ", colors.UserInput(opts.BranchName), ":",
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
	} else {
		_, _ = fmt.Fprint(os.Stderr,
			"  - skipping push to GitHub",
			"\n",
		)
	}

	// figure this out based on whether or not we're on a stacked branch
	branchMeta, _ := meta.ReadBranch(repo, opts.BranchName)
	prBaseBranch := branchMeta.Parent.Name
	prCompareRef := prBaseBranch
	var parentMeta meta.Branch
	if !branchMeta.Parent.Trunk {
		// check if the base branch also has an associated PR
		var ok bool
		parentMeta, ok = meta.ReadBranch(repo, prBaseBranch)
		if !ok {
			return nil, errors.WrapIff(err, "failed to read branch metadata for %q", prBaseBranch)
		}
		if parentMeta.PullRequest == nil {
			// TODO:
			//     We should automagically create PRs for every branch in the stack
			return nil, errors.Errorf(
				"base branch %q does not have an associated pull request "+
					"(create one by checking out the branch and running `av pr create`)",
				prBaseBranch,
			)
		}
	} else {
		logrus.WithField("base", prBaseBranch).Debug("base branch is a trunk branch")
		prCompareRef = "origin/" + prBaseBranch
	}

	commitsList, err := repo.Git("rev-list", "--reverse", fmt.Sprintf("%s..HEAD", prCompareRef))
	if err != nil {
		return nil, errors.WrapIf(err, "failed to determine commits to include in PR")
	}
	if commitsList == "" {
		return nil, errors.Errorf("no commits between %q and %q", prCompareRef, opts.BranchName)
	}

	existingPR, err := getExistingOpenPR(ctx, client, repoMeta, branchMeta, prBaseBranch)
	if err != nil {
		return nil, err
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

		// We need to open an editor to ask the user. Try to populate with
		// reasonable defaults here.
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
			TmpFilePattern: "pr-*.md",
			CommentPrefix:  "%%",
		})
		if err != nil {
			return nil, errors.WrapIf(err, "failed to launch text editor")
		}
		opts.Title, opts.Body = stringutils.ParseSubjectBody(res)
		if opts.Title == "" {
			return nil, errors.New("aborting pull request due to empty message")
		}
	}

	prMeta, err := getPRMetadata(repo, branchMeta, &parentMeta)
	if err != nil {
		return nil, err
	}

	pull, didCreatePR, err := ensurePR(ctx, client, repoMeta, ensurePROpts{
		baseRefName: prBaseBranch,
		headRefName: opts.BranchName,
		title:       opts.Title,
		body:        opts.Body,
		meta:        prMeta,
		draft:       opts.Draft,
		existingPR:  existingPR,
	})
	if err != nil {
		return nil, errors.WrapIf(err, "failed to create PR")
	}

	branchMeta.PullRequest = &meta.PullRequest{
		Number:    pull.Number,
		ID:        pull.ID,
		Permalink: pull.Permalink,
	}
	// It's possible that a new PR is created with the same branch. Reset the MergeCommit.
	branchMeta.MergeCommit = ""
	if err := meta.WriteBranch(repo, branchMeta); err != nil {
		return nil, err
	}
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

	return &CreatePullRequestResult{didCreatePR, branchMeta, pull}, nil
}

type prBodyTemplateData struct {
	Branch  string
	Title   string
	Body    string
	Commits []git.CommitInfo
}

var templateFuncs = template.FuncMap{"trimSpace": strings.TrimSpace}

var prBodyTemplate = template.Must(template.New("prBody").Funcs(templateFuncs).Parse(`%% Creating pull request for branch '{{ .Branch }}'
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
`))

func readDefaultPullRequestTemplate(repo *git.Repo) string {
	tpl := filepath.Join(repo.Dir(), ".github", "PULL_REQUEST_TEMPLATE.md")
	data, err := os.ReadFile(tpl)
	if err != nil {
		return ""
	}
	return string(data)
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
func ensurePR(ctx context.Context, client *gh.Client, repoMeta meta.Repository, opts ensurePROpts) (*gh.PullRequest, bool, error) {
	if opts.existingPR != nil {
		newBody := AddPRMetadata(opts.body, opts.meta)
		updatedPR, err := client.UpdatePullRequest(ctx, githubv4.UpdatePullRequestInput{
			PullRequestID: opts.existingPR.ID,
			Title:         gh.Ptr(githubv4.String(opts.title)),
			Body:          gh.Ptr(githubv4.String(newBody)),
		})
		if err != nil {
			return nil, false, errors.WrapIf(err, "updating PR")
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
		return nil, false, errors.WrapIf(err, "opening pull request")
	}
	return pull, true, nil
}

type UpdatePullRequestResult struct {
	// True if the pull request information changed (e.g., a new pull request
	// was found or if the pull request changed state)
	Changed bool
	// The (updated) branch metadata.
	Branch meta.Branch
	// The pull request object that was returned from GitHub
	Pull *gh.PullRequest
}

// UpdatePullRequestState fetches the latest pull request information from GitHub
// and writes the relevant branch metadata.
func UpdatePullRequestState(ctx context.Context, repo *git.Repo, client *gh.Client, repoMeta meta.Repository, branchName string) (*UpdatePullRequestResult, error) {
	_, _ = fmt.Fprint(os.Stderr,
		"  - fetching latest pull request information for ", colors.UserInput(branchName),
		"\n",
	)

	branch, _ := meta.ReadBranch(repo, branchName)

	page, err := client.GetPullRequests(ctx, gh.GetPullRequestsInput{
		Owner:       repoMeta.Owner,
		Repo:        repoMeta.Name,
		HeadRefName: branchName,
	})
	if err != nil {
		return nil, errors.WrapIf(err, "querying GitHub pull requests")
	}

	if len(page.PullRequests) == 0 {
		// branch has no pull request
		if branch.PullRequest != nil {
			// This should never happen?
			logrus.WithFields(logrus.Fields{
				"branch": branch.Name,
				"pull":   branch.PullRequest.Permalink,
			}).Error("GitHub reported no pull requests for branch but local metadata has pull request")
			return nil, errors.New("GitHub reported no pull requests for branch but local metadata has pull request")
		}

		return &UpdatePullRequestResult{false, branch, nil}, nil
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

	// Write branch metadata regardless of changed to make sure it's in consistent state
	if err := meta.WriteBranch(repo, branch); err != nil {
		return nil, errors.WrapIf(err, "writing branch metadata")
	}

	return &UpdatePullRequestResult{changed, branch, newPull}, nil
}

type PRMetadata struct {
	Parent     string `json:"parent"`
	ParentHead string `json:"parentHead"`
	ParentPull int64  `json:"parentPull,omitempty"`
	Trunk      string `json:"trunk"`
}

const PRMetadataCommentStart = "<!-- av pr metadata\n"
const PRMetadataCommentHelpText = "This information is embedded by the av CLI when creating PRs to track the status of stacks when using Aviator. Please do not delete or edit this section of the PR.\n"
const PRMetadataCommentEnd = "-->\n"

func ParsePRMetadata(input string) (commentStart int, commentEnd int, prMeta PRMetadata, reterr error) {
	buf := bytes.NewBufferString(input)

	// Read until we find the "<!-- av pr metadata" line
	if err := readLineUntil(buf, PRMetadataCommentStart); err != nil {
		reterr = errors.WrapIff(err, "expecting %q", PRMetadataCommentStart)
		return
	}
	commentStart = len(input) - buf.Len() - len(PRMetadataCommentStart)

	// Read until we find the "```" line (which indicates that json starts
	// on the following line)
	if err := readLineUntil(buf, "```\n"); err != nil {
		reterr = errors.WrapIff(err, "expecting \"```\"")
		return
	}

	// We need to create a copy of the buffer here since json.Decoder may read
	// past the end of the JSON data (and we need to access that data below!)
	if err := json.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(&prMeta); err != nil {
		reterr = errors.WrapIff(err, "decoding PR metadata")
		return
	}

	// This will skip over any data lines (since those weren't consumed by buf,
	// only by the copy of buf).
	if err := readLineUntil(buf, "```\n"); err != nil {
		reterr = errors.WrapIff(err, "expecting closing \"```\"")
		return
	}
	if err := readLineUntil(buf, PRMetadataCommentEnd); err != nil {
		reterr = errors.WrapIff(err, "expecting %q", PRMetadataCommentEnd)
		return
	}
	commentEnd = len(input) - buf.Len()
	return
}

func ReadPRMetadata(body string) (PRMetadata, error) {
	_, _, prMeta, err := ParsePRMetadata(body)
	return prMeta, err
}

func AddPRMetadata(body string, prMeta PRMetadata) string {
	buf := bytes.NewBufferString(body)
	if commentStart, commentEnd, _, err := ParsePRMetadata(body); err != nil {
		// No existing metadata comment, so add one.
		logrus.WithError(err).Debug("could not parse PR metadata (assuming it doesn't exist)")
		buf.WriteString("\n\n")
	} else {
		buf.Truncate(commentStart)
		if commentEnd < len(body) {
			// The PR body doesn't end with the metadata comment. This probably
			// means that the PR was edited after it was created with the av CLI
			// (so we should preserve that text that comes after the comment).
			buf.WriteString(body[commentEnd:])
			// We also need newlines here to separate the metadata comment from
			// the text that comes before it.
			buf.WriteString("\n\n")
		}
	}

	buf.WriteString(PRMetadataCommentStart)
	buf.WriteString(PRMetadataCommentHelpText)
	buf.WriteString("```\n")
	// Note: Encoder.Encode implicitly adds a newline at the end of the JSON
	// which is important here so that the ``` below appears on its own line.
	if err := json.NewEncoder(buf).Encode(prMeta); err != nil {
		// shouldn't ever happen since we're encoding a simple struct to a buffer
		panic(errors.WrapIff(err, "encoding PR metadata"))
	}
	buf.WriteString("```\n")
	buf.WriteString(PRMetadataCommentEnd)
	return buf.String()
}

func readLineUntil(b *bytes.Buffer, line string) error {
	for {
		l, err := b.ReadString('\n')
		if err != nil {
			return err
		}
		if l == line {
			return nil
		}
	}
}
