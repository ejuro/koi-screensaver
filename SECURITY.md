# Security

Koi is a visual screensaver only; locking and authentication belong to Omarchy.

It does not record typed keys, passwords or mouse coordinates. It reads terminal input to dismiss the screensaver and writes a small local log containing timestamps and exit reasons, such as “key” or “mouse”. There is no telemetry or runtime network access. Git installation/updates and builds may access the network.

Like other Omarchy plugins, Koi runs unsandboxed as your user. It saves settings and temporarily changes the stock screensaver flag and cursor visibility. Cleanup can also close the stock screensaver; crash recovery is covered under [Remove](README.md#remove).
