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

The CLI relies on a local configuration file at `~/.config/ent.toml`, which should contain a list of remotes, e.g.:

```toml
default_remote = "local"

[remotes.local]
path = "/home/tzn/.cache/ent"

[remotes.01]
url = "https://01.plus"
```

Note that `~` and env variables are **not** expanded.

The CLI can be built and installed with the following command:

```bash
go install ./cmd/ent
```

And is then available via the binary called `ent`:

```bash
ent help
```
