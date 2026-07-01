# Atlas config for versioned migrations.
#
# The desired schema state is loaded directly from the Ent definitions via
# Atlas's native `ent://` loader — no external provider needed. Run Atlas
# commands with `--env local` (the task wrappers set GOWORK=off, required
# because the ent:// loader shells out with -mod=mod which Go forbids while
# the go.work workspace is active).
#
# Note: these are local-dev credentials (same as docker-compose.yml). For real
# environments, use env-var interpolation instead of literals.
env "local" {
  # Desired state: the Ent schema.
  src = "ent://ent/schema"

  # Scratch database Atlas uses to compute the diff (ephemeral Docker Postgres).
  dev = "docker://postgres/17/dev?search_path=public"

  # Target: the Dockerized Postgres (host-mapped to 5433).
  url = "postgres://medmarket:medmarket@localhost:5433/medmarket?sslmode=disable"

  migration {
    dir = "file://ent/migrate/migrations"
  }
}
