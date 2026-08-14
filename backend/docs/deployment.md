# Deployment Guide - Ethiopian Logistics Tracking Platform

This guide covers deploying the platform to a production environment using Docker Compose on a GCP VM.

## Prerequisites

- GCP account with a project configured
- VM instance (e2-standard-2 or larger recommended)
- Docker and Docker Compose installed
- Domain name (optional, but recommended for SSL)

## Step-by-Step Setup

### 1. Prepare the VM

```bash
# Update system packages
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Logout and login again for group changes to take effect
```

### 2. Clone and Configure

```bash
# Clone the repository
git clone <repository-url> /opt/logistics
cd /opt/logistics

# Copy environment file and configure
cp .env.example .env
nano .env
```

Update the following values in `.env`:
- `DB_PASSWORD` - Strong database password
- `GRAFANA_ADMIN_PASSWORD` - Strong Grafana admin password
- Any other values specific to your deployment

### 3. Generate JWT Keys

```bash
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

### 4. Deploy

```bash
cd deployments

# Start core services
docker-compose -f docker-compose.prod.yml up -d

# To include monitoring (Prometheus + Grafana)
docker-compose -f docker-compose.prod.yml --profile monitoring up -d
```

### 5. Verify Deployment

```bash
# Check container status
docker-compose -f docker-compose.prod.yml ps

# Check API health
curl http://localhost/health

# View logs
docker-compose -f docker-compose.prod.yml logs -f api
```

## SSL Configuration

### Using Certbot (Let's Encrypt)

1. Install certbot:
```bash
sudo apt install certbot
```

2. Obtain certificate:
```bash
sudo certbot certonly --standalone -d your-domain.com
```

3. Copy certificates:
```bash
mkdir -p deployments/nginx/ssl
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem deployments/nginx/ssl/cert.pem
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem deployments/nginx/ssl/key.pem
sudo chown -R $USER:$USER deployments/nginx/ssl
```

4. Update nginx.conf to enable HTTPS:
```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    
    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    
    # ... rest of your config
}

server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$host$request_uri;
}
```

5. Restart nginx:
```bash
docker-compose -f docker-compose.prod.yml restart nginx
```

### Auto-renewal

Add a cron job for automatic certificate renewal:
```bash
sudo crontab -e
```

Add:
```
0 3 * * * certbot renew --quiet && cp /etc/letsencrypt/live/your-domain.com/fullchain.pem /opt/logistics/deployments/nginx/ssl/cert.pem && cp /etc/letsencrypt/live/your-domain.com/privkey.pem /opt/logistics/deployments/nginx/ssl/key.pem && cd /opt/logistics/deployments && docker-compose -f docker-compose.prod.yml restart nginx
```

## Backup Setup

### Manual Backup

```bash
chmod +x scripts/backup.sh
./scripts/backup.sh
```

### Automated Backups

Add to crontab:
```bash
crontab -e
```

Add for daily backups at 2 AM:
```
0 2 * * * /opt/logistics/scripts/backup.sh >> /var/log/logistics-backup.log 2>&1
```

### Restore from Backup

```bash
# Decompress backup
gunzip logistics_20240101_020000.dump.gz

# Restore database
docker exec -i logistics-postgres pg_restore -U logistics -d logistics < logistics_20240101_020000.dump
```

## Monitoring

### Access Grafana

```bash
# With monitoring profile enabled
docker-compose -f docker-compose.prod.yml --profile monitoring up -d
```

Access Grafana at `http://your-server:3000`

Default credentials (change these):
- Username: admin
- Password: (value from GRAFANA_ADMIN_PASSWORD)

### Prometheus Metrics

Metrics are available at `http://your-server/metrics`

## Firewall Configuration

```bash
# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw enable
```

## Troubleshooting

### View logs
```bash
docker-compose -f docker-compose.prod.yml logs -f [service-name]
```

### Restart services
```bash
docker-compose -f docker-compose.prod.yml restart [service-name]
```

### Check container health
```bash
docker inspect logistics-api | grep -A 10 Health
```

### Database connection issues
```bash
docker exec -it logistics-postgres psql -U logistics -d logistics
```
