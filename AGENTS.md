# AGENTS.md

## Project overview

This repository is a customized fork of Jovvix, an open-source real-time quiz platform.

The project is being adapted primarily for classroom use with approximately 15–30 Android tablets on a local network.

The main goals are:

- reliable classroom operation;
- Russian-language UI;
- fully local/offline operation;
- teacher-hosted quizzes on a Mac;
- Android tablets used by students through a browser;
- support for Cyrillic and long student names;
- future support for interactive HTML5 educational content;
- easy deployment and backup to other Macs.

When explaining changes to the user, respond in Russian.
Keep code identifiers, filenames, function names, API names, and commands in their original language.

---

## Repository structure

Main areas:

- `app/` — frontend, based on Nuxt/Vue.
- `api/` — backend, written in Go.
- `api/database/migrations/` — database migrations.
- `api/pkg/kratos/` — Ory Kratos configuration.
- `docker-compose.yaml` — local Docker deployment.
- `load-test/` — load-testing related files.
- `docs/` — project documentation.

Before modifying unfamiliar functionality, inspect related frontend, backend, API, database, and WebSocket code instead of assuming behavior from a single file.

---

## Git remotes

The repository has two Git remotes:

- `origin` — customized fork:
  `git@github.com:germanstartiki-prog/jovVix.git`

- `upstream` — original Jovvix repository:
  `https://github.com/Improwised/jovVix.git`

Normal project changes should be committed to the user's fork (`origin`).

Do not push to `upstream`.

Before making significant changes, check:

```bash
git status
git diff

Do not discard existing local modifications unless explicitly requested.

Development environment

Primary development machine:

Apple Silicon Mac
ARM64
Docker Desktop
VS Code

The application is primarily run with Docker Compose.

The web application is currently exposed on host port:

5500

Port 5000 should not be assumed to be available on the host because macOS may use it for AirPlay Receiver / Control Center services.

Do not change the host web port back to 5000 without explicit instruction.

Docker / deployment notes

The upstream Docker configuration required modifications for ARM64/local deployment.

Important existing adaptations include:

ARM64-compatible frontend image/build.
PostgreSQL configuration fixes.
Redis configuration fixes.
Kratos development configuration changes.
local web port changed to 5500.
local API / frontend / Kratos URLs adjusted accordingly.

Be careful when comparing with upstream: some local Docker differences are intentional.

Do not blindly restore upstream Docker files.

Guest users and names

Guest/student names are an important customized area.

Requirements:

Cyrillic must work correctly.
Spaces in names must be preserved.
Names may be up to 50 characters.
Long compound names must be supported.
Unicode strings must never be truncated by raw byte offsets.
The visible student's name should be the human-entered name.
Internal unique usernames may contain generated suffixes.
Internal usernames must not be shown to students when a display name is available.

The database users.username field has been expanded from 12 to 50 characters.

Relevant migration:

api/database/migrations/20260830193000_expand_username_length.up.sql

and its corresponding .down.sql.

GenerateNewStringHavingSuffixName was changed to truncate Unicode safely using runes rather than byte slicing.

Relevant files include:

api/helpers/utils/generators.go
api/controllers/api/v1/auth_controller.go
api/controllers/api/v1/user_controller.go
app/pages/join/index.vue
Important unresolved design issue

Guest display name and backend/internal unique username should be treated as separate values.

A repeated guest session should reuse the authenticated/internal guest identity when appropriate, while still displaying the original human-readable first_name.

Before changing this logic, trace how the username is used in:

guest creation;
/user/who;
play-session state;
WebSocket connection parameters;
answer submission;
scoreboard / final-score lookup.

Do not assume that the visible name can safely replace the backend username.

Quiz termination / WebSocket handling

There was a bug where receiving a TerminateQuiz WebSocket event caused clients to call the host-only HTTP terminate endpoint.

This has been fixed.

Relevant files:

app/composables/user_operation.js
app/composables/admin_operation.js

Player behavior:

receiving TerminateQuiz should stop local ping / finish local state;
player must NOT call the host-only /quiz/terminate API.

Host behavior:

the host already terminates the quiz through the HTTP API;
receiving the resulting WebSocket event must not issue another terminate request.

Do not reintroduce duplicate terminate HTTP requests.

Some WebSocket close/error logging may still occur during normal connection shutdown.
Do not treat all close-time WebSocket console messages as functional failures without tracing the actual session behavior.

Classroom networking goals

The final classroom deployment should work on a local LAN without internet access.

Future networking goals include:

numeric local IP as the primary classroom address;
optional .local / mDNS support;
QR code based on the current reachable LAN address;
Android Chrome clients;
reliable reconnect behavior;
support for roughly 15–30 simultaneous tablets.

Do not introduce dependencies that require public internet access unless explicitly approved.

Offline requirements

The target deployment should eventually work fully offline.

Avoid adding:

CDN-only JavaScript dependencies;
Google Fonts loaded from the internet;
remote telemetry dependencies required for normal operation;
externally hosted assets required for gameplay.

Prefer locally bundled resources.

Existing external dependencies should be identified before being replaced.

Future product requirements

Planned or desired features include:

Russian UI / i18n;
sounds played only on the teacher computer;
podium / winners shown on student tablets;
HTML5 interactive content inside quiz questions;
sandboxed iframe execution;
potential ZIP-packaged HTML5 modules;
communication with interactive content through postMessage;
portable offline deployment to other Macs;
Docker image export/import and data backup scripts.

These are goals, not necessarily implemented features.

Do not claim they already exist unless verified in the code.

Working rules for Codex

Before editing:

Inspect relevant files and call sites.
Explain the intended change briefly.
Avoid unrelated refactoring.
Preserve existing working behavior.
Prefer minimal, reversible changes.

For potentially risky changes:

show or summarize the proposed diff;
do not perform destructive Git operations;
do not delete user data;
do not reset the repository;
do not modify database contents unless explicitly requested;
do not push commits unless explicitly requested.

When testing:

prefer existing project commands;
distinguish build/test failures caused by missing local generated files from actual application defects;
remember that Nuxt .nuxt/ content is generated and may be absent in the editor environment.

When completing a task, report:

files changed;
what behavior changed;
tests or commands run;
anything not verified;
any remaining risks.