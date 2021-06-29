
# Instruction to boot the generated raw image to AWS

Once you generate the raw image with `make raw_disk`, the generated file needs to be imported to AWS. This file describes the step to consume the image on AWS.

1. Upload the image to an S3 bucket
```
aws s3 cp <cos-image> s3://cos-images
```

2. Created the disk container JSON (`container.json` file) as:

```
{
  "Description": "cOS Testing image in RAW format",
  "Format": "raw",
  "UserBucket": {
    "S3Bucket": "cos-images",
    "S3Key": "<cos-image>"
  }
}
```

3. Import the disk as snapshot

```
aws ec2 import-snapshot --description "cOS PoC" --disk-container file://container.json
```

4. Followed the procedure described in [AWS docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/creating-an-ami-ebs.html#creating-launching-ami-from-snapshot) to register an AMI from snapshot. Used all default settings unless for the firmware, were I forced it to UEFI boot.

5. Launch instance with this simple userdata:
```
name: "Default deployment"
stages:
   rootfs.after:
     - name: "Repart image"
       layout:
         # It will partition a device including the given filesystem label or part label (filesystem label matches first)
         device:
           label: COS_RECOVERY
         # Only last partition can be expanded
         # expand_partition:
         #   size: 4096
         add_partitions:
           - fsLabel: COS_STATE
             size: 8192
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # unset size or 0 size means all available space
             # size: 0 
             # default filesystem is ext2 when omitted
             # filesystem: ext4
             pLabel: persistent
   network:
     - if: '[ -z "$(blkid -L COS_SYSTEM || true)" ]'
       name: "Deploy cos-system"
       commands:                                                                 
         - |
             cos-deploy --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.3-3 && \
             shutdown -r +1

```

You can login with default username/password: `root/cos`.

See also https://github.com/rancher-sandbox/cOS-toolkit/pull/235#issuecomment-853762476