# :fox_face: provider-gitlab

[![CI](https://github.com/go-semantic-release/provider-gitlab/workflows/CI/badge.svg?branch=master)](https://github.com/go-semantic-release/provider-gitlab/actions?query=workflow%3ACI+branch%3Amaster)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-semantic-release/provider-gitlab)](https://goreportcard.com/report/github.com/go-semantic-release/provider-gitlab)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/go-semantic-release/provider-gitlab)](https://pkg.go.dev/github.com/go-semantic-release/provider-gitlab)

The GitLab provider for [go-semantic-release](https://github.com/go-semantic-release/semantic-release).

### Configuration

|        Name        |        Default Value         |                               Description                               |
| :----------------: | :--------------------------: | :---------------------------------------------------------------------: |
|   gitlab_baseurl   |        CI_SERVER_URL         |    The base URL of the GitLab instance, including protocol and port.    |
|   gitlab_branch    |       CI_COMMIT_BRANCH       |                         The commit branch name.                         |
|  gitlab_projectid  |        CI_PROJECT_ID         |                     The ID of the current project.                      |
|       token        | GITLAB_TOKEN -> CI_JOB_TOKEN |           A token to authenticate with certain API endpoints.           |
|      git_path      |        CI_PROJECT_DIR        | The full path the repository is cloned to, and where the job runs from. |
| strip_v_tag_prefix |            false             |             Boolean to remove the `v` prefix from the tag.              |
|     log_order      |             dfs              |                   The log order traversal algorithm.                    |

### Log Order Options

> Requirements:
> `CI_JOB_TOKEN` must be used. [GitLab CI/CD job token](https://docs.gitlab.com/ee/ci/jobs/ci_job_token.html)
> `GIT_STRATEGY` must be set to `clone` or `fetch`. [Git strategy doc](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#git-strategy)

log_order=dfs (Default) - Ordering by depth-first search in pre-order

log_order=dfs_post - Ordering by depth-first search in post-order (useful to traverse
history in chronological order)

log_order=bfs - Ordering by breadth-first search

log_order=ctime - Ordering by committer time (more compatible with `git log`)

## Licence

The [MIT License (MIT)](http://opensource.org/licenses/MIT)

Copyright © 2020 [Christoph Witzko](https://twitter.com/christophwitzko)
