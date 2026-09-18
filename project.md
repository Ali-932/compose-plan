# Deploy preview and history for Docker Compose and Swarm

Working ProjectName: compose-plan. Language: Go.

## What it is

A command-line tool for teams that run their app with Docker Compose or Docker Swarm. Run it before a deploy and it prints what will change. Run it to deploy and it writes a diary entry. Later you can compare entries and roll back to any of them.

Docker itself has none of this. Its dry-run mode says "Recreate" without saying why. It remembers one previous version per service and forgets everything older.

## The problem

You edit the compose file and run one command. Docker restarts whatever it decides changed. You never see the list first. A tag like `latest` can point to a new image without the file changing, so a service restarts that you did not touch. After the deploy there is no record of what was running before, who deployed, or when. When production breaks an hour later, you guess.

People asked Docker for a preview in May 2017. The issue is still open with 34 thumbs-up and no reply from Docker.

## Scenarios

**The moved tag.** Tuesday, 4 pm. You raise the worker's memory limit and go to deploy. The preview shows the worker will restart for the memory change, and the web service will also restart because `latest` now points to a build CI pushed at 9:12. You did not know about that build. You pin web to the old image and deploy only the memory change.

**The Friday rollback.** Login breaks at 5 pm. You compare entry 41 and entry 42 and see two changes. You roll back to 41. Containers are back to the 3 pm state in under a minute. The bug report carries the exact image fingerprints.

**What is running right now.** A teammate asks what is in production. You print the history. Each entry has the time, the git commit, who deployed, and the exact image of every service.

**Someone changed prod by hand.** A colleague ran a manual update on the server last night. Your next preview says the live state no longer matches entry 42 and shows the difference before you deploy on top of it.

## Imagined output

```
$ compose-plan plan
worker   restart    memory limit 512M -> 1G
web      restart    image "latest" moved:
                    sha256:8f3a... (Sep 8) -> sha256:c21d... (today 09:12, CI)
db       no change
redis    no change
env      1 value changed in web (DATABASE_URL), value hidden

$ compose-plan apply
deployed entry #42  2026-09-15 16:04  ali  git 3e1f9a0  4 services

$ compose-plan history
#42  Sep 15 16:04  ali    3e1f9a0  worker memory, web image
#41  Sep 15 15:02  ali    9b7c2d1  web image
#40  Sep 12 11:30  sara   1a2b3c4  add redis

$ compose-plan diff 41 42
worker  memory limit   512M -> 1G
web     image          sha256:8f3a... -> sha256:c21d...

$ compose-plan rollback 41
web     image   sha256:c21d... -> sha256:8f3a...
worker  memory  1G -> 512M
done in 48s. recorded as entry #43 (rollback of #42 to #41)
```

## The result

No surprise restarts. Every deploy is written down with exact image fingerprints. Going back to any past deploy is one command. You stop guessing.

## Edge cases

- A tag moved on the registry but the running container still has the old image. That is the main case. Show both fingerprints and the push date.
- The registry needs a login. Use the credentials Docker already stores. If the tool still cannot ask the registry, say "could not check, Docker will pull at deploy" instead of guessing.
- Docker Hub rate limits, or the registry is down. Same answer. Never pretend to know.
- Secrets and environment values. Show that a value changed. Never show the value. A short hash is enough to tell two values apart.
- Variables in the file like `${PORT}` and `.env` files. Render the file the way Compose does, with the Compose library, so the comparison matches what Docker will actually do.
- Several compose files stacked with `-f`. Accept the same flags as Compose.
- A service restarts for a reason outside its own block, such as a changed network or volume, or a dependency restarting. Say "restart, because volume data changed."
- Compose and Swarm store state differently. Compose keeps a config hash in container labels. Swarm keeps the full spec plus one previous spec. Two backends, one output.
- Rolling back to an image the registry has since deleted. Check before starting and refuse with a clear message.
- Database migrations. The tool restores containers, not data. Print that sentence on every rollback.
- Two people deploy at once. Take a lock file on the deploy machine.
- Someone edits the history file. Chain each entry to the previous one with a hash so tampering shows. No signing in version one.
- Fifty services. Group the output by status and put "no change" last, collapsed.
- The deploy machine is not the laptop. Version one keeps the history on whichever machine runs the tool. Committing it to git is the user's choice.

## Implementation stack

- Go, single binary. Install with `go install`, Homebrew, or a release download.
- Docker Engine API through the official Go client.
- The Compose Go library (`compose-go`) to render files the way Compose does.
- `go-containerregistry` to turn a tag into a fingerprint.
- History as a JSON-lines file with a hash chain. SQLite only if querying gets slow.
- Tests run against a throwaway Compose project and a local registry in CI.

Version one is Compose only, with `plan`, `apply`, `history`, `diff`, and `rollback`. Swarm comes second.
