#!/bin/bash

set -x

case "$1" in
    # unit test
    "all" | "")
        go test ./...
        go test -tags=integration ./integration_tests  
        ;;
    "unit"|"")
        go test ./...
        ;;
    # integration test
    "it")
        go test -tags=integration ./integration_tests           
        ;;
esac