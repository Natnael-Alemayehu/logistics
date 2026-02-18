#!/bin/bash
set -e

BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/logistics_${DATE}.dump"

mkdir -p ${BACKUP_DIR}

docker exec logistics-postgres pg_dump -U logistics -Fc logistics >${BACKUP_FILE}

gzip ${BACKUP_FILE}

find ${BACKUP_DIR} -name "*.dump.gz" -mtime +7 -delete

echo "Backup completed: ${BACKUP_FILE}.gz"
