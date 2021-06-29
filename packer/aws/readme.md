# steps

 - build cos (sudo make build)
 - create repo (sudo make create-repo)
 - create raw image (sudo make raw_disk)
 - upload raw image to s3 (aws s3 cp IMAGE s3://cos-images/)
 - import image as snapshot (aws ec2 import-snapshot --description "Cos raw import" --disk-container "file://containers.json")

containers.json:
```json
{
  "Description": "Example image originally in raw format",
  "Format": "raw",
  "UserBucket": {
    "S3Bucket": "cos-images",
    "S3Key": "IMAGE_NAME"
  }
}
```

 - create ami from snapshot in aws console
 - launch packer with aws image creation (make packer-aws)


# image creation

 - Set proper disk partitions (aws/setup-disk.yaml)
 - cos-deploy:
   `cos-deploy --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.3-3`