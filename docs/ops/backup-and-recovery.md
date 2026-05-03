# DB Backup & Disaster Recovery

## Architecture

CarWatch uses a single SQLite database stored at the path configured in
`config.yaml` (`storage.db_path`). If `db_path` is omitted, the binary defaults to
`./data/dedup.db` (see `internal/config`). `config.example.yaml` uses
`./data/carwatch.db`—substitute the path your config actually uses in the backup
commands below. Production compose mounts the named volume at `/data`, so the
live DB is typically `/data/carwatch.db`.

In production the file lives inside a Docker named volume
(`carwatch_carwatch-data` → `/data`). This volume persists across container
restarts, image updates, and VM reboots.

**Restore destination:** Commands below copy into `/data/carwatch.db` because that
matches the default production compose layout. If your `storage.db_path` is
different inside the container, use that path as the **target** of every `cp`
(and adjust `sqlite3` paths in backups/cron the same way).

## Automated daily backup

### 1. Install the cron job on the VM

```bash
# SSH into the VM
make vm-ssh

# Create backup directory inside the volume
docker exec carwatch mkdir -p /data/backups

# Add a daily cron job (runs at 03:00 local time)
(crontab -l 2>/dev/null; echo '0 3 * * * docker exec carwatch sqlite3 /data/carwatch.db ".backup /data/backups/carwatch-$(date +\%Y\%m\%d).db" && docker exec carwatch find /data/backups -name "carwatch-*.db" -type f -mtime +7 -delete') | crontab -
```

If `storage.db_path` in production is not `/data/carwatch.db`, substitute that
path in the `sqlite3` argument above.

The backup uses SQLite's `.backup` command, which is safe to run while the
application is writing (it uses the WAL to produce a consistent snapshot).

### 2. Verify the cron job

```bash
crontab -l    # should list the backup entry
```

### 3. Manual backup

```bash
# From your workstation (runs over SSH via repo Makefile → vm-check-env + ssh)
make vm-backup

# Or from the VM
docker exec carwatch sqlite3 /data/carwatch.db ".backup /data/backups/carwatch-manual.db"
```

Targets `vm-backup` and `vm-backup-list` live in the **repository root**
[`Makefile`](../../Makefile) alongside `vm-ssh` / `vm-deploy`; they are not
shown in snippets elsewhere in this doc.

### 4. List existing backups

```bash
make vm-backup-list
```

## Monitoring

### Application health

Poll `/healthz` for synthetic uptime checks.

### Database file size

Watch growth via the file inside the volume (alert if it nears disk limits):

```bash
docker exec carwatch ls -lh /data/carwatch.db
```

Some deployments also expose size fields on `/healthz` when the health handler
is configured with a database sizer; rely on whatever JSON your running image
returns.

### Disk space on the VM

```bash
make vm-ssh
docker exec carwatch df -h /data
```

## Disaster recovery

### Scenario 1: Container deleted, volume intact

The named volume survives `docker rm`. Just recreate the container:

```bash
make vm-deploy
```

### Scenario 2: Corrupt database

Stop CarWatch before overwriting the primary DB file so SQLite does not keep a
stale WAL handle open, then copy the newest backup into the named volume and
start again:

1. Restore from the latest on-volume backup (adjust `/data/carwatch.db` if your
   `storage.db_path` differs):

   ```bash
   make vm-ssh
   docker stop carwatch
   docker run --rm -v carwatch_carwatch-data:/data alpine sh -c 'cp "$(ls -t /data/backups/carwatch-*.db | head -1)" /data/carwatch.db'
   docker start carwatch
   ```

   The first run may pull the small `alpine` image once.

2. If you cannot use a helper container, you can instead stop the app,
   replace `/data/carwatch.db` via any method that writes into volume
   `carwatch_carwatch-data`, then start CarWatch.

### Scenario 3: VM destroyed (full rebuild)

1. Provision a new VM and install Docker.
2. Clone the repo and run `make vm-sync` to push the compose file.
3. Create the config:

   ```bash
   scp config.yaml firebase-sa.json <user>@<new-ip>:~/carwatch/
   ```

4. Start the container (creates the named volume with an empty DB):

   ```bash
   make vm-deploy
   ```

5. If you have an off-site backup, stop CarWatch before overwriting the DB file
   (avoid restoring while SQLite has the file open), copy into the volume, then
   start. Use your real `storage.db_path` inside the container instead of
   `/data/carwatch.db` when it differs:

   ```bash
   ssh <user>@<new-ip> 'docker stop carwatch'
   scp carwatch-backup.db <user>@<new-ip>:/tmp/
   ssh <user>@<new-ip> 'docker run --rm -v carwatch_carwatch-data:/data -v /tmp:/backup alpine sh -c "cp /backup/carwatch-backup.db /data/carwatch.db" && rm /tmp/carwatch-backup.db'
   ssh <user>@<new-ip> 'docker start carwatch'
   ```

   From your workstation you can use `make vm-stop` / `make vm-start` instead if
   those targets match how you manage the VM.

### Scenario 4: No backup available

If no backup exists, starting with an empty database is safe. The application
will run migrations automatically. Users will need to re-create their
searches, but the bot will begin finding new listings immediately.

## Backup retention

| Scope | Retention | Location |
|-------|-----------|----------|
| Daily on-volume | 7 days | `/data/backups/` inside the Docker volume |

To add off-site backups (e.g. Oracle Object Storage, S3), extend the cron job
to upload after the local backup completes.
