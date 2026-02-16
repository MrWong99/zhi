# Zhi Mirror / Marketplace Compose Deployment

This is a workspace setup to deploy the zhi mirror or marketplace using Docker Compose.
I also serves as a reference for how to implement a lightweight yet fully functional zhi workspace.

## Prerequisites

### Required Tools

- `docker` with the Compose plugin (`docker compose`)

### Required Apps

A Vault server with the `secret` KV v2 engine enabled and a login with permissions to create secrets.
You can use the [Vault workspace](../vault/README.md) in this repository to quickly set up a Vault server for testing.

## Usage

Just run `zhi edit`, login to Vault, make the changes to your config tree and finally apply!
