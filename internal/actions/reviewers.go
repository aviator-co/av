package actions

import (
	"context"
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
)

// AddPullRequestReviewers adds the given reviewers to the given pull request.
// It accepts a list of reviewers, which can be either GitHub user logins or
// team names in the format `@organization/team`.
func AddPullRequestReviewers(
	ctx context.Context,
	client *gh.Client,
	prID githubv4.ID,
	reviewers []string,
) error {
	_, _ = fmt.Fprint(os.Stderr,
		"  - adding ", colors.UserInput(len(reviewers)), " reviewer(s) to pull request\n",
	)

	// We need to map the given reviewers to GitHub node IDs.
	var reviewerIDs []githubv4.ID
	var teamIDs []githubv4.ID
	for _, reviewer := range reviewers {
		if ok, org, team := isTeamName(reviewer); ok {
			team, err := client.OrganizationTeam(ctx, org, team)
			if err != nil {
				return err
			}
			teamIDs = append(teamIDs, team.ID)
		} else {
			user, err := client.User(ctx, reviewer)
			if err != nil {
				return err
			}
			reviewerIDs = append(reviewerIDs, user.ID)
		}
	}

	if _, err := client.RequestReviews(ctx, githubv4.RequestReviewsInput{
		PullRequestID: prID,
		UserIDs:       &reviewerIDs,
		TeamIDs:       &teamIDs,
		Union:         gh.Ptr[githubv4.Boolean](true),
	}); err != nil {
		return errors.WrapIf(err, "requesting reviews")
	}

	return nil
}

func isTeamName(s string) (bool, string, string) {
	before, after, found := strings.Cut(s, "/")
	if !found || before == "" || after == "" {
		return false, "", ""
	}

	// It's common to specify team names as `@aviator-co/engineering`. We want
	// just the organization name (`aviator-co`) and team slug (`engineering`)
	// here, so strip the leading `@` if it exists.
	// This shouldn't cause any ambiguity since GitHub user login's can't
	// contain a slash character.
	before = strings.TrimPrefix(before, "@")
	return true, before, after
}
