package actions

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	// If true, create a PR even if we think one already exists
	Force bool
}

type CreatePullRequestResult struct {
	// True if the pull request was created
	Created bool
	// The (updated) branch metadata.
	Branch meta.Branch
	// The pull request object that was returned from GitHub
	Pull *gh.PullRequest
}

// CreatePullRequest creates a pull request on GitHub for the current branch, if
// one doesn't already exist.
func CreatePullRequest(ctx context.Context, repo *git.Repo, client *gh.Client, opts CreatePullRequestOpts) (*CreatePullRequestResult, error) {
	repoMeta, err := meta.ReadRepository(repo)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprint(os.Stderr,
		"Creating pull request for branch ", colors.UserInput(opts.BranchName), ":",
		"\n",
	)
	if !opts.NoPush {
		pushFlags := []string{"push"}

		// Check if the upstream is set. If not, we set it during push.
		// TODO: Should we store this somewhere? I think currently things will
		//       break if the upstream name is not the same name as the local
		upstream, err := repo.RevParse(&git.RevParse{
			SymbolicFullName: true,
			Rev:              "HEAD@{u}",
		})
		if err != nil {
			// Set the upstream branch
			upstream = "origin/" + opts.BranchName
			pushFlags = append(pushFlags, "--set-upstream", "origin", opts.BranchName)
		} else {
			upstream = strings.TrimPrefix(upstream, "refs/remotes/")
		}
		logrus.WithField("upstream", upstream).Debug("pushing latest changes")

		_, _ = fmt.Fprint(os.Stderr,
			"  - pushing branch to GitHub (", color.CyanString("%s", upstream), ")",
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
	if !branchMeta.Parent.Trunk {
		// check if the base branch also has an associated PR
		baseMeta, ok := meta.ReadBranch(repo, prBaseBranch)
		if !ok {
			return nil, errors.WrapIff(err, "failed to read branch metadata for %q", prBaseBranch)
		}
		if baseMeta.PullRequest == nil {
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
	}

	commitsList, err := repo.Git("rev-list", "--reverse", fmt.Sprintf("%s..HEAD", prBaseBranch))
	if err != nil {
		return nil, errors.WrapIf(err, "failed to determine commits to include in PR")
	}
	if commitsList == "" {
		return nil, errors.Errorf("no commits between %q and %q", prBaseBranch, opts.BranchName)
	}
	commits := strings.Split(commitsList, "\n")
	firstCommit, err := repo.CommitInfo(git.CommitInfoOpts{Rev: commits[0]})
	if err != nil {
		return nil, errors.WrapIf(err, "failed to read first commit")
	}

	if opts.Title == "" {
		opts.Title = firstCommit.Subject
	}
	if opts.Body == "" {
		// Commits bodies are often in a fixed-width format (e.g., 80 characters
		// wide) so most newlines should actually be spaces. Unfortunately,
		// GitHub renders newlines in the commit body (which goes against the
		// Markdown spec, but whatever) which makes it a little bit weird to
		// directly include in the PR body. We could convert singe newlines to
		// spaces to make GitHub happy, but that's not trivial without using a
		// full-on Markdown parser (e.g., "foo\nbar" should become "foo bar" but
		// "foo\n* bar" should stay the same).
		// Maybe someday we can invest in making this better, but it's not the
		// end of the world.
		opts.Body = firstCommit.Body
	}

	pull, didCreatePR, err := getOrCreatePR(ctx, client, repoMeta, getOrCreatePROpts{
		baseRefName: prBaseBranch,
		headRefName: opts.BranchName,
		title:       opts.Title,
		body:        opts.Body,
		draft:       opts.Draft,
	})
	if err != nil {
		return nil, errors.WrapIf(err, "failed to create PR")
	}

	branchMeta.PullRequest = &meta.PullRequest{
		Number:    pull.Number,
		ID:        pull.ID,
		Permalink: pull.Permalink,
	}
	if err := meta.WriteBranch(repo, branchMeta); err != nil {
		return nil, err
	}

	// add the avbeta-stackedprs label to enable Aviator server-side stacked
	// PRs functionality
	if err := client.AddIssueLabels(ctx, gh.AddIssueLabelInput{
		Owner:      repoMeta.Owner,
		Repo:       repoMeta.Name,
		Number:     pull.Number,
		LabelNames: []string{"avbeta-stackedprs"},
	}); err != nil {
		return nil, errors.WrapIf(err, "adding avbeta-stackedprs label")
	}

	var action string
	if didCreatePR {
		action = "created"
	} else {
		action = "fetched existing"
	}
	_, _ = fmt.Fprint(os.Stderr,
		"  - ", action, " pull request for branch ", colors.UserInput(opts.BranchName),
		" (into branch ", colors.UserInput(prBaseBranch), "): ",
		colors.UserInput(pull.Permalink),
		"\n",
	)

	if config.Av.PullRequest.OpenBrowser {
		if err := browser.Open(pull.Permalink); err != nil {
			fmt.Fprint(os.Stderr,
				"  - couldn't open browser ",
				colors.UserInput(err),
				" for pull request link ",
				colors.UserInput(pull.Permalink),
			)
		}
	}

	return &CreatePullRequestResult{didCreatePR, branchMeta, pull}, nil
}

type getOrCreatePROpts struct {
	baseRefName string
	headRefName string
	title       string
	body        string
	draft       bool
}

// getOrCreatePR returns the pull request for the given input, creating a new
// pull request if one doesn't exist. It returns the pull request, a boolean
// indicating whether or not the pull request was created, and an error if one
// occurred.
func getOrCreatePR(ctx context.Context, client *gh.Client, repoMeta meta.Repository, opts getOrCreatePROpts) (*gh.PullRequest, bool, error) {
	existing, err := client.GetPullRequests(ctx, gh.GetPullRequestsInput{
		Owner:       repoMeta.Owner,
		Repo:        repoMeta.Name,
		HeadRefName: opts.headRefName,
		States:      []githubv4.PullRequestState{githubv4.PullRequestStateOpen},
	})
	if err != nil {
		return nil, false, errors.WrapIf(err, "querying existing pull requests")
	}
	if len(existing.PullRequests) > 0 {
		return &existing.PullRequests[0], false, nil
	}

	pull, err := client.CreatePullRequest(ctx, githubv4.CreatePullRequestInput{
		RepositoryID: githubv4.ID(repoMeta.ID),
		BaseRefName:  githubv4.String(opts.baseRefName),
		HeadRefName:  githubv4.String(opts.headRefName),
		Title:        githubv4.String(opts.title),
		Body:         gh.Ptr(githubv4.String(opts.body)),
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
		// openPull is nil
		if currentPull != nil {
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
