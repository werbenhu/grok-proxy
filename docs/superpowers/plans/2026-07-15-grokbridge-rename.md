# GrokBridge Complete Rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace every project-owned `Grok2API` identifier with `GrokBridge`, bind `origin` to `werbenhu/grokbridge`, verify the application, and commit all current workspace changes.

**Architecture:** Apply one deterministic case-aware rename across version-controlled text, then rename project-owned paths. Preserve Git history and license text, while intentionally dropping compatibility for old environment variables, runtime keys, Docker names, binaries, headers, and tool prefixes.

**Tech Stack:** Go 1.26+, React 19, TypeScript 6, Vite 8, pnpm, Docker Compose, Git, PowerShell

## Global Constraints

- Display name is exactly `GrokBridge`.
- Lowercase identifier is exactly `grokbridge`.
- Uppercase identifier is exactly `GROKBRIDGE`.
- Go module is exactly `github.com/werbenhu/grokbridge/backend`.
- Git remote is exactly `https://github.com/werbenhu/grokbridge.git`.
- Do not preserve aliases for old project identifiers.
- Preserve and include the current `.gitignore`, `.gitattributes`, `_init_secrets.ps1`, `make.bat`, and `start.bat` work.
- Do not rewrite Git history or license text.
- Create one implementation commit after verification; do not push.

---

### Task 1: Rename project identifiers and paths

**Files:**
- Rename: `backend/cmd/grok2api/` → `backend/cmd/grokbridge/`
- Rename: `frontend/public/grok2api.png` → `frontend/public/grokbridge.png`
- Modify: every tracked text file returned by `git grep -Il -e Grok2API -e grok2api -e GROK2API -e chenyme/grok2api`
- Modify: `.gitattributes`, `_init_secrets.ps1`, `make.bat`, `start.bat`
- Create: `docs/superpowers/plans/2026-07-15-grokbridge-rename.md`

**Interfaces:**
- Consumes: the naming map and scope from `docs/superpowers/specs/2026-07-15-grokbridge-rename-design.md`
- Produces: Go imports under `github.com/werbenhu/grokbridge/backend`, the `grokbridge` executable and paths, and `GROKBRIDGE_*` runtime variables

- [ ] **Step 1: Capture the pre-change inventory**

Run:

```powershell
git status --short
git grep -Il -e Grok2API -e grok2api -e GROK2API -e chenyme/grok2api
```

Expected: the status contains the five user-owned pending files, and the grep lists the current project identifiers.

- [ ] **Step 2: Apply the case-aware text replacement**

For all tracked and non-ignored untracked text files, replace in this order:

```text
github.com/chenyme/grok2api/backend → github.com/werbenhu/grokbridge/backend
github.com/chenyme/grok2api         → github.com/werbenhu/grokbridge
chenyme/grok2api                    → werbenhu/grokbridge
GROK2API                            → GROKBRIDGE
Grok2API                            → GrokBridge
grok2api                            → grokbridge
```

Use a UTF-8-preserving bulk mechanical rewrite over the file list from `git ls-files` plus `git ls-files --others --exclude-standard`. Skip binary files.

- [ ] **Step 3: Rename project-owned paths**

First verify both source paths resolve inside `E:\github\grok2api` and both targets do not exist. Then run:

```powershell
git mv backend/cmd/grok2api backend/cmd/grokbridge
git mv frontend/public/grok2api.png frontend/public/grokbridge.png
```

Expected: Git records renames and all build references point at the new paths.

- [ ] **Step 4: Format and inspect the mechanical changes**

Run:

```powershell
Set-Location backend
gofmt -w (Get-ChildItem -Recurse -Filter *.go | ForEach-Object FullName)
Set-Location ..
git diff --check
git diff --stat
```

Expected: `git diff --check` exits 0 and the diff contains only the planned rename plus preserved user changes.

### Task 2: Verify all renamed surfaces

**Files:**
- Test: `backend/**/*_test.go`
- Test: `frontend/package.json`
- Test: `docker-compose.yml`
- Inspect: all tracked and non-ignored untracked files

**Interfaces:**
- Consumes: renamed module, command path, runtime variables, build metadata, Docker configuration, and frontend package
- Produces: evidence that no old project identifier remains and build/test entry points still work

- [ ] **Step 1: Prove the old names are absent**

Run a case-insensitive scan across tracked and non-ignored untracked text files for `grok2api` and `chenyme/grok2api`, excluding the design and implementation plan because those documents intentionally describe the migration.

Expected: zero matches outside `docs/superpowers/specs/2026-07-15-grokbridge-rename-design.md` and `docs/superpowers/plans/2026-07-15-grokbridge-rename.md`.

- [ ] **Step 2: Run the backend suite**

Run:

```powershell
Set-Location backend
go test ./...
```

Expected: exit code 0 and no failing Go package.

- [ ] **Step 3: Run frontend lint and production build**

Run:

```powershell
Set-Location frontend
npx --yes pnpm@9.15.9 lint
npx --yes pnpm@9.15.9 build
```

Expected: both commands exit 0; Vite writes the production bundle to `frontend/dist`.

- [ ] **Step 4: Validate Docker Compose**

Run:

```powershell
docker compose config --quiet
```

Expected: exit code 0. If Docker is unavailable, report that limitation explicitly without treating it as a successful check.

### Task 3: Bind the remote and create the implementation commit

**Files:**
- Modify: repository-local Git configuration for `origin`
- Commit: all tracked and non-ignored untracked workspace changes

**Interfaces:**
- Consumes: verified working tree from Tasks 1 and 2
- Produces: `origin=https://github.com/werbenhu/grokbridge.git` and one local implementation commit

- [ ] **Step 1: Bind and verify `origin`**

Run:

```powershell
git remote set-url origin https://github.com/werbenhu/grokbridge.git
git remote -v
```

Expected: fetch and push URLs both equal `https://github.com/werbenhu/grokbridge.git`.

- [ ] **Step 2: Review and stage the complete implementation**

Run:

```powershell
git status --short
git diff --check
git diff --stat
git add -A
git diff --cached --check
git diff --cached --stat
```

Expected: all intended rename files and the five pre-existing user changes are staged; ignored runtime configuration and build output are absent.

- [ ] **Step 3: Commit**

Run:

```powershell
git commit -m "refactor: rename project to GrokBridge"
```

Expected: Git creates one commit containing the complete implementation.

- [ ] **Step 4: Verify the final repository state**

Run:

```powershell
git status --short --branch
git log -2 --oneline --decorate
git remote -v
```

Expected: working tree is clean, the latest commit is the implementation commit, and `origin` points to `werbenhu/grokbridge`.
