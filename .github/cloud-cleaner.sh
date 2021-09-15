LAUNCH_DATE=$(date --date="-8 hours" "+%Y-%m-%dT%H:%M")
regions=( $( aws ec2 describe-regions | jq '.Regions[].RegionName' -r ) )

for region in "${regions[@]}"; do
  echo "Checking instances on region ${region}"
  export AWS_DEFAULT_REGION=${region}
  instances=$( aws ec2 describe-instances --query "Reservations[].Instances[?LaunchTime<=\`$LAUNCH_DATE\`][].{id: InstanceId, type: InstanceType, launched: LaunchTime, region: Placement.AvailabilityZone, tags: Tags}"|jq '.[]' )
  if [ ! -z "${instances}" ]; then
    echo "Adding instances to file: ${instances}"
    echo "${instances}" >> instances.txt
  fi
done