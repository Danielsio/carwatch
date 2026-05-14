# DB Backup & Disaster Recovery

## Architecture

CarWatch supports two database backends:

- **PostgreSQL** (production) — primary backend configured via `storage.postgres` in `config.yaml`
- **SQLite** (development/legacy) — local file at `storage.db_path`

Production uses PostgreSQL running in Docker (`docker-compose.prod.yaml`) with a
named volume `carwatch_pgdata` for persistence.

## Automated daily backup

### PostgreSQL (production)

#### 1. Install the cron job on the VM

```bash
make vm-ssh

mkdir -p ~/carwatch/backups

# Add a daily cron job (runs at 03:00 local time)
(crontab -l 2>/dev/null; echo '0 3 * * * docker exec carwatch-pg pg_dump -U carwatch -Fc carwatch > ~/carwatch/backups/carwatch-$(date +\%Y\%m\%d).dump && find ~/carwatch/backups -name "carwatch-*.dump" -type f -mtime +7 -delete') | crontab -
```

#### 2. Verify the cron job

```bash
crontab -l    # should list the backup entry
```

#### 3. Manual backup

```bash
# From the VM
docker exec carwatch-pg pg_dump -U carwatch -Fc carwatch > ~/carwatch/backups/carwatch-manual.dump
```

#### 4. List existing backups

```bash
ls -lh ~/carwatch/backups/
```

### SQLite (development/legacy)

For local development with SQLite, use SQLite's `.backup` command:

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
   scp carwatch-backup.dump <user>@<new-ip>:~/carwatch/backups/
   ssh <user>@<new-ip> 'cd ~/carwatch && docker compose -f docker-compose.prod.yaml up -d postgres && sleep 5 && docker exec -i carwatch-pg pg_restore -U carwatch -d carwatch --clean < backups/carwatch-backup.dump && docker compose -f docker-compose.prod.yaml up -d'
   ```

### Scenario 4: No backup available

Starting with an empty database is safe. The application runs migrations
automatically. Users will need to re-create their searches, but the bot will
begin finding new listings immediately.

## Backup retention

| Scope | Retention | Location |
|-------|-----------|----------|
| Daily on-VM | 7 days | `~/carwatch/backups/` on the VM |

To add off-site backups (e.g. Oracle Object Storage, S3), extend the cron job
to upload after the local backup completes.
