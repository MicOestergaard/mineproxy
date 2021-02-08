# Minecraft Proxy

This is a simple proxy that allows you to expose a Minecraft LAN game on a fixed port. This can be useful if you want
to port forward incoming connections to your Minecraft game without updating the port forwarding entry in the router
every time you start a new LAN game. 

It works by listening on the LAN for a Minecraft game and then proxies TCP connections from a fixed port to the port the
game is running on.

The program can run on the same host as the Minecraft game or on another host on the same LAN.

## Usage

When the program is started without any arguments it defaults to listening on the default Minecraft port 25565.

      mineproxy

To use a different port run the program with the port number as an argument.

      mineproxy <port number>
