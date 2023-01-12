#!/bin/bash

set -e

if [ -e /etc/environment ]; then
    source /etc/environment
fi

if [ -e /etc/os-release ]; then
    source /etc/os-release
fi

if [ -e /etc/cos/config ]; then
    source /etc/cos/config
fi

if [ -z "$OEM_LABELDIR" ]; then
  OEM_LABELDIR=/oem
fi

if [ -z "$COS_FEATURESDIR" ]; then
  COS_FEATURESDIR=/system/features
fi

usage()
{
    echo "Usage: cos-feature [list|enable|disable] <feature>"
    echo ""
    echo "Example: cos-feature enable k3s"
    echo ""
    echo "To list available features, run: cos-feature list"
    echo "To enable, run: cos-feature enable <feature>"
    echo "To disable, run: cos-feature disable <feature>"
    echo ""
    exit 1
}

list() {
  echo ""
  echo "===================="
  echo "Elemental features list"
  echo ""
  echo "To enable, run: cos-feature enable <feature>"
  echo "To disable, run: cos-feature disable <feature>"
  echo "===================="
  echo ""
  for i in $COS_FEATURESDIR/*.yaml; do
    f=$(basename $i .yaml)
    if [ -L "$OEM_LABELDIR/features/${f}.yaml" ]; then
      enabled="(enabled)"
    fi
    echo "- $f $enabled"
  done

}

enable() {
  for i in $@; do
    if [ ! -e "$COS_FEATURESDIR/$i.yaml" ]; then
      echo "Feature not present"
      exit 1
    fi
    if [ ! -d "$OEM_LABELDIR/features" ]; then
      mkdir $OEM_LABELDIR/features
    fi
    if [ -L "$OEM_LABELDIR/features/$i.yaml" ]; then
      echo "Feature $i enabled already"
      continue
    fi
    ln -s $COS_FEATURESDIR/$i.yaml $OEM_LABELDIR/features/$i.yaml
    echo "$i enabled"
  done
}

disable() {
  for i in $@; do
    if [ -L "$OEM_LABELDIR/features/$i.yaml" ]; then
      rm $OEM_LABELDIR/features/$i.yaml
      echo "Feature $i disabled"
    else
      echo "Feature $i not enabled"
    fi
  done
}

while [ "$#" -gt 0 ]; do
    case $1 in
        list)
            shift 1
            list
            ;;
        enable)
            shift 1
            enable $@
            ;;
        disable)
            shift 1
            disable $@
            ;;
        -h)
            usage
            ;;
        --help)
            usage
            ;;
        *)
            if [ "$#" -gt 2 ]; then
                usage
            fi
            INTERACTIVE=true
            break
            ;;
    esac
done
