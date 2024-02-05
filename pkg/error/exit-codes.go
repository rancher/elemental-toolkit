/*
Copyright © 2022 - 2024 SUSE LLC

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

// Error reading the run config
const ReadingRunConfig = 31

// Error reading the install/upgrade flags
const ReadingInstallUpgradeFlags = 32

// Error reading the config for the command
const ReadingSpecConfig = 33

// Error mounting state partition
const MountStatePartition = 34

// Error mounting recovery partition
const MountRecoveryPartition = 35

// Error during before-upgrade hook
const HookBeforeUpgrade = 36

// Error during before-upgrade-chroot hook
const HookBeforeUpgradeChroot = 37

// Error during after-upgrade hook
const HookAfterUpgrade = 38

// Error during after-upgrade-chroot hook
const HookAfterUpgradeChroot = 39

// Error moving file
const MoveFile = 40

// Error occurred during cleanup
const Cleanup = 41

// Error occurred trying to reboot
const Reboot = 42

// Error occurred trying to shutdown
const PowerOff = 43

// Error occurred when labeling partition
const LabelImage = 44

// Error setting default grub entry
const SetDefaultGrubEntry = 45

// Error occurred during selinux relabeling
const SelinuxRelabel = 46

// Error invalid device specified
const InvalidTarget = 47

// Error deploying image to file
const DeployImage = 48

// Error installing GRUB
const InstallGrub = 49

// Error during before-install hook
const HookBeforeInstall = 50

// Error during after-install hook
const HookAfterInstall = 51

// Error during after-install-chroot hook
const HookAfterInstallChroot = 52

// Error during file download
const DownloadFile = 53

// Error mounting partitions
const MountPartitions = 54

// Error deactivating active devices
const DeactivatingDevices = 55

// Error during device partitioning
const PartitioningDevice = 56

// Device already contains an install
const AlreadyInstalled = 57

// Command requires root privileges
const RequiresRoot = 58

// Error occurred when unmounting partitions
const UnmountPartitions = 59

// Error occurred when formatting partitions
const FormatPartitions = 60

// Error during before-reset hook
const HookBeforeReset = 61

// Error during after-reset-chroot hook
const HookAfterResetChroot = 62

// Error during after-reset hook
const HookAfterReset = 63

// Unsupported flavor
const UnsupportedFlavor = 64

// Error encountered during cloud-init run-stage
const CloudInitRunStage = 65

// Error unpacking image
const UnpackImage = 66

// Error reading file
const ReadFile = 67

// No source was provided for the command
const NoSourceProvided = 68

// Error removing a file
const RemoveFile = 69

// Error calculating checksum
const CalculateChecksum = 70

// Error occurred when unmounting image
const UnmountImage = 71

// Error occurred during post-upgrade hook
const HookPostUpgrade = 72

// Error occurred during post-reset hook
const HookPostReset = 73

// Error occurred during post-install hook
const HookPostInstall = 74

// Error occurred while preparing the image root tree
const DeployImgTree = 75

// Error occurred while creating the OS filesystem image
const CreateImgFromTree = 76

// Error occurred while copying the filesystem image and setting new labels
const CopyFileImg = 77

// Error setting persistent GRUB variables
const SetGrubVariables = 78

// Error occured on before-disk hook
const HookBeforeDisk = 79

// Error occured on after-disk hook
const HookAfterDisk = 80

// Error occured on after-disk-chroot hook
const HookAfterDiskChroot = 81

// Error occured on after-disk hook
const HookPostDisk = 82

// Error occured checking configured size is bigger than minimum required size
const InvalidSize = 83

// Error occured initializing snapshotter
const SnapshotterInit = 84

// Error occured starting an snapshotter transaction
const SnapshotterStart = 85

// Error mounting EFI partition
const MountEFIPartition = 86

// Error mounting Persistent partition
const MountPersistentPartition = 86

// Unknown error
const Unknown int = 255
