# DB Backup & Disaster Recovery

## Architecture

CarWatch uses a single SQLite database stored at the path configured in
`config.yaml` (`storage.db_path`, typically `/data/carwatch.db`).

In production the file lives inside a Docker named volume
(`carwatch_carwatch-data` → `/data`). This volume persists across container
restarts, image updates, and VM reboots.

## Automated daily backup

### 1. Install the cron job on the VM

```bash
# SSH into the VM
make vm-ssh

# Create backup directory inside the volume
docker exec carwatch mkdir -p /data/backups

# Add a daily cron job (runs at 03:00 local time)
(crontab -l 2>/dev/null; echo '0 3 * * * docker exec carwatch sqlite3 /data/carwatch.db ".backup /data/backups/carwatch-$(date +\%Y\%m\%d).db" && find /data/backups -name "carwatch-*.db" -mtime +7 -delete') | crontab -
```

The backup uses SQLite's `.backup` command, which is safe to run while the
application is writing (it uses the WAL to produce a consistent snapshot).

### 2. Verify the cron job

```bash
crontab -l    # should list the backup entry
```

### 3. Manual backup

```bash
# From your workstation
make vm-backup

# Or from the VM
docker exec carwatch sqlite3 /data/carwatch.db ".backup /data/backups/carwatch-manual.db"
```

### 4. List existing backups

```bash
make vm-backup-list
```

## Monitoring

### DB size in `/healthz`

The health endpoint includes `db_size_bytes` and `db_size_mb`:

```json
{
  "status": "ok",
  "db_size_bytes": 41943040,
  "db_size_mb": 40.0,
  ...
}
```

Monitor this value and set an alert if it approaches your disk limit.

### Disk space on the VM

```bash
make vm-ssh
df -h /data
```

## Disaster recovery

### Scenario 1: Container deleted, volume intact

The named volume survives `docker rm`. Just recreate the container:

```bash
make vm-deploy
```

### Scenario 2: Corrupt database

1. Stop the container:
   ```bash
   make vm-stop
   ```

2. Copy the latest backup over the corrupt file:
   ```bash
   make vm-ssh
   LATEST=$(ls -t /var/lib/docker/volumes/carwatch_carwatch-data/_data/backups/carwatch-*.db | head -1)
   cp "$LATEST" /var/lib/docker/volumes/carwatch_carwatch-data/_data/carwatch.db
   ```

3. Restart:
   ```bash
   make vm-start
   ```

### Scenario 3: VM destroyed (full rebuild)

1. Provision a new VM and install Docker.
2. Clone the repo and run `make vm-sync` to push the compose file.
3. Create the config:
   ```bash
   scp config.yaml firebase-sa.json <user>@<new-ip>:~/carwatch/
   ```
4. Restore the DB from an off-site backup (if available):
   ```bash
   scp carwatch-backup.db <user>@<new-ip>:~/carwatch/data/carwatch.db
   ```
5. Start:
   ```bash
   make vm-deploy
   ```

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
