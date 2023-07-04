#!/bin/bash

set -ex

WEB_DIR=./web


function build_web() {
	cd $WEB_DIR
	yarn
	yarn build
	touch dist/.gitkeep
	cd ..
}

function build_docker() {
  build_web

  version=$(date '+%Y%m%d%H%M%S')
  repo="tio"
  gitCommit=`git rev-parse HEAD`
  docker build --build-arg version=${version} --build-arg gitCommit=${gitCommit} \
    --platform linux/amd64 \
    -t $repo:$version \
    -f build/docker/Dockerfile .
}

build_docker
