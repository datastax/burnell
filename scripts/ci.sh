#!/bin/bash

#
# Run the CI flow and build the binary
# Prerequisite -
# 1. Go runtime
#

# absolute directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

BASE_PKG_DIR="github.com/datastax/burnell/src/"
ALL_PKGS=""

cd $DIR/../src
# test lint, vet, and build as basic build steps in CI
echo run golint
golint ./...
echo run go vet
go vet ./...

echo run go build
mkdir -p ${DIR}/../bin
rm -f ${DIR}/../bin/burnell
GIT_COMMIT=$(git rev-list -1 HEAD)
go build -o ${DIR}/../bin/burnell -ldflags "-X main.gitCommit=$GIT_COMMIT"

cd $DIR/../src/logserver
go build -o ${DIR}/../bin/logcollector