#!/usr/bin/env bash

set -euo pipefail

pluginDir=".semrel/$(go env GOOS)_$(go env GOARCH)/provider-gitlab/0.0.0-dev/"
[[ ! -d "$pluginDir" ]] && {
  echo "creating $pluginDir"
  mkdir -p "$pluginDir"
}

go build -o "$pluginDir/provider-gitlab" ./cmd/provider-gitlab
