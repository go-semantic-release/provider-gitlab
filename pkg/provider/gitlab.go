package provider

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/semrel"
	"github.com/xanzy/go-gitlab"
)

const TAG_PLACEHOLDER = "{version}"

var PVERSION = "dev"

type GitLabRepository struct {
	projectID string
	branch    string
	tagFormat string
	client    *gitlab.Client
}

func (repo *GitLabRepository) Init(config map[string]string) error {
	gitlabBaseUrl := config["gitlab_baseurl"]
	if gitlabBaseUrl == "" {
		gitlabBaseUrl = os.Getenv("CI_SERVER_URL")
	}

	token := config["token"]
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}
	if token == "" {
		return errors.New("gitlab token missing")
	}

	branch := config["gitlab_branch"]
	if branch == "" {
		branch = os.Getenv("CI_COMMIT_BRANCH")
	}

	projectID := config["gitlab_projectid"]
	if projectID == "" {
		projectID = os.Getenv("CI_PROJECT_ID")
	}
	if projectID == "" {
		return fmt.Errorf("gitlab_projectid is required")
	}

	tagFormat := config["tag_format"]
	if tagFormat == "" {
		tagFormat = "v" + TAG_PLACEHOLDER
	}

	repo.projectID = projectID
	repo.branch = branch
	repo.tagFormat = tagFormat

	var (
		client *gitlab.Client
		err    error
	)

	if gitlabBaseUrl != "" {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(gitlabBaseUrl))
	} else {
		client, err = gitlab.NewClient(token)
	}

	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	repo.client = client
	return nil
}

func (repo *GitLabRepository) GetInfo() (*provider.RepositoryInfo, error) {
	project, _, err := repo.client.Projects.GetProject(repo.projectID, nil)

	if err != nil {
		return nil, err
	}
	return &provider.RepositoryInfo{
		Owner:         "",
		Repo:          "",
		DefaultBranch: project.DefaultBranch,
		Private:       project.Visibility == gitlab.PrivateVisibility,
	}, nil
}

func (repo *GitLabRepository) GetCommits(fromSha, toSha string) ([]*semrel.RawCommit, error) {
	opts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		// No Matter the order ofr fromSha and toSha gitlab always returns commits in reverse chronological order
		RefName: gitlab.String(fmt.Sprintf("%s...%s", fromSha, toSha)),
	}

	allCommits := make([]*semrel.RawCommit, 0)

	for {
		commits, resp, err := repo.client.Commits.ListCommits(repo.projectID, opts)

		if err != nil {
			return nil, err
		}

		for _, commit := range commits {
			allCommits = append(allCommits, &semrel.RawCommit{
				SHA:        commit.ID,
				RawMessage: commit.Message,
			})
		}

		// We cannot always rely on the total pages header
		// https://gitlab.com/gitlab-org/gitlab-foss/-/merge_requests/23931
		// if resp.CurrentPage >= resp.TotalPages {
		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

func (repo *GitLabRepository) GetReleases(rawRe string) ([]*semrel.Release, error) {
	re := regexp.MustCompile(rawRe)
	allReleases := make([]*semrel.Release, 0)

	opts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	for {
		tags, resp, err := repo.client.Tags.ListTags(repo.projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, tag := range tags {
			if rawRe != "" && !re.MatchString(tag.Name) {
				continue
			}

			version, err := semver.NewVersion(tag.Name)
			if err != nil {
				continue
			}

			allReleases = append(allReleases, &semrel.Release{
				SHA:     tag.Commit.ID,
				Version: version.String(),
			})
		}

		if resp.CurrentPage >= resp.TotalPages {
			break
		}

		opts.Page = resp.NextPage
	}

	return allReleases, nil
}

func (repo *GitLabRepository) CreateRelease(release *provider.CreateReleaseConfig) error {
	tag := strings.ReplaceAll(repo.tagFormat, TAG_PLACEHOLDER, release.NewVersion)

	// Gitlab does not have any notion of pre-releases
	_, _, err := repo.client.Releases.CreateRelease(repo.projectID, &gitlab.CreateReleaseOptions{
		TagName: &tag,
		Ref:     &release.SHA,
		// TODO: this may been to be wrapped in ```
		Description: &release.Changelog,
	})

	return err
}

func (repo *GitLabRepository) Name() string {
	return "GitLab"
}

func (repo *GitLabRepository) Version() string {
	return PVERSION
}
