# Multiverse

Multiverse is a universal, general purpose, Content-Addressable Store (CAS) to publish and serve static content at Internet scale.

This is not an officially supported Google product

## Server

The Multiverse server exposes an HTTP API to allow clients to push and pull individual nodes, identified by their hash.

In order to run the server locally, use the following command:

```bash
./run_server
```

## Command-Line Interface

The Multiverse CLI offers a way to operate on files on the local file system and sync them to one or more Multiverse servers or local directories.

The CLI relies on a local configuration file at `~/.config/multiverse.toml`, which should contain a list of remotes, e.g.:

```toml
default_remote = "local"

[remotes.local]
path = "/home/tzn/.cache/multiverse"

[remotes.01]
url = "https://01.plus"
```

Note that `~` and env variables are **not** expanded.

The CLI can be built and installed with the following command:

```bash
go install ./cmd/multi
```

And is then available via the binary called `multi`:

```bash
multi help
```
