# Security

Koi is a visual screensaver only; locking and authentication belong to Omarchy.

Like other Omarchy plugins, it runs unsandboxed as your user. It launches terminals, saves its settings, temporarily changes the stock screensaver flag and cursor visibility, and writes an exit log without recording typed keys. Cleanup uses Omarchy's screensaver process-matching convention and can also close the stock saver. Crash cleanup is best effort; see [removal and recovery](README.md#update-and-remove).

Runtime code uses no network, telemetry or elevated privileges. Git installation/updates and Go builds may access the network.

Report vulnerabilities privately through the repository's **Security → Report a vulnerability** option when available. Otherwise ask [ejuro](https://github.com/ejuro) for a private contact route. Include the plugin commit, affected versions and reproduction steps; keep exploit details and personal data out of public issues.
