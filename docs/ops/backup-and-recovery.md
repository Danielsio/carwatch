# DB Backup & Disaster Recovery

## Architecture

CarWatch supports two database backends:

- **PostgreSQL** (production) -- primary backend configured via `storage.postgres` in `config.yaml`
- **SQLite** (development/legacy) -- local file at `storage.db_path`

Production uses PostgreSQL running in Docker (`docker-compose.prod.yaml`) with a
named volume `carwatch_pgdata` for persistence.

## Automated daily backup

### PostgreSQL (production)

Backups are automated via a systemd timer that runs `scripts/backup-pg.sh` daily
at 03:00. The script uses `pg_dump` with gzip compression, names files with
timestamps (`carwatch-backup-YYYYMMDD-HHMMSS.sql.gz`), and prunes backups older
than 7 days.

#### 1. Install the backup timer on the VM

```bash
make vm-setup-backup
```

This copies the backup script and systemd units to the VM, enables the timer,
and shows the timer status.

#### 2. Verify the timer is active

```bash
make vm-backup-status
```

Or from the VM:

```bash
systemctl list-timers carwatch-backup.timer
journalctl -u carwatch-backup.service --since today
```

#### 3. Manual backup

From your workstation:

```bash
make vm-backup
```

Or from the VM:

```bash
~/carwatch/scripts/backup-pg.sh
```

#### 4. List existing backups

```bash
make vm-backup-list
```

### Systemd units

| File | Purpose |
|------|---------|
| `scripts/backup-pg.sh` | Backup script: pg_dump, compress, prune |
| `scripts/carwatch-backup.service` | Systemd oneshot service |
| `scripts/carwatch-backup.timer` | Systemd timer (daily at 03:00) |

### SQLite (development/legacy)

For local development with SQLite, use the legacy backup script:

```bash
scripts/backup-db.sh ./data/carwatch.db
```

Or SQLite's built-in `.backup` command:

```bash
sqlite3 ./data/carwatch.db ".backup ./data/backups/carwatch-manual.db"
```

## Monitoring

### Application health

Poll `/healthz` for synthetic uptime checks. The endpoint returns JSON with
status, uptime, cycle count, and database metrics.

### Database size

For PostgreSQL:

```bash
docker exec carwatch-pg psql -U carwatch -c "SELECT pg_size_pretty(pg_database_size('carwatch'));"
```

The `/healthz` endpoint also reports `db_size_bytes` when configured.

### Disk space on the VM

```bash
make vm-ssh
df -h
docker system df
```

## Disaster recovery

### Scenario 1: Container deleted, volume intact

The named volume survives `docker rm`. Just recreate the container:

```bash
make vm-deploy
```

### Scenario 2: Corrupt or lost database

1. Stop CarWatch:

   ```bash
   make vm-ssh
   docker compose -f docker-compose.prod.yaml down
   ```

2. Restore from the latest backup:

   ```bash
   docker compose -f docker-compose.prod.yaml up -d postgres
   # For .sql.gz backups (from backup-pg.sh):
   gunzip -c ~/carwatch/backups/carwatch-backup-YYYYMMDD-HHMMSS.sql.gz | docker exec -i carwatch-pg psql -U carwatch carwatch
   # For .dump backups (legacy pg_dump -Fc):
   docker exec -i carwatch-pg pg_restore -U carwatch -d carwatch --clean < ~/carwatch/backups/carwatch-YYYYMMDD.dump
   docker compose -f docker-compose.prod.yaml up -d
   ```

### Scenario 3: VM destroyed (full rebuild)

1. Provision a new VM and install Docker.
2. Clone the repo and run `make vm-sync` to push the compose file.
3. Create the config:

   ```bash
   scp config.yaml firebase-service-account.json <user>@<new-ip>:~/carwatch/
   ```

4. Start the stack (creates named volumes with empty DB):

   ```bash
   make vm-deploy
   ```

5. If you have a backup, restore it:

   ```bash
   scp carwatch-backup.sql.gz <user>@<new-ip>:~/carwatch/backups/
   ssh <user>@<new-ip> 'cd ~/carwatch && docker compose -f docker-compose.prod.yaml up -d postgres && sleep 5 && gunzip -c backups/carwatch-backup.sql.gz | docker exec -i carwatch-pg psql -U carwatch carwatch && docker compose -f docker-compose.prod.yaml up -d'
   ```

6. Set up the automated backup timer:

   ```bash
   make vm-setup-backup
   ```

### Scenario 4: No backup available

Starting with an empty database is safe. The application runs migrations
automatically. Users will need to re-create their searches, but the bot will
begin finding new listings immediately.

## Backup retention

| Scope | Retention | Location |
|-------|-----------|----------|
| Daily on-VM | 7 days | `~/carwatch/backups/` on the VM |

To add off-site backups (e.g. Oracle Object Storage, S3), extend the backup
script to upload after the local backup completes.
