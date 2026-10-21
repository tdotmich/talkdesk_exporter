# Security

## Report a vulnerability

Do not open a public issue for a security problem.
Use GitHub private vulnerability reporting: **Security** tab > **Report a vulnerability**.

## How the exporter handles secrets

- `TALKDESK_CLIENT_ID` and `TALKDESK_CLIENT_SECRET` come only from environment variables.
  The config file rejects unknown fields, so a secret put in the file stops the exporter at start-up.
- The exporter never logs the client secret, the OAuth2 access token, or the Live stream URL
  (the stream URL contains a token).
- `/metrics` contains contact centre figures. Use `-web.config.file` for TLS and basic auth,
  or keep the port on a private network.
