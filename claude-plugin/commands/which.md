---
description: Show which project is active and how it was resolved
---

Run `arc which` to display the currently active project and its resolution source.

Shows:
- The active project ID and name
- Where the project was resolved from (command line flag, local config, or server path match)
- The project config file path
- Any warnings about the configuration

Useful for debugging project resolution issues when commands target the wrong project.
