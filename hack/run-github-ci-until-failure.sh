#!/usr/bin/env bash

# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Run a Workflow in GitHub Actions until it fails.
# Goal: Find flaky tests, which only fail in CI.

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

# See `gh workflow list` for the workflow names.
WORKFLOW="Test Code"

while true; do
    # Start the CI job for the current commit
    echo "Triggering CI workflow..."
    gh workflow run "$WORKFLOW"

    # Get the most recent workflow run ID
    echo "Waiting for the workflow to appear..."
    sleep 5
    RUN_ID=$(gh run list --limit 1 --json databaseId --jq '.[0].databaseId')

    if [ -z "$RUN_ID" ]; then
        echo "Failed to retrieve workflow run ID."
        exit 1
    fi

    # Monitor the workflow until it finishes
    while true; do
        echo "Checking workflow status..."
        STATUS=$(gh run view "$RUN_ID" --json status --jq '.status')
        echo "... status: $STATUS"
        if [ "$STATUS" == "in_progress" ] || [ "$STATUS" == "queued" ] || [ "$STATUS" == "pending" ]; then
            echo "... waiting ..."
            sleep 10
            continue
        fi
        break
    done

    # Check the conclusion of the workflow
    CONCLUSION=$(gh run view "$RUN_ID" --json conclusion --jq '.conclusion')
    if [ "$CONCLUSION" != "success" ]; then
        echo "Workflow failed"
        gh run view "$RUN_ID"
        exit 1
    fi
    echo "Workflow succeeded. Retrying..."
done
