[![Build Status](https://asottile.visualstudio.com/asottile/_apis/build/status/asottile.dockerfile?branchName=master)](https://asottile.visualstudio.com/asottile/_build/latest?definitionId=14&branchName=master)
[![Build status](https://ci.appveyor.com/api/projects/status/l5kj12ysd49xul1l?svg=true)](https://ci.appveyor.com/project/asottile/dockerfile)

dockerfile
==========

The goal of this repository is to provide a wrapper around
[docker/docker](https://github.com/docker/docker)'s parser for dockerfiles.


## python library

### Installation

This project uses [setuptools-golang](https://github.com/asottile/setuptools-golang)
when built from source.  To build from source you'll need a go compiler.

If you're using linux and sufficiently new pip (>=8.1) you should be able to
just download prebuilt manylinux1 wheels.

```
pip install dockerfile
```

### Usage

There's three api functions provided by this library:

#### `dockerfile.all_cmds()`

List all of the known dockerfile cmds.

```python
>>> dockerfile.all_cmds()
('add', 'arg', 'cmd', 'copy', 'entrypoint', 'env', 'expose', 'from', 'healthcheck', 'label', 'maintainer', 'onbuild', 'run', 'shell', 'stopsignal', 'user', 'volume', 'workdir')
```

#### `dockerfile.parse_file(filename)`

Parse a Dockerfile by filename.
Returns a `tuple` of `dockerfile.Command` objects representing each layer of
the Dockerfile.
Possible exceptions:
- `dockerfile.GoIOError`: The file could not be opened.
- `dockerfile.ParseError`: The Dockerfile was not parseable.

```python
>>> pprint.pprint(dockerfile.parse_file('testfiles/Dockerfile.ok'))
(Command(cmd='from', sub_cmd=None, json=False, original='FROM ubuntu:xenial', start_line=1, flags=(), value=('ubuntu:xenial',)),
 Command(cmd='cmd', sub_cmd=None, json=True, original='CMD ["echo", "hi"]', start_line=2, flags=(), value=('echo', 'hi')))
```

#### `dockerfile.parse_string(s)`

Parse a dockerfile using a string.
Returns a `tuple` of `dockerfile.Command` objects representing each layer of
the Dockerfile.
Possible exceptions:
- `dockerfile.ParseError`: The Dockerfile was not parseable.

```python
>>> dockerfile.parse_string('FROM ubuntu:xenial')
(Command(cmd='from', sub_cmd=None, json=False, original='FROM ubuntu:xenial', start_line=1, flags=(), value=('ubuntu:xenial',)),)
```

## go library

Slightly more convenient than the api provided by docker/docker?  Might not be
terribly useful -- the main point of this repository was a python wrapper.

### Installation

```
go get github.com/asottile/dockerfile
```

### Usage

[godoc](https://godoc.org/github.com/asottile/dockerfile)
