/*
Copyright © 2022 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// provides a custom error interface and exit codes to use on the elemental-cli
package error

//
// Provided exit codes for elemental-cli

// To make it easy to generate them you have to respect the structure:
//
// comment that explains the error
// const NamedConstant = ERRORCODE
//
// This way you can later run `make build_docs` to generate the elemental_exit-codes.md in the docs dir automatically
// And they will be generated into a Markdown list of EXITCODE -> COMMENT

// Error closing a file
const CloseFile = 10

// Error running a command
const CommandRun = 11

// Error copying data
const CopyData = 12

// Error copying a file
const CopyFile = 13

// Wrong cosign flags used in cmd
const CosignWrongFlags = 14

// Error creating a dir
const CreateDir = 15

// Error creating a file
const CreateFile = 16

// Error creating a temporal dir
const CreateTempDir = 17

// Error dumping the source
const DumpSource = 18

// Error creating a gzip writer
const GzipWriter = 19

// Error trying to identify the source
const IdentifySource = 20

// Error calling mkfs
const MKFSCall = 21

// There is not packages for the given architecture
const NoPackagesForArch = 22

// No luet repositories configured
const NoReposConfigured = 23

// Error opening a file
const OpenFile = 24

// Output file already exists
const OutFileExists = 25

// Error reading the build config
const ReadingBuildConfig = 26

// Error reading the build-disk config
const ReadingBuildDiskConfig = 27

// Error running stat on a file
const StatFile = 28

// Error creating a tar archive
const TarHeader = 29

// Error truncating a file
const TruncateFile = 30

// Unknown error
const Unknown int = 255
