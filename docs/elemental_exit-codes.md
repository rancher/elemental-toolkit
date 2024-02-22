# Exit codes for elemental CLI


| Exit code | Meaning |
| :----: | :---- |
| 10 | Error closing a file|
| 11 | Error running a command|
| 12 | Error copying data|
| 13 | Error copying a file|
| 14 | Wrong cosign flags used in cmd|
| 15 | Error creating a dir|
| 16 | Error creating a file|
| 17 | Error creating a temporal dir|
| 18 | Error dumping the source|
| 19 | Error creating a gzip writer|
| 20 | Error trying to identify the source|
| 21 | Error calling mkfs|
| 22 | There is not packages for the given architecture|
| 24 | Error opening a file|
| 25 | Output file already exists|
| 26 | Error reading the build config|
| 27 | Error reading the build-disk config|
| 28 | Error running stat on a file|
| 29 | Error creating a tar archive|
| 30 | Error truncating a file|
| 31 | Error reading the run config|
| 32 | Error reading the install/upgrade flags|
| 33 | Error reading the config for the command|
| 34 | Error mounting state partition|
| 35 | Error mounting recovery partition|
| 36 | Error during before-upgrade hook|
| 37 | Error during before-upgrade-chroot hook|
| 38 | Error during after-upgrade hook|
| 39 | Error during after-upgrade-chroot hook|
| 40 | Error moving file|
| 41 | Error occurred during cleanup|
| 42 | Error occurred trying to reboot|
| 43 | Error occurred trying to shutdown|
| 44 | Error occurred when labeling partition|
| 45 | Error setting default grub entry|
| 46 | Error occurred during selinux relabeling|
| 47 | Error invalid device specified|
| 48 | Error deploying image to file|
| 49 | Error installing GRUB|
| 50 | Error during before-install hook|
| 51 | Error during after-install hook|
| 52 | Error during after-install-chroot hook|
| 53 | Error during file download|
| 54 | Error mounting partitions|
| 55 | Error deactivating active devices|
| 56 | Error during device partitioning|
| 57 | Device already contains an install|
| 58 | Command requires root privileges|
| 59 | Error occurred when unmounting partitions|
| 60 | Error occurred when formatting partitions|
| 61 | Error during before-reset hook|
| 62 | Error during after-reset-chroot hook|
| 63 | Error during after-reset hook|
| 64 | Unsupported flavor|
| 65 | Error encountered during cloud-init run-stage|
| 66 | Error unpacking image|
| 67 | Error reading file|
| 68 | No source was provided for the command|
| 69 | Error removing a file|
| 70 | Error calculating checksum|
| 71 | Error occurred when unmounting image|
| 72 | Error occurred during post-upgrade hook|
| 73 | Error occurred during post-reset hook|
| 74 | Error occurred during post-install hook|
| 75 | Error occurred while preparing the image root tree|
| 76 | Error occurred while creating the OS filesystem image|
| 77 | Error occurred while copying the filesystem image and setting new labels|
| 78 | Error setting persistent GRUB variables|
| 79 | Error occured on before-disk hook|
| 80 | Error occured on after-disk hook|
| 81 | Error occured on after-disk-chroot hook|
| 82 | Error occured on after-disk hook|
| 83 | Error occured checking configured size is bigger than minimum required size|
| 84 | Error occured initializing snapshotter|
| 85 | Error occured starting an snapshotter transaction|
| 86 | Error mounting EFI partition|
| 87 | Error mounting Persistent partition|
| 255 | Unknown error|
