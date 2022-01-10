#!/usr/bin/env python

# Copyright 2015 The Kubernetes Authors.
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

from __future__ import print_function

import argparse
import datetime
import difflib
import glob
import os
import re
import sys
import json

parser = argparse.ArgumentParser()
parser.add_argument(
    "filenames",
    help="list of files to check, all files if unspecified",
    nargs='*')

# Rootdir defaults to the directory **above** the repo-infra dir.
rootdir = os.path.dirname(__file__) + "/../../"
rootdir = os.path.abspath(rootdir)
parser.add_argument(
    "--rootdir", default=rootdir, help="root directory to examine")

default_boilerplate_dir = os.path.join(rootdir, "repo-infra/verify/boilerplate")
parser.add_argument(
    "--boilerplate-dir", default=default_boilerplate_dir)

parser.add_argument(
    "-v", "--verbose",
    help="give verbose output regarding why a file does not pass",
    action="store_true")

parser.add_argument(
    "--ensure",
    help="ensure all files which should have appropriate licence headers have them prepended",
    action="store_true")

args = parser.parse_args()

verbose_out = sys.stderr if args.verbose else open("/dev/null", "w")

default_skipped_dirs = ['Godeps', '.git', 'vendor', 'third_party', '_gopath', '_output']

# list all the files that contain 'DO NOT EDIT', but are not generated
default_skipped_not_generated = []


def get_refs():
    refs = {}

    for path in glob.glob(os.path.join(args.boilerplate_dir, "boilerplate.*.txt")):
        extension = os.path.basename(path).split(".")[1]

        ref_file = open(path, 'r')
        ref = ref_file.read().splitlines()
        ref_file.close()
        refs[extension] = ref

    return refs


def is_generated_file(filename, data, regexs, files_to_skip):
    for d in files_to_skip:
        if d in filename:
            return False

    p = regexs["generated"]
    return p.search(data)


def match_and_delete(content, re):
    match = re.search(content)
    if match is None:
        return content, None
    return re.sub("", content, 1), match.group()


def replace_specials(content, extension, regexs):
    # remove build tags from the top of Go files
    if extension == "go" or extension == "generatego":
        re = regexs["go_build_constraints"]
        return match_and_delete(content, re)

    # remove shebang from the top of shell files
    if extension == "sh":
        re = regexs["shebang"]
        return match_and_delete(content, re)

    return content, None


def file_passes(filename, refs, regexs, not_generated_files_to_skip):
    try:
        f = open(filename, 'r')
    except Exception as exc:
        print("Unable to open %s: %s" % (filename, exc), file=verbose_out)
        return False

    data = f.read()
    f.close()

    ref, extension, generated = analyze_file(
        filename, data, refs, regexs, not_generated_files_to_skip)

    return file_content_passes(data, filename, ref, extension, generated, regexs)


def file_content_passes(data, filename, ref, extension, generated, regexs):
    if ref is None:
        return True

    data, _ = replace_specials(data, extension, regexs)

    data = data.splitlines()

    # if our test file is smaller than the reference it surely fails!
    if len(ref) > len(data):
        print('File %s smaller than reference (%d < %d)' %
              (filename, len(data), len(ref)),
              file=verbose_out)
        return False

    # trim our file to the same number of lines as the reference file
    data = data[:len(ref)]

    p = regexs["year"]
    for d in data:
        if p.search(d):
            if generated:
                print('File %s has the YEAR field, but it should not be in generated file' % filename, file=verbose_out)
            else:
                print('File %s has the YEAR field, but missing the year of date' % filename, file=verbose_out)
            return False

    if not generated:
        # Replace all occurrences of the regex "2014|2015|2016|2017|2018" with "YEAR"
        p = regexs["date"]
        for i, d in enumerate(data):
            (data[i], found) = p.subn('YEAR', d)
            if found != 0:
                break

    # if we don't match the reference at this point, fail
    if ref != data:
        print("Header in %s does not match reference, diff:" % filename, file=verbose_out)
        if args.verbose:
            print(file=verbose_out)
            for line in difflib.unified_diff(ref, data, 'reference', filename, lineterm=''):
                print(line, file=verbose_out)
            print(file=verbose_out)
        return False

    return True


def file_extension(filename):
    return os.path.splitext(filename)[1].split(".")[-1].lower()


def read_config_file(conf_path):
    try:
        with open(conf_path) as json_data_file:
            return json.load(json_data_file)
    except ValueError:
        raise
    except:
        return {'dirs_to_skip': default_skipped_dirs, 'not_generated_files_to_skip': default_skipped_not_generated}


def normalize_files(files, dirs_to_skip):
    newfiles = []
    for pathname in files:
        if any(x in pathname for x in dirs_to_skip):
            continue
        newfiles.append(pathname)
    for i, pathname in enumerate(newfiles):
        if not os.path.isabs(pathname):
            newfiles[i] = os.path.join(args.rootdir, pathname)
    return newfiles


def get_files(extensions, dirs_to_skip):
    files = []
    if len(args.filenames) > 0:
        files = args.filenames
    else:
        for root, dirs, walkfiles in os.walk(args.rootdir):
            # don't visit certain dirs. This is just a performance improvement
            # as we would prune these later in normalize_files(). But doing it
            # cuts down the amount of filesystem walking we do and cuts down
            # the size of the file list
            for d in dirs_to_skip:
                if d in dirs:
                    dirs.remove(d)

            for name in walkfiles:
                pathname = os.path.join(root, name)
                files.append(pathname)

    files = normalize_files(files, dirs_to_skip)
    outfiles = []
    for pathname in files:
        basename = os.path.basename(pathname)
        extension = file_extension(pathname)
        if extension in extensions or basename in extensions:
            outfiles.append(pathname)
    return outfiles


def analyze_file(file_name, file_content, refs, regexs, not_generated_files_to_skip):
    # determine if the file is automatically generated
    generated = is_generated_file(
        file_name, file_content, regexs, not_generated_files_to_skip)

    base_name = os.path.basename(file_name)
    if generated:
        extension = "generatego"
    else:
        extension = file_extension(file_name)

    if extension != "":
        ref = refs[extension]
    else:
        ref = refs.get(base_name, None)

    return ref, extension, generated


def ensure_boilerplate_file(file_name, refs, regexs, not_generated_files_to_skip):
    with open(file_name, mode='r+') as f:
        file_content = f.read()

        ref, extension, generated = analyze_file(
            file_name, file_content, refs, regexs, not_generated_files_to_skip)

        # licence header
        licence_header = os.linesep.join(ref)

        # content without shebang and such
        content_without_specials, special_header = replace_specials(
            file_content, extension, regexs)

        # new content, to be writen to the file
        new_content = ''

        # shebang and such
        if special_header is not None:
            new_content += special_header

        # licence header
        current_year = str(datetime.datetime.now().year)
        year_replacer = regexs['year']
        new_content += year_replacer.sub(current_year, licence_header, 1)

        # actual content
        new_content += os.linesep + content_without_specials

        f.seek(0)
        f.write(new_content)


def get_dates():
    years = datetime.datetime.now().year
    return '(%s)' % '|'.join((str(year) for year in range(2014, years+1)))


def get_regexs():
    regexs = {}
    # Search for "YEAR" which exists in the boilerplate, but shouldn't in the real thing
    regexs["year"] = re.compile('YEAR')
    # get_dates return 2014, 2015, 2016, 2017, or 2018 until the current year as a regex like: "(2014|2015|2016|2017|2018)";
    # company holder names can be anything
    regexs["date"] = re.compile(get_dates())
    # strip // +build \n\n build constraints
    regexs["go_build_constraints"] = re.compile(
        r"^(// \+build.*\n)+\n", re.MULTILINE)
    # strip #!.* from shell scripts
    regexs["shebang"] = re.compile(r"^(#!.*\n)\n*", re.MULTILINE)
    # Search for generated files
    regexs["generated"] = re.compile('DO NOT EDIT')
    return regexs


def main():
    config_file_path = os.path.join(args.rootdir, ".boilerplate.json")
    config = read_config_file(config_file_path)

    regexs = get_regexs()
    refs = get_refs()
    filenames = get_files(refs.keys(), config.get('dirs_to_skip'))
    not_generated_files_to_skip = config.get('not_generated_files_to_skip', [])

    for filename in filenames:
        if not file_passes(filename, refs, regexs, not_generated_files_to_skip):
            if args.ensure:
                print("adding boilerplate header to %s" % filename)
                ensure_boilerplate_file(
                    filename, refs, regexs, not_generated_files_to_skip)
            else:
                print(filename, file=sys.stdout)

    return 0


if __name__ == "__main__":
    sys.exit(main())
