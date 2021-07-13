#!/bin/bash 

set -e

disk="$1"
s3_bucket="cos-images"
disk_name="cOS-Vanilla"
disk_desc="cOS Vanilla Image"
: "${git_sha:=none}"

[ -f "${disk}" ] || exit 1

echo "Uploading to S3 Bucket: ${s3_bucket}"
aws s3 cp "${disk}" "s3://${s3_bucket}" --metadata "Name=${disk_name}"

echo "Import snapshot to EC2"
task_id=$(aws ec2 import-snapshot --description "${disk_desc}" \
    --disk-container "Description=${disk_desc},Format=raw,UserBucket={S3Bucket=${s3_bucket},S3Key=$(basename ${disk})}" \
    --tag-specifications "ResourceType=import-snapshot-task,Tags=[{Key=Name,Value=${disk_name}}]" \
    | jq -r '.ImportTaskId')

task_status=''

# 10min timeout to import snapshot
counter=0
while [ ! "${task_status}" = "completed" ] && [ $counter -lt 60  ]; do
    echo "Waiting for import-snapshot task to finalize. $((counter*10))s"
    sleep 10
    task_status=$(aws ec2 describe-import-snapshot-tasks \
        --import-task-ids ${task_id} \
        | jq -r '.ImportSnapshotTasks[0].SnapshotTaskDetail.Status')
    counter=$((counter + 1))
done

[ ! "${task_status}" = "completed" ] && exit 1

snap_id=$(aws ec2 describe-import-snapshot-tasks \
    --import-task-ids ${task_id} \
    | jq -r '.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId')

echo "Tagging Snapshot"
aws ec2 create-tags --resources "${snap_id}" \
    --tags Key=Name,Value=${disk_name} Key=Project,Value=cOS Key=Git_SHA,Value=$git_sha Key=Flavor,Value=recovery

echo "Register AMI from snapshot"
ami_id=$(aws ec2 register-image \
   --name "${disk_name}-$(date +"%m%d%Y_%H%M%S")" \
   --description "${disk_desc}" \
   --architecture x86_64 \
   --virtualization-type hvm \
   --ena-support \
   --boot-mode uefi \
   --root-device-name "/dev/sda1" \
   --block-device-mappings "DeviceName=/dev/sda1,Ebs={SnapshotId=${snap_id}}" \
   | jq -r '.ImageId')

echo "Tagging AMI"
aws ec2 create-tags --resources "${ami_id}" --tags \
   --tags Key=Name,Value=${disk_name} Key=Project,Value=cOS Key=Git_SHA,Value=$git_sha Key=Flavor,Value=recovery

echo "AMI Created: ${ami_id}"
