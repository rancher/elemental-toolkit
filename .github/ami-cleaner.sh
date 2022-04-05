#!/bin/bash
DO_CLEANUP=${DO_CLEANUP:-false}
AMI_OWNER=${AMI_OWNER:-"053594193760"}
MAX_AMI_NUMBER=${MAX_AMI_NUMBER:-20}

set -e


regions=( $( aws ec2 describe-regions | jq '.Regions[].RegionName' -r ) )



echo "------------------------------------------------------------------"
if [[ "${DO_CLEANUP}"  == "true" ]]; then
  printf "\xF0\x9F\x92\x80 Cleaning mode activated\n"
else
  printf "\xF0\x9F\x93\x84 Reporting mode activated\n"
fi
echo "Maximun number of AMIs allowed: $MAX_AMI_NUMBER"
echo "Owner of the AMIs: $AMI_OWNER"
echo "Regions: ${regions[@]}"
echo "------------------------------------------------------------------"

touch removed.txt

containsElement () {
  local e match="$1"
  shift
  for e; do [[ "$e" == "$match" ]] && return 0; done
  return 1
}

for region in "${regions[@]}"; do
  printf "\xE2\x86\x92 Checking AMIS on region %s for owner %s\n" "${region}" "${AMI_OWNER}"
  export AWS_DEFAULT_REGION=${region}
  ami_number=$( aws ec2 describe-images --owners $AMI_OWNER | jq '.Images| length' )
  snapshots_number=$( aws ec2 describe-snapshots --owner-id $AMI_OWNER | jq '.Snapshots| length' )
  printf "\xE2\x86\x92 Found %s AMIs\n" "$ami_number"
  printf "\xE2\x86\x92 Found %s Snapshots\n" "$snapshots_number"
  if [[ $ami_number > $MAX_AMI_NUMBER ]]; then
    printf "\xE2\x86\x92 AMI number is bigger than max number allowed, engaging cleanup.\n"
    to_clean=$(expr $ami_number - $MAX_AMI_NUMBER)
    printf "\xE2\x86\x92 Cleaning %s AMIs+Snapshots\n" "$to_clean"
    to_clean_amis=$(aws ec2 describe-images --owners $AMI_OWNER |jq -r ".Images | sort_by(.CreationDate) | .[:${to_clean}] | .[]| {ImageId: .ImageId, CreationDate: .CreationDate, SnapshotId: .BlockDeviceMappings[].Ebs.SnapshotId}")
      if [[ "${DO_CLEANUP}"  == "true" ]]; then
          ami_ids=( $( echo $to_clean_amis | jq -r ".ImageId" ) )
          for ami in "${ami_ids[@]}"; do
            printf "\xE2\x9D\x8C Removing AMI %s\n" "$ami"
            aws ec2 deregister-image --image-id ${ami}
            echo "Removed AMI ${ami} on region ${region}" >> removed.txt
          done
          snapshot_ids=( $( echo $to_clean_amis | jq -r ".SnapshotId" ) )
          # Snapshot wont be deleted if its in use
          for snapshot in "${snapshot_ids[@]}"; do
            printf "\xE2\x9D\x8C Removing Snapshot %s\n" "$snapshot"
            aws ec2 delete-snapshot --snapshot-id ${snapshot}
            echo "Removed Snapshot ${snapshot} on region ${region}" >> removed.txt
          done
      else
        printf "\xE2\x86\x92 Would have removed the following AMIs+Snapshots:\n"
        echo $to_clean_amis | jq
      fi
  else
    printf "\xE2\x9C\x85 Ami number is equal or lower to max number allowed, skipping.\n"
  fi
  if [[ $snapshots_number > $ami_number ]]; then
    printf "\U2757 snapshots(%s) and AMIs(%s) do not match! Checking....\n" "$snapshots_number" "$ami_number"
    ami_snapshots=( $(aws ec2 describe-images --owners $AMI_OWNER |jq -r ".Images | .[].BlockDeviceMappings[].Ebs.SnapshotId") )
    snapshots=( $( aws ec2 describe-snapshots --owner-id $AMI_OWNER | jq -r '.Snapshots| .[].SnapshotId' ) )
    for snap in "${snapshots[@]}"; do
        if containsElement $snap "${ami_snapshots[@]}"; then
            printf "\U1F44D snapshot %s appears to be linked to an AMI\n" "$snap"
        else
            printf "\U274c snapshot %s does not appear to be linked to an AMI\n" "$snap"
            if [[ "${DO_CLEANUP}"  == "true" ]]; then
                aws ec2 delete-snapshot --snapshot-id ${snap}
                 printf "\U274c snapshot %s deleted\n" "$snap"
                 echo "Removed Snapshot ${snapshot} on region ${region}" >> removed.txt
            else
                printf "\U274c Would have erased snap %s\n" $snap
            fi
        fi
    done
  fi
done


