# Security

AprsMonitor holds credentials (SMTP password, webhook URLs) and makes only
outbound connections; it listens on no port.

- Keep `config.toml` readable only by the service user, or reference secrets as
  `env:NAME` so they stay out of the file (see docs/configuration.md).
- Webhook URLs are removed from error messages and logs.

**Reporting a vulnerability:** please use GitHub's private vulnerability
reporting on the repository (Security tab, "Report a vulnerability") rather than
a public issue.
