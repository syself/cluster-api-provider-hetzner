## Verify & Ensure boilerplate

- `verify-boilerplate.sh`:  
   Verifies that the boilerplate for various formats (go files, Makefile, etc.)
   is included in each file.
- `ensure-boilerplate.sh`:  
   Ensure that various formats (see above) have the boilerplate included.

The scripts assume the root of the repo to be one level up of the directory
the scripts are in.

If this is not the case, you can configure the root of the reop by either
setting `REPO_ROOT` or by calling the scripts with `--root-dir=<root>`.

You can put a config file into the root of your repo named `boilerplate.json`.
The config can look something like this:
```json
{
  "dirs_to_skip" : [
    "vendor",
    "tools/contrib"
  ],
  "not_generated_files_to_skip" : [
    "some/file",
    "some/other/file.something"
  ]
}
```
Currently supported settings are
- `dirs_to_skip`  
  A list of directories which is excluded when checking or adding the headers
- `not_generated_files_to_skip`  
  A list of all the files contain 'DO NOT EDIT', but are not generated

All other settings will be ignored.

### Tests

To run the test, cd into the boilerplate directory and run `python -m unittest boilerplate_test`.