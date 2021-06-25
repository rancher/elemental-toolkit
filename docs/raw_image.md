
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
name: "Default ec2 user"
stages:
   network.after:
     - name: "Setup user"
       users:
         ec2user:
           name: "ec2user"
           passwd: "$6$M5bXBW/.7pspU1n7$d3Un967.AG8yf9YK2qlUFJt/3EQR57Vrlhtil866FglGY9dI2/arcBCcbSk7/faSq8pkwf1dkD.tDX7iXLOuG1"
           primary_group: "users"
           shell: "/bin/bash"
           homedir: "/home/ec2user"
```

See also https://github.com/rancher-sandbox/cOS-toolkit/pull/235#issuecomment-853762476