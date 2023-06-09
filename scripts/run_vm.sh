#!/bin/bash

set -e

SCRIPT=$(realpath -s "${0}")
SCRIPTS_PATH=$(dirname "${SCRIPT}")
TESTS_PATH=$(realpath -s "${SCRIPTS_PATH}/../tests")

: "${ELMNTL_FIRMWARE:=/usr/share/qemu/ovmf-x86_64.bin}"
: "${ELMNTL_FWDIP:=127.0.0.1}"
: "${ELMNTL_FWDPORT:=2222}"
: "${ELMNTL_MEMORY:=4096}"
: "${ELMNTL_LOGFILE:=${TESTS_PATH}/serial.log}"
: "${ELMNTL_PIDFILE:=${TESTS_PATH}/testvm.pid}"
: "${ELMNTL_TESTDISK:=${TESTS_PATH}/testdisk.qcow2}"
: "${ELMNTL_DISKSIZE:=20G}"
: "${ELMNTL_DISPLAY:=none}"
: "${ELMNTL_ACCEL:=kvm}"
: "${ELMNTL_TARGETARCH:=$(uname -p)}"
: "${ELMNTL_MACHINETYPE:=q35}"
: "${ELMNTL_CPU:=max}"

function _abort {
    echo "$@" && exit 1
}

function start {
  local base_disk=$1
  local usrnet_arg="-netdev user,id=user0,hostfwd=tcp:${ELMNTL_FWDIP}:${ELMNTL_FWDPORT}-:22 -device e1000,netdev=user0"
  local accel_arg
  local memory_arg="-m ${ELMNTL_MEMORY}"
  local firmware_arg="-drive if=pflash,format=raw,readonly=on,file=${ELMNTL_FIRMWARE}"
  local disk_arg="-hda ${ELMNTL_TESTDISK}"
  local serial_arg="-serial file:${ELMNTL_LOGFILE}"
  local pidfile_arg="-pidfile ${ELMNTL_PIDFILE}"
  local display_arg="-display ${ELMNTL_DISPLAY}"
  local daemon_arg="-daemonize"
  local machine_arg="-machine type=${ELMNTL_MACHINETYPE}"
  local cdrom_arg
  local cpu_arg="-cpu ${ELMNTL_CPU}"
  local vmpid
  local kvm_arg

  [ -f "${base_disk}" ] || _abort "Disk not found: ${base_disk}"

  if [ -f "${ELMNTL_PIDFILE}" ]; then
    vmpid=$(cat "${ELMNTL_PIDFILE}")
    if ps -p ${vmpid} > /dev/null; then
      echo "test VM is already running with pid ${vmpid}"
      exit 0
    else
      echo "removing outdated pidfile ${ELMNTL_PIDFILE}"
      rm "${ELMNTL_PIDFILE}"
    fi
  fi

  case "${base_disk}" in
      *.qcow2)
        qemu-img create -f qcow2 -b "${base_disk}" -F qcow2 "${ELMNTL_TESTDISK}" > /dev/null
        ;;
      *.iso)
        qemu-img create -f qcow2 "${ELMNTL_TESTDISK}" "${ELMNTL_DISKSIZE}" > /dev/null
        cdrom_arg="-cdrom ${base_disk}"
        ;;
      *)
        _abort "Expected a *.qcow2 or *.iso file"
        ;;
  esac

  [ "hvf" == "${ELMNTL_ACCEL}" ] && accel_arg="-accel ${ELMNTL_ACCEL}" && firmware_arg="-bios ${ELMNTL_FIRMWARE} ${firmware_arg}"
  [ "kvm" == "${ELMNTL_ACCEL}" ] && cpu_arg="-cpu host" && kvm_arg="-enable-kvm"

  qemu-system-${ELMNTL_TARGETARCH} ${kvm_arg} ${disk_arg} ${cdrom_arg} ${firmware_arg} ${usrnet_arg} \
      ${kvm_arg} ${memory_arg} ${graphics_arg} ${serial_arg} ${pidfile_arg} \
      ${daemon_arg} ${display_arg} ${machine_arg} ${accel_arg} ${cpu_arg}
}

function stop {
  local vmpid
  local killprog

  if [ -f "${ELMNTL_PIDFILE}" ]; then
    vmpid=$(cat "${ELMNTL_PIDFILE}")
    killprog=$(which kill)
    if ${killprog} --version | grep -q util-linux; then
        ${killprog} --verbose --timeout 1000 TERM --timeout 5000 KILL --signal QUIT ${vmpid}
    else
        ${killprog} -9 ${vmpid}
    fi
    rm -f "${ELMNTL_PIDFILE}"
  else
    echo "No pidfile ${ELMNTL_PIDFILE} found, nothing to stop"
  fi
}

function clean {
  ([ -f "${ELMNTL_LOGFILE}" ] && rm -f "${ELMNTL_LOGFILE}") || true
  ([ -f "${ELMNTL_TESTDISK}" ] && rm -f "${ELMNTL_TESTDISK}") || true
}

cmd=$1
disk=$2

case $cmd in
  start)
    start "${disk}"
    ;;
  stop)
    stop
    ;;
  clean)
    clean
    ;;
  *)
    _abort "Unknown command: ${cmd}"
    ;;
esac

exit 0
