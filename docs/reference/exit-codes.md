---
description: vlx exit codes, for scripts.
---

# Exit codes

| Code | Meaning | Example |
| --- | --- | --- |
| `0` | Success. | The call returned its replies. |
| `1` | Failure that fits nowhere else. | The connection broke mid-stream. |
| `2` | Usage error: bad flags, arguments, parameters or config. | `.id: expected int, got string` |
| `3` | The service returned a Varlink error. | `io.systemd.Resolve.NoNameServers` |
| `4` | vlx refused the call under its safety policy. | A WRITE method without `--allow-writes`, or without `--yes` in a script. |
| `5` | The service could not be found or reached. | No socket matches the name; nothing listens on it. |
| `130` | Interrupted with ++ctrl+c++. | A stream stopped by hand. |

```sh
vlx call demo SetDefaultLanguage '{"language":"german"}'
case $? in
  0) echo done ;;
  4) echo "refused: rerun with --allow-writes" ;;
  *) echo "failed" ;;
esac
```
