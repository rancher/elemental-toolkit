# Tips and tricks for development

Some helpful tips and docs for development

## Enable video capture on virtualbox (packer, tests)

When building the images with packer or running the tests with virtualbox, there is a possibility of enabling video capture of the machine output for easy troubleshooting.
This is helpful in situations where we cannot access the machine directly like in CI tests.

This is disabled by default but can be enabled so the output is captured. The CI is prepared to save the resulting artifacts if this is enabled.

For enabling it in tests you just need to set the env var `ENABLE_VIDEO_CAPTURE` to anything (true, on, whatever you like) and a capture.webm will be created and upload as an artifact in the CI

For packer you need to set the `PKR_VAR_enable_video_capture` var to `on` as packer is not as flexible as vagrant. The default for this var is `off` thus disabling the capture.

After enabling one of those vars, and only if the job fails, you will get the capture.webm file as an artifact in the CI.


# Test setups

There is currently 3 different test setups that we use to verify cOS

 - Installer tests from ISO (With 2 variations, bios and efi)
 - Raw image tests
 - Tests from packer boxes (With 2 variations, x86_64 and arm64 architecture)

The main difference in the tests are the source from which we boot, as they cover different use cases.

## Tests from ISO

In the case of the installer tests, we boot from the ISO, simulating how and end user would install cOS into a baremetal machine.

The test setup is done via the `make/Makefile.test` with `create_vm_from_iso_bios` or `create_vm_from_iso_efi` targets.
The test run is done via ginkgo, test suites are under the `test/installer` directory.

The full test workflow for this is as follows:

For efi:

- *(Makefile)* Create a vbox machine
- *(Makefile)* Set efi
- *(Makefile)* Don't set the boot order. Vbox efi does not allow to set the boot order so it will always boot from dvd if it's inserted.
- *(Ginkgo)* Run test suite
- *(Ginkgo)* Force unmount the dvd. This is mandatory as otherwise we cannot boot from disk and test that the installation was correct.
- *(Ginkgo)* Reboot. Now because we removed the dvd, it will boot from disk.
- *(Ginkgo)* Tests will check to see if whatever was run in the test was correctly (i.e. layout, partitions, config files, etc...)
- *(Ginkgo)* After each test, we store the location of the ISO and mount the cOS iso again so the new test will boot from dvd. Wipe the disk partition table as well.


If running this locally, there is an extra target in the `make/Makefile.test` to clean up the existing machine so it doesn't leave anything around. 
Use `make clean_vm_from_iso` to clean up the vm and its artifacts (Note that this will also remove the serial_port1.log file which contains the serial output) 


## Raw image tests

Raw image tests are tests run from the raw image created by `elemental build-disk` which creates a raw image with the recovery partition only.
This raw image is the base to create the different cloud images currently (AWS, GCE, AZURE)

The test setup is done via the `make/Makefile.test` with `create_vm_from_raw_image` target.
The test run is done via ginkgo, test suites are under the `test/recovery-raw-disk` directory.

The full test workflow for this is as follows:

- *(Makefile)* Tranform the raw image disk into a VDI disk
- *(Makefile)* Create user-data iso (using `packer/aws/aws.yaml` as teh source of cloud-data) so the disk is partitioned
- *(Makefile)* Set efi firmware (no bios test currently)
- *(Makefile)* Create the vbox VM
- *(Ginkgo)* Run test suite (currently only cos-deploy)

Currently, the raw disk test have a very big shortcoming and that is that becuase we are booting from the converted raw disk, once a test is run the disk has changed from its original status, so further tests are not possible as we would have a modified disk thus invalidating any results.
Hopefully in the future we use the snapshopt capabilities of vbox to take a snapshot on test suite start and restoring that snapshot after each test, so all the test can have the same base.


If running this locally, there is an extra target in the `make/Makefile.test` to clean up the existing machine, so it doesn't leave anything around.
Use `make clean_raw_disk_test` to clean up the vm and its artifacts (Note that this will also remove the serial_port1.log file which contains the serial output) 


## Tests from packer boxes

These tests are run from the packer boxes generated as part of the packer target and are the simplest ones, as we can use cos-reset to restore the boxes back to the start.


The test setup is done via the `make/Makefile.test` with `prepare-test` target.
The test run is done via ginkgo, test suites are under the `test/` directory.
Test machines are brougth up with vagrant using `tests/Vagrantfile` as the vagrant file.

The full test workflow for this is as follows:

- *(Makefile)* Create the vbox/qemu VM with vagrant
- *(Ginkgo)* Run test suite
- *(Ginkgo)* cos-reset after each test


Currently, the Vagrantfile contains 2 different vms, one for x86_64 (cos) and another one for arm64 (cos-arm64)
As virtualbox does not support arm64 architectures, we rely on qemu+libvirt to run the arm64 tests


If running this locally, there is an extra target in the `make/Makefile.test` to clean up the existing machine, so it doesn't leave anything around.
Use `make test-clean` to clean up the vm and its artifacts

NOTE: Currently on qemu, the `test-clean` target is unable to really remove the packer box from libvirt. This is a shortcoming of vagrant-libvirt. 
To fully remove the box you would need to manually do so with virsh (i.e. `$ virsh vol-delete cos_vagrant_box_image_0_box.img --pool default`)


## Auto tags

Currently there is a weekly job at `.github/workflows/autorelease.yaml` that runs each week and compares the current version of the system/cos package to the latest tag available in the repo.
If there are different, the job auto tags the current commit (main) with that version from system/cos and pushes it to github. That results in the release pipeline auto triggering and creating a new release.
The job can also be manually run fron the actions tab in github, if a release is needed with the current version, and we need it to be released now.

## Repository tags/commits

We currently use the commit of the branch or the tag if we are triggering the job on a tag, to create the artifacts repo and publish, thus pinpointing the repo contents to the current artifacts.
tag/commit calculation is done in the base `Makefile` and passed to create-repo (on `make/Makefile.iso`) and publish-repo (on `make/Makefile.build`)

## AMI auto cleaning

There is a scheduled job at `.github/workflows/ami-cleaner.yaml` that iterates over all the aws zones and checks how many AMIs and snapshots we have, and cleans older ones as to not have an infinity supply of images stored.
It uses the `.github/ami-cleaner.sh` script to clean up the images and has several options:

 - `DO_CLEANUP` true/false (default: false): Turns on real cleaning mode, otherwise it will act in reporting mode, printing what wactions would have taken.
 - `AMI_OWNER` string (default: 053594193760): Which AMI owner to look for the images. This should not be changed unless our accounts change.
 - `MAX_AMI_NUMBER` int (default: 20): Maximum number of AMIs that can be. This emans that anything over this number will be removed, starting with the oldest ones.

This script also checks for discrepancies between the number of AMIs and snapshots linked to those AMIs, as you can remove an AMI but never remove the backing snapshot. If a discrepancy is found, it will find the orphan snapshots (i.e. has no link to an AMI) and remove those.

## Resigner

The job at `.github/workflows/resigner.yaml` will use the code at `.github/resign.go` to verify the artifacts in a repo and if they are not signed, it will try to sign them and push those signatures to the repo.
The following environment variables are available:

 - `FINAL_REPO` Repo to check artifacts for signatures
 - `COSIGN_REPOSITORY` Repo that contains the signatures for the final_repo, if not given it uses the default FINAL_REPO
 - `FULCIO_URL` Set a fulcio url for the signing part. Leave empty to use cosign default url
 - `REFERENCEID` Name of the repository.yaml that will be downloaded.
 - `DEBUGLOGLEVEL` Set debug log level

Note that this should not be needed to run manually as the ci job can be manually triggered with the above fields pre-filled.
