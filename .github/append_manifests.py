#!/usr/bin/env python

import yaml
import sys


def main():
    if len(sys.argv) != 4:
        print("This utility gets a yaml file, a content file and a key and inserts the content from the content"
              "file into the yaml file at the given key")
        print("Missing arguments: [YAML FILE] [CONTENT FILE] [KEY TO ADD THE CONTENT AT]")
        exit(1)

    with open(sys.argv[1], "r") as stream:
        yaml_file = yaml.safe_load(stream)

    with open(sys.argv[2], "r") as mtree:
        yaml_file[sys.argv[3]] = mtree.readlines()

    with open(sys.argv[1], "w") as outfile:
        outfile.write(yaml.dump(yaml_file))


if __name__ == "__main__":
    main()
