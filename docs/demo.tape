# Renders docs/demo.gif. Run from the repo root with Docker running: vhs docs/demo.tape
Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 640
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

# Setup: fresh build, a throwaway git repo in /tmp/demo, images pulled ahead.
Hide
Type "go build -o /tmp/cp-bin/compose-plan ./cmd/compose-plan && export PATH=/tmp/cp-bin:$PATH PS1='$ ' GIT_AUTHOR_NAME=ali GIT_COMMITTER_NAME=ali EMAIL=ali@example.com" Enter
Type "rm -rf /tmp/demo && mkdir /tmp/demo && cd /tmp/demo && git init -q" Enter
Type `printf 'services:\n  web:\n    image: nginx:1.26-alpine\n  cache:\n    image: redis:7-alpine\n' > compose.yaml && git add . && git commit -qm v1` Enter
Type "docker pull -q nginx:1.26-alpine && docker pull -q nginx:1.27-alpine && docker pull -q redis:7-alpine && clear" Enter
Sleep 20s
Show

Type "cat compose.yaml" Sleep 300ms Enter Sleep 2s
Type "compose-plan plan" Sleep 300ms Enter Sleep 3s
Type "compose-plan apply 2>/dev/null" Sleep 300ms Enter Sleep 5s

Type "sed -i 's/1.26/1.27/' compose.yaml && git commit -qam 'bump nginx'" Sleep 300ms Enter Sleep 1s
Type "compose-plan plan" Sleep 300ms Enter Sleep 3s
Type "compose-plan apply 2>/dev/null" Sleep 300ms Enter Sleep 5s

Type "compose-plan history" Sleep 300ms Enter Sleep 2s
Type "compose-plan diff 1 2" Sleep 300ms Enter Sleep 3s
Type "compose-plan rollback 1 2>/dev/null" Sleep 300ms Enter Sleep 7s

Hide
Type "docker compose down -q 2>/dev/null; docker compose down >/dev/null 2>&1" Enter
Sleep 5s
