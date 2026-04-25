#ifndef UAGENT_H_
#define UAGENT_H_

#include <Uefi.h>

#include <Library/BaseMemoryLib.h>
#include <Library/MemoryAllocationLib.h>
#include <Library/PrintLib.h>
#include <Library/UefiBootServicesTableLib.h>
#include <Library/UefiLib.h>
#include <Library/UefiRuntimeServicesTableLib.h>
#include <Protocol/LoadedImage.h>
#include <Protocol/PxeBaseCode.h>
#include <Protocol/ServiceBinding.h>
#include <Protocol/Tcp4.h>

#define UAGENT_VERSION         L"1.5.3"
#define REMOTE_PORT            8080
#define COMMAND_BUF_SIZE       128
#define RESPONSE_CHUNK_SIZE    512
#define COMMAND_HEADER_SIZE    3

typedef struct {
  EFI_HANDLE                    ServiceHandle;
  EFI_HANDLE                    ChildHandle;
  EFI_SERVICE_BINDING_PROTOCOL  *ServiceBinding;
  EFI_TCP4_PROTOCOL             *Tcp4;
  EFI_EVENT                     ConnectEvent;
  EFI_EVENT                     TransmitEvent;
  EFI_EVENT                     ReceiveEvent;
  EFI_EVENT                     CloseEvent;
  EFI_TCP4_CONNECTION_TOKEN     ConnectToken;
  EFI_TCP4_IO_TOKEN             TransmitToken;
  EFI_TCP4_IO_TOKEN             ReceiveToken;
  EFI_TCP4_CLOSE_TOKEN          CloseToken;
  UINT8                         ReceiveStash[RESPONSE_CHUNK_SIZE];
  UINTN                         ReceiveStashLength;
} SOCKET_CLIENT;

typedef enum {
  TcpSendText          = 1,
  TcpGetApps           = 2,
  TcpConnectSession    = 3,
  TcpOutputText        = 4,
  TcpDisconnectSession = 5,
  TcpPushEfiApp        = 6,
  TcpExecuteEfiApp     = 7,
  TcpEchoText          = 8,
  TcpMaxCommand,
} COMMAND_TYPE;

typedef struct {
  COMMAND_TYPE  Type;
  CHAR16        *Text;
  VOID          *Payload;
  UINTN         PayloadSize;
} TCP_COMMAND;

#define UAGENT_DEBUG_PROTOCOL_REVISION  0x00010000

typedef struct _UAGENT_DEBUG_PROTOCOL UAGENT_DEBUG_PROTOCOL;

#define UAGENT_DEBUG_PROTOCOL_GUID  { 0x1cce5d0b, 0x4a36, 0x4c7d, { 0x97, 0x71, 0x3f, 0x67, 0x2c, 0xc1, 0x4d, 0x92 } }

typedef
EFI_STATUS
(EFIAPI *UAGENT_SEND_DEBUG_MESSAGE)(
  IN UAGENT_DEBUG_PROTOCOL  *This,
  IN CONST CHAR16           *Message
  );

struct _UAGENT_DEBUG_PROTOCOL {
  UINT32                     Revision;
  UAGENT_SEND_DEBUG_MESSAGE  SendDebugMessage;
};

extern EFI_GUID  gUagentDebugProtocolGuid;

EFI_STATUS
InitSocketClient (
  IN  EFI_HANDLE     ImageHandle,
  OUT SOCKET_CLIENT  *Client
  );

EFI_STATUS
SendCommandPacket (
  IN SOCKET_CLIENT      *Client,
  IN CONST TCP_COMMAND  *Command
  );

EFI_STATUS
ReceiveCommandPacket (
  IN  SOCKET_CLIENT  *Client,
  OUT TCP_COMMAND    *Command
  );

VOID
FreeCommandPacket (
  IN OUT TCP_COMMAND  *Command
  );

EFI_STATUS
ReceiveAndPrintResponse (
  IN SOCKET_CLIENT  *Client
  );

VOID
CloseSocketClient (
  IN SOCKET_CLIENT  *Client
  );

EFI_STATUS
RunRemoteSession (
  IN EFI_HANDLE  ImageHandle
  );

EFI_STATUS
InstallUagentDebugProtocol (
  VOID
  );

VOID
UninstallUagentDebugProtocol (
  VOID
  );

VOID
SetUagentActiveClient (
  IN SOCKET_CLIENT  *Client
  );

EFI_STATUS
RunShell (
  IN EFI_HANDLE  ImageHandle
  );

#endif
