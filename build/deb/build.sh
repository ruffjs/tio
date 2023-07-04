#!/bin/bash

set -ex

NAEM=tio
BUILD_DIR=./build/deb
DIST_DIR=./dist
WEB_DIR=./web

# Set env
if [ -z "$GOARCH" ] 
then
	export GOOS=linux
	export GOARCH=amd64
fi

function build_web() {
	cd $WEB_DIR
	yarn
	yarn build
	touch dist/.gitkeep
	cd ..
}

function build_deb() {
	rm -rf ${DIST_DIR}

  deb=${DIST_DIR}/deb
  mkdir -p ${DIST_DIR}
	cp -r ${BUILD_DIR}/pack-deb $deb

  version=$(date '+%Y%m%d%H%M%S')

	# Set version
	if [[ "$OSTYPE" == "linux-gnu"* ]]; then
		sed -i "s/{version}/${version}/g" ${deb}/DEBIAN/control
	elif [[ "$OSTYPE" == "darwin"* ]]; then
		sed -i "" "s/{version}/${version}/g" ${deb}/DEBIAN/control
	else
		echo "unknown os"
		exit 1
	fi

  CGO_ENABLED=1 go build \
	  -ldflags "-X main.Version=${version} -X main.GitCommit=`git rev-parse HEAD`" \
	  -o ${DIST_DIR}/${NAEM} \
		cmd/tio/main.go


	rm -fr ${deb}/opt/tio/${NAEM}
	rm -fr ${deb}/opt/tio/.gitkeep

	cp ${DIST_DIR}/${NAEM} ${deb}/opt/tio/
	cp config.yaml ${deb}/opt/tio/

	dpkg-deb --root-owner-group --build ${deb} ${DIST_DIR}/${NAEM}_${GOOS}_${GOARCH}.deb
}

build_web
build_deb
