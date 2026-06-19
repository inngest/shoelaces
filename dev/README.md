# Shoelaces Development Profile

Run a local development server from the repository root:

```sh
make dev
```

The `dev/shoelaces.yaml` profile listens on `localhost:18081`, disables TFTP,
uses SQLite persistence under `dev/data-dir/runtime/shoelaces.db`, and uses
`dev/data-dir` for site policy and disk overrides. The boot templates come from
the embedded provisioning defaults unless you add `.slc` files under
`dev/data-dir`.

See [`docs/persistence.md`](../docs/persistence.md) for the runtime database
layout, retention behavior, and sqlc/Goose workflow.

Useful URLs:

- <http://localhost:18081/>
- <http://localhost:18081/start>
- <http://localhost:18081/poll/1/06-66-de-ad-be-ef>
- <http://localhost:18081/configs/static/README.txt>
