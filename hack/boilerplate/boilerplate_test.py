#!/usr/bin/env python

# Copyright 2016 The Kubernetes Authors.
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
import boilerplate
import unittest
import io
import os
import sys
import tempfile
import re
from contextlib import contextmanager

base_dir = os.getcwd()


class DefaultArgs(object):
    def __init__(self):
        self.filenames = []
        self.rootdir = "."
        self.boilerplate_dir = base_dir
        self.verbose = True
        self.ensure = False


class TestBoilerplate(unittest.TestCase):
    """
    Note: run this test from the inside the boilerplate directory.

    $ python -m unittest boilerplate_test
    """

    def setUp(self):
        os.chdir(base_dir)
        boilerplate.args = DefaultArgs()

    def test_boilerplate(self):
        os.chdir("testdata/default/")

        # capture stdout
        old_stdout = sys.stdout
        sys.stdout = io.StringIO()

        ret = boilerplate.main()

        output = sorted(sys.stdout.getvalue().split())

        sys.stdout = old_stdout

        self.assertCountEqual(
            output, ['././fail.go', '././fail.py', '././fail.sh'])

    def test_read_config(self):
        config_file = "./testdata/with_config/.boilerplate.json"
        config = boilerplate.read_config_file(config_file)
        self.assertCountEqual(config.get('dirs_to_skip'), [
                         'dir_to_skip', 'dont_want_this', 'not_interested', '.'])
        self.assertCountEqual(config.get('not_generated_files_to_skip'), [
                         'alice skips a file', 'bob skips another file'])

    def test_read_nonexistent_config(self):
        config_file = '/nonexistent'
        config = boilerplate.read_config_file(config_file)
        self.assertCountEqual(config['dirs_to_skip'],
                         boilerplate.default_skipped_dirs)
        self.assertCountEqual(config['not_generated_files_to_skip'],
                         boilerplate.default_skipped_not_generated)

    def test_read_malformed_config(self):
        config_file = './testdata/with_config/.boilerplate.bad.json'
        with self.assertRaises(Exception):
            boilerplate.read_config_file(config_file)

    def test_read_config_called_with_correct_path(self):
        boilerplate.args.rootdir = "/tmp/some/path"
        with function_mocker('read_config_file', boilerplate, return_value={}) as mock_args:
            boilerplate.main()
            self.assertEqual(len(mock_args), 1)
            self.assertEqual(
                mock_args[0][0], "/tmp/some/path/.boilerplate.json")

    def test_get_files_with_skipping_dirs(self):
        refs = boilerplate.get_refs()
        skip_dirs = ['.']
        files = boilerplate.get_files(refs, skip_dirs)

        self.assertEqual(files, [])

    def test_get_files_with_skipping_not_generated_files(self):
        refs = boilerplate.get_refs()
        regexes = boilerplate.get_regexs()
        files_to_skip = ['boilerplate.py']
        filename = 'boilerplate.py'

        passes = boilerplate.file_passes(
            filename, refs, regexes, files_to_skip)

        self.assertEqual(passes, True)

    def test_ignore_when_no_valid_boilerplate_template(self):
        with tempfile.NamedTemporaryFile() as temp_file_to_check:
            passes = boilerplate.file_passes(
                temp_file_to_check.name, boilerplate.get_refs(), boilerplate.get_regexs(), [])
            self.assertEqual(passes, True)

    def test_add_boilerplate_to_file(self):
        with tmp_copy("./testdata/default/fail.sh", suffix='.sh') as tmp_file_name:
            boilerplate.ensure_boilerplate_file(
                tmp_file_name, boilerplate.get_refs(), boilerplate.get_regexs(), []
            )

            passes = boilerplate.file_passes(
                tmp_file_name, boilerplate.get_refs(), boilerplate.get_regexs(), [])
            self.assertEqual(passes, True)

            with open(tmp_file_name) as x:
                first_line = x.read().splitlines()[0]
                self.assertEqual(first_line, '#!/usr/bin/env bash')

    def test_replace_specials(self):
        extension = "sh"
        regexs = boilerplate.get_regexs()

        original_content = "\n".join([
            "#!/usr/bin/env bash",
            "",
            "something something",
            "#!/usr/bin/env bash",
        ])
        expected_content = "\n".join([
            "something something",
            "#!/usr/bin/env bash",
        ])
        expected_match = "\n".join([
            "#!/usr/bin/env bash",
            "\n",
        ])

        actual_content, actual_match = boilerplate.replace_specials(
            original_content, extension, regexs
        )

        self.assertEqual(actual_content, expected_content)
        self.assertEqual(actual_match, expected_match)

    def test_ensure_command_line_flag(self):
        os.chdir("./testdata/default/")
        boilerplate.args.ensure = True

        with function_mocker('ensure_boilerplate_file', boilerplate) as mock_args:
            boilerplate.main()
            changed_files = list(map(lambda x: x[0], mock_args))

            self.assertCountEqual(changed_files, [
                "././fail.sh",
                "././fail.py",
                "././fail.go",
            ])


@contextmanager
def tmp_copy(file_org, suffix=None):
    file_copy_fd, file_copy = tempfile.mkstemp(suffix)

    with open(file_org) as org:
        os.write(file_copy_fd, bytes(org.read(),'utf-8'))
        os.close(file_copy_fd)

    yield file_copy

    os.unlink(file_copy)


@contextmanager
def function_mocker(function_name, original_holder, return_value=None):
    # save original function implementation
    original_implementation = getattr(original_holder, function_name)

    # keep track of the args
    mock_call_args = []

    # mock the function
    def the_mock(*args):
        mock_call_args.append(args)
        if return_value is not None:
            return return_value

    # use the mock in place of the original implementation
    setattr(original_holder, function_name, the_mock)

    # run
    yield mock_call_args

    # reset the original implementation
    setattr(original_holder, function_name, original_implementation)
