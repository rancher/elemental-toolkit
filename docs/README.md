The website content it is deployed automatically in the `gh-pages` branch by Github Actions, and no manual intevention is needed.

## Test your changes

After cloning the repo (with submodules), just run `make serve` to test the website locally.

```
$> git clone --recurse-submodule https://github.com/rancher/elemental-toolkit
$> cd elemental-toolkit
$> cd docs; make serve
```

Or just sync the submodules:

```
$> git submodule update --init --recursive --depth 1
```

To run the website locally in other platforms, e.g. MacOS:

```
$> HUGO_PLATFORM=macOS-64bit make serve
```
