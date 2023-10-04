package provider

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	git_provider "github.com/go-semantic-release/provider-git/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/semrel"
	"github.com/xanzy/go-gitlab"
)

var PVERSION = "dev"

type GitLabRepository struct {
	projectID       string
	branch          string
	stripVTagPrefix bool
	client          *gitlab.Client
	gitRepo         *git_provider.Repository
}

func (repo *GitLabRepository) Init(config map[string]string) error {
	gitlabBaseURL := config["gitlab_baseurl"]
	if gitlabBaseURL == "" {
		gitlabBaseURL = os.Getenv("CI_SERVER_URL")
	}

	token := config["token"]
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
		repo.gitRepo = nil

		if token == "" {
			if os.Getenv("CI_JOB_TOKEN") != "" {
				repo.gitRepo = &git_provider.Repository{}
				config := map[string]string{
					"default_branch": os.Getenv("CI_DEFAULT_BRANCH"),
					"tagger_name":    os.Getenv("GITLAB_USER_NAME"),
					"tagger_email":   os.Getenv("GITLAB_USER_EMAIL"),
					"remote_name":    "origin",
					"git_path":       os.Getenv("CI_PROJECT_DIR"),
				}
				repo.gitRepo.Init(config)
			} else {
				return errors.New("gitlab token missing")
			}
		}
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

	var err error
	stripVTagPrefix := config["strip_v_tag_prefix"]
	repo.stripVTagPrefix, err = strconv.ParseBool(stripVTagPrefix)

	if stripVTagPrefix != "" && err != nil {
		return fmt.Errorf("failed to set property strip_v_tag_prefix: %w", err)
	}

	repo.projectID = projectID
	repo.branch = branch

	gitlabClientOpts := []gitlab.ClientOptionFunc{}

	if gitlabBaseURL != "" {
		gitlabClientOpts = append(gitlabClientOpts, gitlab.WithBaseURL(gitlabBaseURL))
	}

	var client *gitlab.Client
	if token != "" {
		client, err = gitlab.NewClient(token, gitlabClientOpts...)
	} else {
		client, err = gitlab.NewJobClient(os.Getenv("CI_JOB_TOKEN"), gitlabClientOpts...)
	}

	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	repo.client = client
	return nil
}

func (repo *GitLabRepository) GetInfo() (*provider.RepositoryInfo, error) {
	if repo.gitRepo != nil {
		return &provider.RepositoryInfo{
			Owner:         os.Getenv("CI_PROJECT_NAMESPACE"),
			Repo:          os.Getenv("CI_PROJECT_NAME"),
			DefaultBranch: os.Getenv("CI_DEFAULT_BRANCH"),
			Private:       os.Getenv("CI_PROJECT_VISIBILITY") == "private",
		}, nil
	}

	project, _, err := repo.client.Projects.GetProject(repo.projectID, nil)
	if err != nil {
		return nil, err
	}
	namespace, repoName, _ := strings.Cut(project.PathWithNamespace, "/")
	return &provider.RepositoryInfo{
		Owner:         namespace,
		Repo:          repoName,
		DefaultBranch: project.DefaultBranch,
		Private:       project.Visibility == gitlab.PrivateVisibility,
	}, nil
}

func (repo *GitLabRepository) GetCommits(fromSha, toSha string) ([]*semrel.RawCommit, error) {
	/*if repo.gitRepo != nil {
		return repo.gitRepo.GetCommits(fromSha, toSha)
	}*/

	var refName *string
	if fromSha == "" {
		refName = gitlab.String(toSha)
	} else {
		// No Matter the order ofr fromSha and toSha gitlab always returns commits in reverse chronological order
		refName = gitlab.String(fmt.Sprintf("%s...%s", fromSha, toSha))
	}

	opts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		RefName: refName,
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
				Annotations: map[string]string{
					"author_name":     commit.AuthorName,
					"author_email":    commit.AuthorEmail,
					"author_date":     commit.AuthoredDate.Format(time.RFC3339),
					"committer_name":  commit.CommitterName,
					"committer_email": commit.CommitterEmail,
					"committer_date":  commit.CommittedDate.Format(time.RFC3339),
				},
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
	/*if repo.gitRepo != nil {
		return repo.gitRepo.GetReleases(rawRe)
	}*/

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
	prefix := "v"
	if repo.stripVTagPrefix {
		prefix = ""
	}

	tag := prefix + release.NewVersion

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
