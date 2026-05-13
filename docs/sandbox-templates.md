# Sandbox Templates

Sandbox templates define the image and default runtime settings used when
creating Aone sandboxes. The CLI stores per-project template state in
`aone.sandbox.toml`.

## Start A New Template Project

```sh
aone sandbox template init --name my-app --language go --path ./my-app
cd my-app
```

Supported scaffold languages are `go`, `typescript`, and `python`.

The generated project includes an `aone.sandbox.toml` file:

```toml
template_name = "my-app"
dockerfile = "Dockerfile"
```

After the first successful build, the CLI writes the server-assigned
`template_id` back to the same file.

## Build A Dockerfile Template

Inside a template project:

```sh
aone sandbox template create my-app
```

`template create` is the normal user-facing build command. It looks for
`aone.Dockerfile` or `Dockerfile` by default, uploads the Dockerfile contents,
waits for the build to finish, and saves resolved template details to
`aone.sandbox.toml`.

Common overrides:

```sh
aone sandbox template create my-app \
  --dockerfile ./Dockerfile \
  --path . \
  --cmd "/usr/local/bin/my-app" \
  --ready-cmd "curl -fsS http://localhost:8080/health" \
  --cpu-count 2 \
  --memory-mb 2048 \
  --disk-size-mb 10240
```

Use `--public true` or `--public false` to set template visibility when the API
supports it.

The lower-level `aone sandbox template build` command still exists for internal
or advanced flows, but it is hidden from normal help output. Prefer
`template create` for Dockerfile projects.

## Project Config Lookup

Template commands that accept `--path` and `--config` resolve configuration in
this order:

1. `--config <file>` when provided.
2. `<path>/aone.sandbox.toml` when `--path` is provided.
3. `./aone.sandbox.toml` from the current directory.

Supported fields:

```toml
template_id = "tpl_xxx"
template_name = "my-app"
dockerfile = "Dockerfile"
start_cmd = ""
ready_cmd = ""
cpu_count = 0
memory_mb = 0
disk_size_mb = 0
public = false
```

Flag values win over config values. Missing flags are filled from the project
config when available.

## Create A Sandbox From The Template

After `template_id` has been written to `aone.sandbox.toml`, run:

```sh
aone sandbox create
```

When no template argument is provided, `sandbox create` reads `template_id` from
the project config. If no config or template ID is available, it falls back to
the `base` template.

Use `--detach` when you want the sandbox to keep running without opening an
interactive terminal:

```sh
aone sandbox create --detach
```

Useful runtime overrides:

```sh
aone sandbox create \
  --timeout 3600 \
  --env-var NODE_ENV=production \
  --metadata service=my-app,owner=platform \
  --allow-out api.example.com:443 \
  --deny-out 169.254.169.254
```

## Migrate An Existing Dockerfile

To convert an existing Dockerfile into SDK-native template code:

```sh
aone sandbox template migrate --language go --path .
```

Use `--language typescript` or `--language python` for other SDK targets, and
`--dockerfile` when the Dockerfile is not named `aone.Dockerfile` or
`Dockerfile`.

The generated code is a starting point. Review it before committing, especially
for readiness checks, environment variables, copied files, and commands that
depend on Docker-specific behavior.
