# smoke app

Tiny HTTP server for testing gate locally.

## Run Through Development gate

From the repository root:

```bash
just smoke serve
```

The recipe prints the local HTTPS URL after gate starts. It uses a temporary
upstream port through `SMOKE_PORT`, then gate injects that port into the
sample server. Use `Ctrl-C` to stop the sample server; cleanup runs
automatically.

For a non-interactive smoke check:

```bash
just smoke check
```

The first browser visit may show `ERR_CERT_AUTHORITY_INVALID`. That means the local gate CA is not trusted yet. For smoke testing, use the browser's advanced/proceed flow.

To remove the warning:

```bash
bin/gate trust
```

Run it from the repository root. It may ask for administrator approval.
