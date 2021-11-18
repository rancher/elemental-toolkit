# Tips and tricks for development

Some helpful tips and docs for development

## Enable video capture on virtualbox (packer, tests)

When building the images with packer or running the tests with virtualbox, there is a possibility of enabling video capture of the machine output for easy troubleshooting.
This is helful on situations when we cannot access the machine directly like in CI tests.

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

For bios:

 - *(Makefile)* Create a vbox machine
 - *(Makefile)* Set bios
 - *(Makefile)* Set the boot order, so it boots from disk first, then dvd. This works because the initial disk is empty, so it will force booting from dvd on the first boot and allows us to test the installer from ISO.
 - *(Ginkgo)* Run test suite
 - *(Ginkgo)* Reboot. Now because of the boot order, the VM will boot from disk properly as it contains grub
 - *(Ginkgo)* Tests will check to see if whatever was run in the test was correctly (i.e. layout, partitions, config files, etc...)
 - *(Ginkgo)* After each test, the disk partition table is wiped out so the next test will see an empty disk and boot from dvd again.

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

Raw image tests are tests run from the raw image created by `images/img-builder.sh` which creates a raw image with the recovery partition only.
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
