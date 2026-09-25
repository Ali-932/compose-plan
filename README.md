# compose-plan

compose-plan shows what a Docker Compose deploy will change before you run it. It also records each deploy you make with it, so you can compare deploys and roll back.

`docker compose up` recreates whatever it decides changed. It does not tell you why, and a tag like `latest` can point to a new image without your file changing. After the deploy, nothing records what was running before.

```
$ compose-plan plan
cache   update   volumes: none -> [{"source":"cache-data","target":"/data","type":"volume",...
web     update   1 env value(s) changed (DATABASE_URL), values hidden
                 command: none -> ["nginx","-g","daemon off; worker_processes 2;"]
                 ports: none -> [{"mode":"ingress","protocol":"tcp","published":"18091","...
                 restart: none -> "unless-stopped"
```

## Install

Requires Go 1.26 or newer and Docker with the Compose plugin.

```
go install github.com/Ali-932/compose-plan/cmd/compose-plan@latest
```

## Usage

Run it in the directory that holds your `compose.yaml`. It takes the same `-f`, `-p`, and `--env-file` flags as `docker compose`:

```
compose-plan [-f file]... [-p name] [--env-file file]... <command>
```

| Command | What it does |
|---|---|
| `plan` | Shows what `apply` would change, service by service, with the reason. |
| `apply` | Runs `docker compose up -d --remove-orphans` and records the deploy in the history. |
| `history` | Lists recorded deploys, newest first. |
| `diff A B` | Shows which images differ between history entries A and B. |
| `rollback N` | Deploys the images recorded in entry N again. Your current compose file supplies every other setting. |

A typical deploy:

```
$ compose-plan plan
web   update   image nginx:1.26-alpine -> nginx:1.27-alpine

$ compose-plan apply
deployed entry #2  2026-09-25 18:04  ali  3e1f9a0  1 services

$ compose-plan history
#2   Sep 25 18:04   ali   3e1f9a0   web
#1   Sep 25 15:02   ali   9b7c2d1   web

$ compose-plan diff 1 2
web   image   sha256:1eadbb078203... -> sha256:65645c7bb6a0...

$ compose-plan rollback 1
web   image   sha256:65645c7bb6a0... -> sha256:1eadbb078203...
deployed entry #3  2026-09-25 18:10  ali  3e1f9a0  1 services
containers were restored, data was not: database migrations run since that entry are still applied
config at #1 was commit 9b7c2d1, current tree is 3e1f9a0; to restore it: git checkout 9b7c2d1 && compose-plan apply
```

## What the plan checks

`plan` lists every service with one of four actions: `create`, `update`, `remove`, or `no change`.

- `create`: the service is in the file but not running.
- `remove`: the service is running but no longer in the file.
- `update`: the image in the file differs from the running one, the tag now points to a different image, or a setting changed since the last deploy.

A tag can point to a different image because you rebuilt it on this machine or because the registry moved it. When the registry moved it, `apply` pulls the new image first. Services with a `build` section skip the registry check.

To find changed settings, compose-plan compares each service's config in your file with the config it recorded at the last deploy. Every top-level key counts: ports, volumes, command, memory, environment, and the rest.

Environment values never appear in the output or in the history file. The history stores an 8-character hash of each value, which is enough to tell that a value changed.

To check a tag, compose-plan asks the registry with the credentials Docker already stores. If the registry cannot be reached, the plan prints "could not check registry" with the error instead of guessing.

`apply` always runs `docker compose up -d`, so Compose makes the final decision about what to recreate. The plan explains that decision in advance. It does not replace it.

## The history file

Each project keeps its history in `.compose-plan/history.jsonl`, next to the compose file. Every line is one deploy: the time, the user, the git commit, the exact image each service ran, and each service's config with environment values hashed. Each entry includes a hash of the one before it, so editing an old entry by hand makes every command that reads the history fail with a hash mismatch. `apply` locks the file while it writes, so a second deploy started at the same time fails with "another deploy is in progress".

To share the history, commit the file to git.

## Development

```
go test -short ./...   # unit tests, no Docker needed
go test ./...          # also runs the end-to-end tests in e2e/ against Docker
```

The end-to-end tests create a throwaway project named `cptest` and remove it when they finish. The original design brief, with the scenarios and edge cases, is in [project.md](project.md).

## License

MIT. See [LICENSE](LICENSE).
