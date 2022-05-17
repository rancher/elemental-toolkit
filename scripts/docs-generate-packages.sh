#!/bin/bash
set -e
# This scripts read package specs with a 'documentation' field and generates the corresponding pages in the website

TREE_DIR=$1
if [ -z "$TREE_DIR" ]; then 
    echo "Need to specify a package tree directory"
    exit 1
fi

GENERATED_DOCS="${2:-docs/content/en/docs/Reference/Packages}"

echo "Generating docs at $GENERATED_DOCS"

PKG_LIST=$(luet tree pkglist --tree $TREE_DIR -o json)

header() {
    local pn=$1
    local pc=$2
    local pv=$3
    cat <<EOF
---
title: "$pc/$pn"
linkTitle: "$pn"
date: $(date '+%Y-%m-%d')
description: >
  Documentation for the $pc/$pn package
---

{{% alert title="Note" %}}
{{<package package="$pc/$pn" >}} is part of the cOS toolkit repository and can be installed with:

\`\`\`bash
luet install -y $pc/$pn
\`\`\`

in the derivative's Dockerfile.
{{% /alert %}}

EOF
}

write_package() {
    local package_path=$1
    local package_name=$2
    local package_category=$3
    local package_version=$4
    local readme_file=$5
    local subdir=$6
    if [ -z "$readme_file" ]; then
        readme_file="README-$package_category-$package_name.md"
    fi
    if [ -e "$package_path/$readme_file" ]; then
      echo "$(header $package_name $package_category $package_version)" > "$GENERATED_DOCS/$subdir/$package_category-$package_name.md"
      cat "$package_path/$readme_file" >> "$GENERATED_DOCS/$subdir/$package_category-$package_name.md"
      echo "Generated $GENERATED_DOCS/$subdir/$package_category-$package_name.md"
    fi
}

if [ ! -d $GENERATED_DOCS ]; then
    mkdir -p $GENERATED_DOCS
fi

for i in $(echo "$PKG_LIST" | jq -rc '.packages[]'); do
    PACKAGE_PATH=$(echo "$i" | jq -r ".path")
    PACKAGE_NAME=$(echo "$i" | jq -r ".name")
    PACKAGE_CATEGORY=$(echo "$i" | jq -r ".category")
    PACKAGE_VERSION=$(echo "$i" | jq -r ".version")

    # We read README.md and README-cat-name.md files.
    # When a collection is found, we generate an _index.md with README.md and README-head.md content if they exists, while
    # collection packages specific documentation can be supplied with files like `README-cat-name.md`
    if [ -e "$PACKAGE_PATH/collection.yaml" ]; then
       if [ -e "$PACKAGE_PATH/README.md" ] && [ -e "$PACKAGE_PATH/README-head.md" ]; then
        # Unique folder for each collection
        dir=$(yq e '.linkTitle' $PACKAGE_PATH/README-head.md | head -n1)
        dir="${dir/ /-}"
        if [ ! -d "$GENERATED_DOCS/$dir" ]; then
            mkdir -p "$GENERATED_DOCS/$dir"
            cat "$PACKAGE_PATH/README-head.md" > "$GENERATED_DOCS/$dir/_index.md"
            cat "$PACKAGE_PATH/README.md" >> "$GENERATED_DOCS/$dir/_index.md"
            echo "Generated $GENERATED_DOCS/$dir/_index.md"
        fi
        write_package $PACKAGE_PATH $PACKAGE_NAME $PACKAGE_CATEGORY $PACKAGE_VERSION "" "$dir"
       else
        write_package $PACKAGE_PATH $PACKAGE_NAME $PACKAGE_CATEGORY $PACKAGE_VERSION "" ""
       fi
    else
        # Packages which are not a collection are easier, the README.md is 
        # directly used for generating the page
        write_package $PACKAGE_PATH $PACKAGE_NAME $PACKAGE_CATEGORY $PACKAGE_VERSION "README.md" ""
    fi
done
