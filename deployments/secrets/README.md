# Provider keys

The app sees this directory at `/run/secrets`, read-only. Put the files a
provider issues here, and point the settings in `.env` at them:

```
TELEBIRR_PRIVATE_KEY_FILE=/run/secrets/telebirr_private_key.pem
TELEBIRR_PUBLIC_KEY_FILE=/run/secrets/telebirr_public_key.pem
```

The app runs as user 10001, so give it the files and no one else:

```bash
sudo chown 10001 deployments/secrets/*.pem
sudo chmod 0400 deployments/secrets/*.pem
```

Nothing here but this README is committed (see `.gitignore`), and the
directory is outside the image (`.dockerignore`).
