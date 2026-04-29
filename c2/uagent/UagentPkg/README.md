# UagentPkg

`UagentPkg` is a small EDK2 UEFI application intended to start after PXE boot, reuse the DHCP-provided IPv4 configuration, connect to a fixed server on TCP port `8080`, and provide a server-driven remote-control session.

## Current layout

- `Uagent.c`: UEFI entry point only.
- `Shell.c`: interactive shell, command parsing, and dispatch.
- `TcpClient.c`: PXE/TCP4 discovery, socket setup, transmit, receive, and cleanup.
- `Uagent.h`: shared internal constants, types, and function declarations.
- `Uagent.inf`: module definition.
- `UagentPkg.dsc`: package/platform build description.

## Startup behavior

- On boot, the app prints its version and disables the UEFI watchdog timer.
- On boot, the EFI app immediately connects out to the server and enters a persistent remote session loop.
- The server sends packets to EFI, and EFI sends result packets back.
- If the server session fails to start, the app falls back to the local shell.

## Go server

- The current server project is the Go implementation at `/home/alexa/Documents/SanderStuff/aau/cyber2/userve/uservego`.
- The Go shell prompt is `userve>`.
- `userve> echo test` does not send the literal text `echo test` as a `TcpSendText` packet.
- Instead, the Go server sends `TcpEchoText` with payload `test`.
- Raw commands that should be interpreted by the EFI-side command parser are sent with `TcpSendText`.

## Supported remote commands

- `TcpSendText` with `help`: show remote command help.
- `TcpSendText` with `status`: report that the remote session is active.
- `TcpSendText` with `echo <text>`: return text back to the server.
- `TcpSendText` with `disconnect`: close the remote session.
- `TcpSendText` with `reboot`: request a cold reboot through UEFI runtime services.
- `TcpEchoText`: return the packet text payload directly without parsing it as a shell command.
- `TcpGetApps`: currently returns `No local app registry implemented`.
- `TcpDisconnectSession`: close the remote session.
- `TcpPushEfiApp`: store an uploaded EFI application payload in memory for later execution.
- `TcpExecuteEfiApp`: load and start the most recently uploaded EFI application from memory.

## Local fallback shell

- `help`: show command list.
- `connect`: retry the remote session manually.
- `exit` or `quit`: leave the CLI.

## Build

From the EDK2 workspace root:

```sh
build -p UagentPkg/UagentPkg.dsc -m UagentPkg/Uagent.inf -a X64 -t GCC5
```

The generated EFI binary is written to:

`Build/UagentPkg/DEBUG_GCC5/X64/Uagent.efi`

## Notes

- The remote server address is currently hard-coded in `TcpClient.c` as `192.168.70.1:8080`.
- The application expects PXE DHCP to have already completed on the selected NIC.
- Binary requests use the packet format `[command:1][payload_length:2][payload:N]`.
- TCP is a byte stream, so header and payload may arrive in the same receive call.
- The EFI client now keeps a small internal receive stash so extra bytes are not lost between header and payload reads.
- Command IDs are:
  - `1 = TcpSendText`
  - `2 = TcpGetApps`
  - `3 = TcpConnectSession`
  - `4 = TcpOutputText`
  - `5 = TcpDisconnectSession`
  - `6 = TcpPushEfiApp`
  - `7 = TcpExecuteEfiApp`
  - `8 = TcpEchoText`
- The current `Shell.c` remote dispatcher actively handles `TcpSendText`, `TcpGetApps`, `TcpDisconnectSession`, `TcpEchoText`, `TcpPushEfiApp`, and `TcpExecuteEfiApp`.
- Uploaded EFI binaries are kept in memory for the duration of the remote session and executed with `LoadImage()` and `StartImage()`.




