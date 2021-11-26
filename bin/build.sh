#!/bin/bash -eu

source /build-common.sh

COMPILE_IN_DIRECTORY="cmd/tailscale-discovery"
BINARY_NAME="tailscale-discovery"

standardBuildProcess

packageLambdaFunction
