#!/bin/bash
set -ex

for i in $(ls config); do 
    gomplate --left-delim "{{{" --right-delim "}}}" -V --datasource config=config/$i --file build.yaml.gomplate > workflows/build-$i
done