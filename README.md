# compose-plan

Deploy preview and history for Docker Compose. Run it before a deploy and it
prints what will change and why. Run it to deploy and it writes a diary entry
you can compare against and roll back to.

Docker's own dry run says "Recreate" without saying why, and remembers one
previous version per service. A `latest` tag can move on the registry
without the file changing, so a service restarts that you did not touch.

```
$ compose-plan plan
worker   restart     memory limit 512M -> 1G
web      restart     image "latest" moved: sha256:8f3a... -> sha256:c21d...
db       no change

$ compose-plan history
#42  Sep 15 16:04  ali    3e1f9a0  worker memory, web image
#41  Sep 15 15:02  ali    9b7c2d1  web image
```

## Install

```
go install github.com/Ali-932/compose-plan@latest
```

Accepts the same `-f`, `-p` and `--env-file` flags as `docker compose`, and
uses the registry credentials Docker already stores.

## Status

Skeleton only. Nothing is implemented yet.

```
main.go               flag parsing and subcommand dispatch
internal/compose/     render compose files (compose-go v2)
internal/engine/      what is running (moby/moby/client)
internal/registry/    tag -> digest (go-containerregistry)
internal/plan/        diff desired vs running, render the plan
internal/history/     hash-chained JSONL diary, lock file
testdata/compose.yaml two-service project for tests
```

Version one is Compose only: `plan`, `apply`, `history`, `diff`, `rollback`.
Swarm comes second.

The full brief, scenarios and edge cases are in [project.md](project.md).
