# Ent

Ent is an experimental universal, general purpose, Content-Addressable Store (CAS) to explore transparency logs, policies and graph structures.

This is not an officially supported Google product.

## Object Store

At its core, Ent can store and retrieve uninterpreted bytes (called objects) by their hash.

## DAG Service

TODO

## Server

The Ent server exposes an HTTP API to allow clients to push and pull individual nodes, identified by their hash.

In order to run the server locally, use the following command:

```bash
./run_server
```

## Command-Line Interface

The Ent CLI offers a way to operate on files on the local file system and sync them to one or more Ent servers or local directories.

### Installation

The CLI can be built and installed with the following command:

```bash
go install ./cmd/ent
```

And is then available via the binary called `ent`:

```bash
ent help
```

### Configuration

The CLI relies on a local configuration file at `~/.config/ent.toml`, which should contain a list of remotes, e.g.:

```toml
default_remote = "01"

[remotes.01]
url = "https://01.plus"

[remotes.local]
path = "/home/tzn/.cache/ent"
```

Note that `~` and env variables are **not** expanded.

### `status`

`ent status` returns a summary of each file in the current directory, indicating
for each of them whether or not it is present in the remote.

### `push`

`ent push` pushes any file from the current directory to the remote if it is not
already there.

### `make`

`ent make` reads a file called `entplan.toml` in the current directory, such as
the following:

```toml
[[overrides]]
path = "example"
from = "bafybeie46l4ev3o5jvzsuuwdnrp3z45522crey6oaauilgimvlyngokoxm"

[[overrides]]
path = "test/cmd"
from = "bafybeidglc4sbje2sbrfmr6ukt2db5alsg3annuwzjlj3pjpyqh2vh2go4"
```

Each `overrides` entry specifies a local path and the id of a node to pull into
that path from a remote.

For each entry, `ent make` creates the directory at the specified path (if not
already existing) and recursively pulls the specified node into it.

Directories not specified in `entplan.toml` are left unaffected.

It is conceptually similar to
[git submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules).
