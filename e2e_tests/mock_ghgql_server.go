package e2e_tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	// These are ugly, but this is easy way to tell which query is being used.
	prQuery = "query($after:String$baseRefName:String$first:Int!$headRefName:String$owner:String!$repo:String!$states:[PullRequestState!]){repository(owner: $owner, name: $repo){pullRequests(states: $states, headRefName: $headRefName, baseRefName: $baseRefName, first: $first, after: $after){nodes{id,number,headRefName,baseRefName,isDraft,permalink,state,title,body,mergeCommit{oid},timelineItems(last: 10, itemTypes: [CLOSED_EVENT, MERGED_EVENT]){nodes{... on ClosedEvent{closer{... on Commit{oid}}},... on MergedEvent{commit{oid}}}}},pageInfo{endCursor,hasNextPage,hasPreviousPage,startCursor}}}}"
)

func RunMockGitHubServer(t *testing.T) *mockGitHubServer {
	s := &mockGitHubServer{t: t, Server: nil}
	s.Server = httptest.NewServer(s)
	return s
}

type mockGitHubServer struct {
	t *testing.T

	pulls []mockPR

	*httptest.Server
}

type mockPR struct {
	ID          string
	Number      int
	HeadRefName string
	BaseRefName string
	IsDraft     bool
	State       string
	Title       string
	Body        string

	MergeCommitOID  string
	ClosedCommitOID string
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type graphqlResponse struct {
	Data map[string]interface{} `json:"data"`
}

func (s *mockGitHubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req graphqlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.t.Logf("Failed to decode request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if req.Query == prQuery {
		s.t.Logf("Received PR query: %s", req.Variables)
		if err := json.NewEncoder(w).Encode(s.handlePRQuery(req)); err != nil {
			s.t.Logf("Failed to encode response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	s.t.Logf("Received unexpected query: %s", req.Query)
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *mockGitHubServer) handlePRQuery(req graphqlRequest) graphqlResponse {
	headRefName := req.Variables["headRefName"].(string)
	var prs []interface{}
	for _, pr := range s.pulls {
		if pr.HeadRefName != headRefName {
			continue
		}
		gqlpr := map[string]interface{}{
			"id":          pr.ID,
			"number":      pr.Number,
			"headRefName": pr.HeadRefName,
			"baseRefName": pr.BaseRefName,
			"isDraft":     pr.IsDraft,
			"permalink":   fmt.Sprintf("https://github.invalid/mock/mock/pulls/%d", pr.Number),
			"state":       pr.State,
			"title":       pr.Title,
			"body":        pr.Body,
		}
		if pr.MergeCommitOID != "" {
			gqlpr["mergeCommit"] = map[string]string{"oid": pr.MergeCommitOID}
		}
		if pr.ClosedCommitOID != "" {
			gqlpr["timelineItems"] = map[string]interface{}{
				"nodes": []interface{}{
					map[string]interface{}{
						"__typename": "ClosedEvent",
						"closer": map[string]interface{}{
							"__typename": "Commit",
							"oid":        pr.ClosedCommitOID,
						},
					},
				},
			}
		}
		prs = append(prs, gqlpr)
	}
	return graphqlResponse{
		Data: map[string]interface{}{
			"repository": map[string]interface{}{
				"pullRequests": map[string]interface{}{
					"nodes": prs,
				},
			},
		},
	}
}
