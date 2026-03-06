# e2e log collector CLI

This CLI collects machine logs using the logic in `test/e2e/log_collector.go`.

## Usage

Example:

```bash
go run ./test/e2e/cli --output-dir /tmp/cp-0-logs 1.2.3.4
```

Notes:

- Requires `HETZNER_SSH_PRIV` to be set to base64-encoded private key content.
- Connects as `root` over SSH port `22`.
