#!/usr/bin/env bash

WORKER_FLAVOR="m1.medium"
IMAGE="SLES15-SP2-JeOS.x86_64-15.2-OpenStack-Cloud-GM"
WORKER_NAME="${WORKER_NAME:-}"
WORKER_TEMPLATE="github-runner"
CMD="usage"
GITHUB_URL=${GITHUB_URL:-https://github.com/rancher-sandbox/cOS-toolkit}
LABELS=""
TOKEN=""

function usage(){
cat <<USAGE
Manage the ECP Worker Stack

Notes:
    The jenkins-worker-environment.yaml will override any passed values

ACTIONS:
    -c|--create         Create worker
    -d|--delete         Delete worker
    -i|--ip             Get worker ip

OPTIONS:
    --worker-flavor     The heat flavor to use for the workers (for the default see the heat template for the worker type)
    --worker-name       The name for the worker appended to the stack name
    --worker-template   Template used for creating the worker (github-runner) (default: ${WORKER_TEMPLATE})
    --image             Name of the worker image
    --token             Github runner token
    --github_url        Github url for runner token (default: ${GITHUB_URL})
    --labels            Labels to add to the runner (comma separated list)

USAGE
}

while [[ $# != 0 ]] ; do
  case $1 in
    -c|--create)
      CMD="createWorker"
      ;;
    -d|--delete)
      CMD="deleteWorker"
      ;;
    -i|--ip)
      CMD="getWorkerIP"
      ;;
    --token)
      TOKEN=${2}
      shift
      ;;
    --github_url)
      GITHUB_URL=${2}
      shift
      ;;
    --labels)
      LABELS=${2}
      shift
      ;;
    --worker-flavor)
      WORKER_FLAVOR=${2}
      shift
      ;;
    --worker-name)
      WORKER_NAME=${2}
      shift
      ;;
    --worker-template)
      WORKER_TEMPLATE=${2}
      shift
      ;;
    --image)
      IMAGE=${2}
      shift
      ;;
    -h|--help)
      echo "${USAGE}"
      exit 0
      ;;
    *)
      echo "Invalid parameter ${2}"
      echo "${USAGE}"
      exit 0
      ;;
  esac
  shift
done

set -eu

function showWorkerInfo {
    openstack --insecure server show "$WORKER_NAME"
}

function createWorker() {
    if [ -z "${WORKER_NAME}" ]; then
      echo "WORKER_NAME not set"
      exit 1
    fi


    HEAT_PARAMS="--parameter worker_name=${WORKER_NAME}"

    if [[ -n $WORKER_FLAVOR ]]; then
       HEAT_PARAMS="${HEAT_PARAMS} --parameter worker_flavor=${WORKER_FLAVOR}"
    fi

    if [[ -n $IMAGE ]]; then
      HEAT_PARAMS="${HEAT_PARAMS} --parameter image=${IMAGE}"
    fi

    if [[ -n $GITHUB_URL ]]; then
      HEAT_PARAMS="${HEAT_PARAMS} --parameter github_url=${GITHUB_URL}"
    fi

    if [[ -n $LABELS ]]; then
      # Append image and size to labels
      LABELS="${LABELS},${IMAGE},${WORKER_FLAVOR}"
    else
      LABELS="${IMAGE},${WORKER_FLAVOR}"
    fi

    HEAT_PARAMS="${HEAT_PARAMS} --parameter labels=${LABELS}"

    if [[ -z $TOKEN ]]; then
      echo "Need a runner token to configure the worker"
      exit 1
    else
      HEAT_PARAMS="${HEAT_PARAMS} --parameter token=${TOKEN}"
    fi

    # Create the stack
    STACK_NAME=${STACK_NAME:-$WORKER_NAME}
    # shellcheck disable=SC2086
    openstack --insecure stack create --wait -t "${WORKER_TEMPLATE}".yaml -e environment.yaml "${STACK_NAME}" ${HEAT_PARAMS}

    showWorkerInfo

}

function deleteWorker() {
    openstack --insecure stack delete --wait --yes "${WORKER_NAME}"
}

# execute command
eval ${CMD}
