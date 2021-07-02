# Multiverse

Multiverse is a universal, general purpose, Content-Addressable Store (CAS) to publish and serve static content at Internet scale.

## Server

The Multiverse server exposes an HTTP API to allow clients to push and pull individual nodes, identified by their hash.

In order to run the server locally, use the following command:

```bash
./run_server
```

## Command-Line Interface

The Multiverse CLI offers a way to operate on files on the local file system and sync them to one or more Multiverse servers or local directories.


The CLI can be built and installed with the following command:

```bash
go install ./cmd/multi
```

And is then available via the binary called `multi`:

```bash
multi help
```
