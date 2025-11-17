#!/bin/bash
# Yarn wrapper script for easier command execution
# Usage: ./yarn.sh <command> [args...]
export NODE_OPTIONS="--no-node-snapshot"
exec node .yarn/releases/yarn-4.4.1.cjs "$@"
