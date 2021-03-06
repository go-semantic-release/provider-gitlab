package main

import (
	gitlabProvider "github.com/go-semantic-release/provider-gitlab/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/plugin"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		Provider: func() provider.Provider {
			return &gitlabProvider.GitLabRepository{}
		},
	})
}
