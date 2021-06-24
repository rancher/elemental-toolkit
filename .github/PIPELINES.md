# Github pipelines for cos toolkit

Requirements:
- gomplate

Pipelines are generated from `build.yaml.gomplate`, which is a gomplate template. To generate the pipelines, run `./pipelines.sh`.

Each configuration in `config` is turned to its own pipeline, named `build-$NAME`.
