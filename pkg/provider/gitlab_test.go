package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/semrel"
	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"
)

var validTags = map[string]bool{
	"v2.0.0": true,
	"2.0.0":  true,
}

func TestNewGitlabRepository(t *testing.T) {
	require := require.New(t)

	var repo *GitLabRepository
	repo = &GitLabRepository{}
	err := repo.Init(map[string]string{})
	require.EqualError(err, "gitlab token missing")

	repo = &GitLabRepository{}
	err = repo.Init(map[string]string{
		"token":            "token",
		"gitlab_projectid": "1",
	})
	require.NoError(err)

	repo = &GitLabRepository{}
	err = repo.Init(map[string]string{
		"gitlab_baseurl":     "https://mygitlab.com",
		"token":              "token",
		"gitlab_projectid":   "1",
		"strip_v_tag_prefix": "true",
	})
	require.NoError(err)
	require.Equal("https://mygitlab.com/api/v4/", repo.client.BaseURL().String(), "invalid custom instance initialization")
}

func createGitlabCommit(sha, message string) *gitlab.Commit {
	commitDate := time.Now()
	return &gitlab.Commit{
		ID:             sha,
		Message:        message,
		AuthorName:     "author",
		AuthorEmail:    "author@gitlab.com",
		AuthoredDate:   &commitDate,
		CommitterName:  "committer",
		CommitterEmail: "committer@gitlab.com",
		CommittedDate:  &commitDate,
	}
}

var testSHA = "deadbeef"

func createGitlabTag(name string) *gitlab.Tag {
	return &gitlab.Tag{Name: name, Commit: &gitlab.Commit{
		ID: testSHA,
	}}
}

var (
	gitlabProjectID     = 12324322
	gitlabDefaultBranch = "master"
	gitlabProjects      = gitlab.Project{DefaultBranch: gitlabDefaultBranch, Visibility: gitlab.PrivateVisibility, ID: gitlabProjectID}
	gitlabCommits       = []*gitlab.Commit{
		createGitlabCommit("abcd", "feat(app): new feature"),
		createGitlabCommit("dcba", "Fix: bug"),
		createGitlabCommit("cdba", "Initial commit"),
		createGitlabCommit("efcd", "chore: break\nBREAKING CHANGE: breaks everything"),
	}
	gitlabTags = []*gitlab.Tag{
		createGitlabTag("test-tag"),
		createGitlabTag("v1.0.0"),
		createGitlabTag("v2.0.0"),
		createGitlabTag("v2.1.0-beta"),
		createGitlabTag("v3.0.0-beta.2"),
		createGitlabTag("v3.0.0-beta.1"),
		createGitlabTag("2020.04.19"),
	}
)

//nolint:errcheck
//gocyclo:ignore
func GitlabHandler(w http.ResponseWriter, r *http.Request) {
	// Rate Limit headers
	if r.Method == http.MethodGet && r.URL.Path == "/api/v4/" {
		json.NewEncoder(w).Encode(struct{}{})
		return
	}

	if r.Header.Get("PRIVATE-TOKEN") == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/api/v4/projects/%d", gitlabProjectID) {
		json.NewEncoder(w).Encode(gitlabProjects)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/api/v4/projects/%d/repository/commits", gitlabProjectID) {
		json.NewEncoder(w).Encode(gitlabCommits)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/api/v4/projects/%d/repository/tags", gitlabProjectID) {
		json.NewEncoder(w).Encode(gitlabTags)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/api/v4/projects/%d/releases", gitlabProjectID) {
		var data map[string]string
		json.NewDecoder(r.Body).Decode(&data)
		r.Body.Close()

		if _, ok := validTags[data["tag_name"]]; !ok {
			http.Error(w, "invalid tag name", http.StatusBadRequest)
			return
		}

		fmt.Fprint(w, "{}")
		return
	}

	http.Error(w, "invalid route", http.StatusNotImplemented)
}

func getNewGitlabTestRepo(t *testing.T) (*GitLabRepository, *httptest.Server) {
	ts := httptest.NewServer(http.HandlerFunc(GitlabHandler))
	repo := &GitLabRepository{}
	err := repo.Init(map[string]string{
		"gitlab_baseurl":   ts.URL,
		"token":            "gitlab-examples-ci",
		"gitlab_branch":    "",
		"gitlab_projectid": strconv.Itoa(gitlabProjectID),
	})
	require.NoError(t, err)

	return repo, ts
}

func TestGitlabGetInfo(t *testing.T) {
	repo, ts := getNewGitlabTestRepo(t)
	defer ts.Close()
	repoInfo, err := repo.GetInfo()
	require.NoError(t, err)
	require.Equal(t, gitlabDefaultBranch, repoInfo.DefaultBranch)
	require.True(t, repoInfo.Private)
}

func TestGitlabGetCommits(t *testing.T) {
	repo, ts := getNewGitlabTestRepo(t)
	defer ts.Close()
	commits, err := repo.GetCommits("", "")
	require.NoError(t, err)
	require.Len(t, commits, 4)

	for i, c := range commits {
		require.Equal(t, c.SHA, gitlabCommits[i].ID)
		require.Equal(t, c.RawMessage, gitlabCommits[i].Message)
		require.Equal(t, c.Annotations["author_name"], gitlabCommits[i].AuthorName)
		require.Equal(t, c.Annotations["author_email"], gitlabCommits[i].AuthorEmail)
		require.Equal(t, c.Annotations["committer_name"], gitlabCommits[i].CommitterName)
		require.Equal(t, c.Annotations["committer_email"], gitlabCommits[i].CommitterEmail)
		require.Equal(t, c.Annotations["author_date"], gitlabCommits[i].AuthoredDate.Format(time.RFC3339))
		require.Equal(t, c.Annotations["committer_date"], gitlabCommits[i].CommittedDate.Format(time.RFC3339))
	}
}

func TestGitlabGetReleases(t *testing.T) {
	repo, ts := getNewGitlabTestRepo(t)
	defer ts.Close()

	testCases := []struct {
		vrange          string
		re              string
		expectedSHA     string
		expectedVersion string
	}{
		{"", "", testSHA, "2020.4.19"},
		{"", "^v[0-9]*", testSHA, "2.0.0"},
		{"2-beta", "", testSHA, "2.1.0-beta"},
		{"3-beta", "", testSHA, "3.0.0-beta.2"},
		{"4-beta", "", testSHA, "4.0.0-beta"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("VersionRange: %s, RE: %s", tc.vrange, tc.re), func(t *testing.T) {
			releases, err := repo.GetReleases(tc.re)
			require.NoError(t, err)
			release, err := semrel.GetLatestReleaseFromReleases(releases, tc.vrange)
			require.NoError(t, err)
			require.Equal(t, tc.expectedSHA, release.SHA)
			require.Equal(t, tc.expectedVersion, release.Version)
		})
	}
}

func TestGitlabCreateRelease(t *testing.T) {
	repo, ts := getNewGitlabTestRepo(t)
	defer ts.Close()
	err := repo.CreateRelease(&provider.CreateReleaseConfig{NewVersion: "2.0.0", SHA: testSHA})
	require.NoError(t, err)
}

func TestGitlabStripVTagRelease(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(GitlabHandler))
	defer ts.Close()

	repo := &GitLabRepository{}
	err := repo.Init(map[string]string{
		"gitlab_baseurl":     ts.URL,
		"token":              "gitlab-examples-ci",
		"gitlab_branch":      "",
		"gitlab_projectid":   strconv.Itoa(gitlabProjectID),
		"strip_v_tag_prefix": "true",
	})

	require.NoError(t, err)

	err = repo.CreateRelease(&provider.CreateReleaseConfig{NewVersion: "2.0.0", SHA: testSHA})
	require.NoError(t, err)
}
