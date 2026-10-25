# Inside the sandbox

Every task container starts with `ax-task-runner` as PID 1. On boot it:

1. Loads the `Task` and every bound `Workspace` spec.
2. Starts its servers: probes and guest management on port 80, and the metadata endpoints on `127.0.0.1:8099`.
3. On the first run, prepares each workspace at its own path, in binding order: clones Git repos, sets up the skills path, and, if the binding has a goal, hands that goal to an Antigravity agent to finish environment setup. The agent needs `GEMINI_API_KEY` in the container and gets 10 minutes by default; set `AX_BOOTSTRAP_TIMEOUT` to a Go duration to change that. The task reports not-ready until every workspace, including any agent run, has finished.
4. Starts `spec.command` as a child process, with the first workspace as its working directory and `AX_METADATA_URL` plus `spec.env` in its environment, and supervises it.

The runner stays up as PID 1 whether or not the command is still running, so the metadata server keeps answering and `ax ssh` still works after the command has exited. Its exit code is logged. When the sandbox is stopped or suspended, the runner sends the command's process group `SIGTERM`, waits ten seconds, and then kills whatever is left.

## Probes

Port 80 speaks HTTP/1.1 and `h2c`. Agent Substrate and the AX server use it to check on the task:

| Endpoint | Method | Returns | Description |
|---|---|---|---|
| `/healthz` | `GET` | `text/plain` | Liveness. Always `200 OK`. |
| `/readyz` | `GET` | `text/plain` | Readiness. `503` while the workspace is initializing, `200` once clones, MCP config, and skills are in place. |

## Metadata server

Your agent can introspect its own configuration without any SDK. The metadata endpoints listen on loopback only, at the address in `AX_METADATA_URL`, so they're reachable from inside the task and nowhere else:

| Endpoint | Method | Returns | Description |
|---|---|---|---|
| `/metadata/v1alpha1/ax/task` | `GET` | `application/yaml` | Task launch configuration, excluding status. |
| `/metadata/v1alpha1/ax/workspaces` | `GET` | `application/yaml` | Every bound `Workspace`, as a multi-document stream in binding order. |

```bash
# From inside a task:
curl -s "$AX_METADATA_URL/metadata/v1alpha1/ax/task"
```

## Guest services

When a task sets `spec.debug: true`, port 80 also serves the [Agent Substrate guest services](https://github.com/agent-substrate/env) over gRPC. They are off by default because they allow arbitrary process execution and file access inside the sandbox, and `ax ssh` refuses to connect to a task that has not enabled them.

- **Process service**: start, inspect, stream output from, and kill processes. This is what powers `ax ssh`.
- **File system service**: streaming file reads and writes inside the workspace.

## Environment provided to your command

| Variable | Value |
|---|---|
| `AX_METADATA_URL` | Base URL of the metadata server, `http://127.0.0.1:8099` by default |

## Capabilities

The task container's processes run with Agent Substrate's default set of Linux capabilities: `AUDIT_WRITE`, `KILL`, and `NET_BIND_SERVICE`. Root inside the sandbox can therefore not change the owner of a file or start a process as another user, so a command that wants to run untrusted tools as an unprivileged user cannot.

An operator can change the set for every task with two `ax-server` flags. Names are written without the `CAP_` prefix:

| Flag | Effect |
|---|---|
| `--task-capabilities-add` | Comma-separated capabilities granted on top of the default set, such as `CHOWN,DAC_OVERRIDE,SETGID,SETUID`. `ALL` is refused. |
| `--task-capabilities-drop` | Comma-separated capabilities removed from the default set. `ALL` removes the whole set, so `--task-capabilities-drop=ALL` with `--task-capabilities-add` gives an exact set. |

The server puts them in the `securityContext.capabilities` of the guest container of each task's ActorTemplate, and Substrate applies `drop` first, then `add`. The capabilities apply inside the gVisor sandbox, which stays the boundary between the task and the node. The flags are a setting of the server, not part of a task: a change reaches the templates of tasks created after it, and an existing task keeps its template. While the flags are set, a task whose template Substrate refuses fails with `TemplateCreationFailed` instead of falling back to the default template, which does not carry the capabilities.
