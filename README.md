# Minecraft Proxy

This is a simple proxy that allows you to expose a Minecraft LAN game on a fixed port. This can be useful if you want
to port forward incoming connections to your Minecraft game without updating the port forwarding entry in the router
every time you start a new LAN game.

## Modes of Operation

### Automatic Discovery Mode (Default)
By default, the proxy listens on the LAN for Minecraft game announcements and automatically proxies connections to the detected game. This works by listening for multicast announcements and then proxying TCP connections from your chosen port to wherever the game is running.

### Manual Mode
You can also specify a target Minecraft server manually using the `-manual` flag. In this mode, you'll be prompted to enter the IP address and port of the Minecraft server you want to proxy to.

## Usage

The program supports various command-line flags to customize its behavior:

```bash
# Default mode: Listen on port 25565 with automatic LAN game discovery
mineproxy

# Use a different port (e.g., 25566) with automatic discovery
mineproxy -port 25566

# Manual mode: Manually specify target server, listening on default port
mineproxy -manual

# Manual mode with custom port
mineproxy -manual -port 25566
```

### Command-line Flags

- `-port`: Port number to listen on (default: 25565)
- `-manual`: Enable manual server input mode instead of automatic discovery
- `-help`: Display help message with all available options

## Network Configuration

The program can run on:
- The same host as the Minecraft game
- Another host on the same LAN

When using automatic discovery mode, ensure that multicast traffic (UDP port 4445) is allowed on your network for the proxy to detect LAN games.
