#include "Uagent.h"

#include <Library/UefiApplicationEntryPoint.h>

EFI_GUID  gUagentDebugProtocolGuid = UAGENT_DEBUG_PROTOCOL_GUID;

STATIC SOCKET_CLIENT  *mActiveClient        = NULL;
STATIC EFI_HANDLE     mDebugProtocolHandle  = NULL;

STATIC
EFI_STATUS
EFIAPI
UagentSendDebugMessage (
  IN UAGENT_DEBUG_PROTOCOL  *This,
  IN CONST CHAR16           *Message
  )
{
  EFI_STATUS   Status;
  TCP_COMMAND  Command;

  if ((This == NULL) || (Message == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  if ((mActiveClient == NULL) || (mActiveClient->Tcp4 == NULL)) {
    return EFI_NOT_READY;
  }

  Command.Type        = TcpOutputText;
  Command.Text        = (CHAR16 *)Message;
  Command.Payload     = NULL;
  Command.PayloadSize = 0;

  Status = SendCommandPacket (mActiveClient, &Command);
  return Status;
}

STATIC UAGENT_DEBUG_PROTOCOL  mUagentDebugProtocol = {
  UAGENT_DEBUG_PROTOCOL_REVISION,
  UagentSendDebugMessage
};

EFI_STATUS
InstallUagentDebugProtocol (
  VOID
  )
{
  EFI_STATUS  Status;

  if (mDebugProtocolHandle != NULL) {
    return EFI_ALREADY_STARTED;
  }

  Status = gBS->InstallProtocolInterface (
                  &mDebugProtocolHandle,
                  &gUagentDebugProtocolGuid,
                  EFI_NATIVE_INTERFACE,
                  &mUagentDebugProtocol
                  );
  return Status;
}

VOID
UninstallUagentDebugProtocol (
  VOID
  )
{
  if (mDebugProtocolHandle == NULL) {
    return;
  }

  gBS->UninstallProtocolInterface (
         mDebugProtocolHandle,
         &gUagentDebugProtocolGuid,
         &mUagentDebugProtocol
         );
  mDebugProtocolHandle = NULL;
}

VOID
SetUagentActiveClient (
  IN SOCKET_CLIENT  *Client
  )
{
  mActiveClient = Client;
}

EFI_STATUS
EFIAPI
UefiMain (
  IN EFI_HANDLE        ImageHandle,
  IN EFI_SYSTEM_TABLE  *SystemTable
  )
{
  EFI_STATUS  Status;

  gBS->SetWatchdogTimer (0, 0, 0, NULL);

  Print (L"version: %s\n", UAGENT_VERSION);

  Status = RunRemoteSession (ImageHandle);
  if (!EFI_ERROR (Status)) {
    return EFI_SUCCESS;
  }

  Print (L"Remote session failed: %r\n", Status);
  Print (L"Falling back to local shell.\n");
  return RunShell (ImageHandle);
}
