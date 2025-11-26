#!/usr/bin/env python3

# Copyright 2023 The Kubernetes Authors.
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

import re
import sys
import json

keys_to_skip = [
    "controller",
    "controllerGroup",
    "controllerKind",
    "reconcileID",
    "HetznerCluster",
    "Cluster",
    "namespace",
    "name",
    "Machine",
    "stack",
    "stacktrace",
    "logger",
]

# Only show the name. Not a nested struct.
# in: 'HCloudMachine': {'name': 'myname', 'namespace': '...'}
# out:'HCloudMachine': 'myname'
keys_to_name = [
    "HCloudMachine",
    "HCloudMachineTemplate",
    "HCloudRemediation",
    "HCloudRemediationTemplate",
    "HetznerBareMetalHost",
    "HetznerBareMetalMachine",
    "HetznerBareMetalMachineTemplate",
    "HetznerBareMetalRemediation",
    "HetznerBareMetalRemediationTemplate",
    "HetznerCluster",
    "HetznerClusterTemplate",
]

rows_to_skip_regex = [
    r"maxprocs: Leaving GOMAXPROCS=\d+: CPU quota undefined",
    r"^Random Seed: \d+",
]

rows_to_trigger_test_filter = ["Running Suite:"]

# List of fixed-strings.
rows_to_skip = [
    '"Starting workers" controller/controller',
    '"starting manager"',
    '"Starting metrics server"',
    '"starting server"',
    '"Reconciling finished"',
    '"Creating cluster scope"',
    '"Starting reconciling cluster"',
    '"Completed function"',
    '"Adding request."',
    '"Serving metrics server"',
    '"Starting webhook server"',
    '"Serving webhook server"',
    '"approved csr"',
    '"Registering webhook"',
    '"Stopping and waiting for',
    '"All workers finished"',
    '"Shutdown signal received, waiting for all workers to finish"',
    "'statusCode': 200, 'method': 'GET', 'url': 'https://robot-ws.your-server.de",
]

rows_to_skip_for_tests = [
    "attempting to acquire leader lease caph-system/hetzner.cluster.x-k8s.io.",
    "Update to resource only changes insignificant fields",
    '"Wait completed, proceeding to shutdown the manager"',
    '"starting control plane" logger="controller-runtime.test-env"',
    '"installing CRDs" logger="controller-runtime.test-env"',
    '"reading CRDs from path"',
    '"read CRDs from file"',
    '"installing CRD"',
    '"adding API in waitlist" logger="controller-runtime.test-env"',
    '"installing webhooks" logger="controller-runtime.test-env"',
    '"installing mutating webhook" logger="controller-runtime.test-env"',
    '"installing validating webhook" logger="controller-runtime.test-env"',
    'cluster.x-k8s.io\\": prefer a domain-qualified finalizer',
    '"Update to resource changes significant fields, will enqueue event"',
    '"Wait for update being in local cache"',
    '"Created load balancer"',
    '"Created network with opts',
    '"Starting workers" controller="',
    '"controller-runtime.certwatcher"',
    "controller-runtime.webhook",
    "certwatcher/certwatcher",
    "Registering a validating webhook",
    "Registering a mutating webhook",
    "Starting EventSource",
    "Starting Controller",
    "Wait completed, proceeding to shutdown the manager",
    "unable to decode an event from the watch stream: context canceled",
    "client rate limiter Wait returned an error: context canceled",
    'os-ssh-secret": context canceled',
    "http: TLS handshake error from 127.0.0.1:",
    '"Cluster infrastructure did not become ready, blocking further processing"',
    'predicate="IgnoreInsignificant',
    '"HetznerCluster is not available yet"',
    '"Unable to write event (broadcaster is shut down)"',
    '"Unable to write event (may retry after sleeping)"',
]


def main():

    if len(sys.argv) == 1 or sys.argv[1] in ["-h", "--help"]:
        print(
            """%s [file|-]
    filter the logs of caph-controller-manager.
    Used for debugging.
    """
            % sys.argv[0]
        )
        sys.exit(1)

    if sys.argv[1] == "-":
        fd = sys.stdin
    else:
        fd = open(sys.argv[1])
    read_logs(fd)


def read_logs(fd):
    for line in fd:
        try:
            handle_line(line)
        except BrokenPipeError:
            return


ansi_pattern = re.compile(r"\x1B\[[0-9;]*[A-Za-z]")

filtering_test_data = False


def write_line(line):
    ascii_line = ansi_pattern.sub("", line)
    for r in rows_to_trigger_test_filter:
        if r in ascii_line:
            global filtering_test_data
            filtering_test_data = True
            rows_to_skip.extend(rows_to_skip_for_tests)

    for r in rows_to_skip:
        if r in ascii_line:
            return
    for r in rows_to_skip_regex:
        if re.search(r, ascii_line):
            return
    sys.stdout.write(line)


def handle_line(line):
    # Extract leading dots (•) and print them on a separate line
    leading_dots = ""
    while line.startswith("•"):
        leading_dots += "•"
        line = line[1:]
    if leading_dots:
        print(leading_dots)
    line = line.strip()  # Remove whitespace after extracting dots
    if not line.startswith("{"):
        write_line(line)
        return
    try:
        data = json.loads(line)
    except json.decoder.JSONDecodeError as err:
        write_line("\n")
        write_line(str(err))
        write_line(line)
        write_line("\n")
        return
    for key in keys_to_skip:
        data.pop(key, None)

    # in: 'HCloudMachine': {'name': 'myname', 'namespace': '...'}
    # out:'HCloudMachine': 'myname'
    for key in keys_to_name:
        value = data.get(key)
        if not value:
            continue
        name = value.get("name")
        if name:
            data[key] = name

    t = data.pop("time", "")
    t = re.sub(r"^.*T(.+)*\..+$", r"\1", t)  # '2023-04-17T12:12:53.423Z

    # skip too long entries
    for key, value in list(data.items()):
        if not isinstance(value, str):
            continue
        if len(value) > 1_000:
            data[key] = value[:1_000] + "...cut..."

    level = data.pop("level", "").ljust(5)
    file = data.pop("file", "")
    message = data.pop("message", "")

    if data:
        values = []
        for key, value in data.items():
            value = str(value)
            if any(c.isspace() for c in value):
                value = f"<{value}>"
            values.append(f"{key}={value}")
        data = " ".join(values)
    else:
        data = ""

    new_line = f'{t} {level} "{message}" {file} {data}\n'
    write_line(new_line)


if __name__ == "__main__":
    main()
