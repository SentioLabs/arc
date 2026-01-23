---
description: Manage the arc server daemon
argument-hint: start|stop|status|logs|restart
---

Manage the arc server with `arc server` subcommands.

**Start server:**
```bash
arc server start              # Start as daemon
arc server start --foreground # Run in foreground
arc server start --port 8080  # Custom port
```

**Other commands:**
```bash
arc server stop      # Stop the server
arc server status    # Check if running
arc server logs      # View server logs
arc server logs -f   # Follow logs
arc server restart   # Restart the server
```

**Data location:** `~/.arc/` (data.db, server.log, server.pid, cli-config.json)
