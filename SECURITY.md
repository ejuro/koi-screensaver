# Security

Koi is a visual screensaver only; locking and authentication belong to Omarchy.

It reads terminal input only to dismiss the screensaver. It does not record typed keys, passwords or mouse coordinates, and creates no activity log. There is no telemetry or runtime network access. Git installation/updates and builds may access the network.

Like other Omarchy plugins, Koi runs unsandboxed as your user. It saves settings and temporarily changes the stock screensaver flag and cursor visibility. Cleanup can also close the stock screensaver; crash recovery is covered under [Remove](README.md#remove).
