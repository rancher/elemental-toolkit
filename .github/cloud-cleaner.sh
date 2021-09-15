HOURS=${HOURS:-8}
LAUNCH_DATE=$(date --date="-${HOURS} hours" "+%Y-%m-%dT%H:%M")
DO_CLEANUP=${DO_CLEANUP:-false}
EXCLUDE_LIST=${EXCLUDE_LIST:-}

regions=( $( aws ec2 describe-regions | jq '.Regions[].RegionName' -r ) )

echo "------------------------------------------------------------------"
echo "Checking instances that started before ${LAUNCH_DATE}"
echo "------------------------------------------------------------------"
touch instances.txt

function list_includes_item {
  local list="$1"
  local item="$2"
  if [[ $list =~ (^|[[:space:]])"$item"($|[[:space:]]) ]] ; then
    # yes, list includes item
    result=0
  else
    result=1
  fi
  return $result
}


if [[ "${DO_CLEANUP}"  == "true" ]]; then
  printf "\xF0\x9F\x92\x80 Cleaning mode activated\n"
  if [ ! -z "${EXCLUDE_LIST}" ]; then
    printf "\xF0\x9F\x93\x84 Exclude list: %s\n" "${EXCLUDE_LIST}"
  else
    printf "\xF0\x9F\x93\x84 Exclude list empty\n"
  fi
else
  printf "\xF0\x9F\x93\x84 Reporting mode activated\n"
fi

for region in "${regions[@]}"; do
  printf "\xE2\x86\x92 Checking instances on region %s\n" "${region}"
  export AWS_DEFAULT_REGION=${region}
  if [[ "${DO_CLEANUP}"  == "true" ]]; then
    instances=( $( aws ec2 describe-instances --query "Reservations[].Instances[?LaunchTime<=\`$LAUNCH_DATE\`][].InstanceId"|jq '.[]' -r ) )
    for instance in "${instances[@]}"; do
      if list_includes_item "${EXCLUDE_LIST}" "${instance}" ; then
        printf "\xE2\x9D\x97 Instance %s excluded from shutting down, skipping...\n" "${instance}"
        echo "Skipped terminating instance ${instance} on region ${region} due to EXCLUDE_LIST" >> instances.txt
      else
        printf "\xE2\x9D\x8C Terminating instance %s\n" "${instance}"
        out=$( aws ec2 terminate-instances --instance-ids "${instance}" |jq '.TerminatingInstances[0].CurrentState.Name' -r )
        printf "\xE2\x9C\x85 Instance %s reported status %s\n" "${instance}" "${out}"
        echo "Terminated instance ${instance} on region ${region} with status ${out}" >> instances.txt
      fi
    done
  else
    instances_report=$( aws ec2 describe-instances --query "Reservations[].Instances[?LaunchTime<=\`$LAUNCH_DATE\`][].{id: InstanceId, type: InstanceType, launched: LaunchTime, region: Placement.AvailabilityZone, tags: Tags}"|jq '.[]' )
    if [ ! -z "${instances_report}" ]; then
      printf "\xE2\x9C\x85 Adding instances to file: %s\n" "${instances_report}"
      echo "${instances_report}" >> instances.txt
    fi
  fi
done