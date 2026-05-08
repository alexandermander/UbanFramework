#include "Uagent.h"

STATIC
VOID
EFIAPI
Tcp4TokenNotify (
  IN EFI_EVENT  Event,
  IN VOID       *Context
  )
{
  (VOID)Event;
  (VOID)Context;
}

STATIC
EFI_STATUS
WaitForTcp4Token (
  IN EFI_TCP4_PROTOCOL          *Tcp4,
  IN EFI_TCP4_COMPLETION_TOKEN  *CompletionToken
  )
{
  if ((Tcp4 == NULL) || (CompletionToken == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  while (CompletionToken->Status == EFI_NOT_READY) {
    Tcp4->Poll (Tcp4);
    gBS->Stall (1000);
  }

  return CompletionToken->Status;
}

EFI_STATUS
InitSocketClient (
  IN  EFI_HANDLE     ImageHandle,
  OUT SOCKET_CLIENT  *Client
  )
{
  EFI_STATUS                  Status;
  EFI_LOADED_IMAGE_PROTOCOL   *LoadedImage;
  EFI_PXE_BASE_CODE_PROTOCOL  *PxeBaseCode;
  EFI_HANDLE                  *HandleBuffer;
  EFI_HANDLE                  ControllerHandle;
  UINTN                       HandleCount;
  UINTN                       HandleIndex;
  EFI_TCP4_CONFIG_DATA        ConfigData;
  EFI_IPv4_ADDRESS            RemoteAddress;
  EFI_IPv4_ADDRESS            StationAddress;
  EFI_IPv4_ADDRESS            SubnetMask;
  BOOLEAN                     FoundPxeHandle;

  if (Client == NULL) {
    return EFI_INVALID_PARAMETER;
  }

  ZeroMem (Client, sizeof (*Client));
  HandleBuffer     = NULL;
  ControllerHandle = NULL;
  HandleCount      = 0;
  FoundPxeHandle   = FALSE;

  ZeroMem (&StationAddress, sizeof (StationAddress));
  ZeroMem (&SubnetMask, sizeof (SubnetMask));
  ZeroMem (&RemoteAddress, sizeof (RemoteAddress));
  ZeroMem (&ConfigData, sizeof (ConfigData));

  Status = gBS->OpenProtocol (
                  ImageHandle,
                  &gEfiLoadedImageProtocolGuid,
                  (VOID **)&LoadedImage,
                  ImageHandle,
                  NULL,
                  EFI_OPEN_PROTOCOL_GET_PROTOCOL
                  );
  if (!EFI_ERROR (Status) && (LoadedImage->DeviceHandle != NULL)) {
    Status = gBS->OpenProtocol (
                    LoadedImage->DeviceHandle,
                    &gEfiPxeBaseCodeProtocolGuid,
                    (VOID **)&PxeBaseCode,
                    ImageHandle,
                    NULL,
                    EFI_OPEN_PROTOCOL_GET_PROTOCOL
                    );
    if (!EFI_ERROR (Status) &&
        (PxeBaseCode->Mode != NULL) &&
        PxeBaseCode->Mode->Started &&
        !PxeBaseCode->Mode->UsingIpv6 &&
        PxeBaseCode->Mode->DhcpAckReceived)
    {
      ControllerHandle = LoadedImage->DeviceHandle;
      CopyMem (&StationAddress, &PxeBaseCode->Mode->StationIp.v4, sizeof (StationAddress));
      CopyMem (&SubnetMask, &PxeBaseCode->Mode->SubnetMask.v4, sizeof (SubnetMask));
      FoundPxeHandle = TRUE;
    }
  }

  if (!FoundPxeHandle) {
    Status = gBS->LocateHandleBuffer (
                    ByProtocol,
                    &gEfiPxeBaseCodeProtocolGuid,
                    NULL,
                    &HandleCount,
                    &HandleBuffer
                    );
    if (EFI_ERROR (Status) || (HandleCount == 0)) {
      Print (L"InitSocketClient: no PXE base code handles: %r\n", Status);
      return EFI_NOT_FOUND;
    }

    for (HandleIndex = 0; HandleIndex < HandleCount; HandleIndex++) {
      Status = gBS->OpenProtocol (
                      HandleBuffer[HandleIndex],
                      &gEfiPxeBaseCodeProtocolGuid,
                      (VOID **)&PxeBaseCode,
                      ImageHandle,
                      NULL,
                      EFI_OPEN_PROTOCOL_GET_PROTOCOL
                      );
      if (EFI_ERROR (Status) || (PxeBaseCode->Mode == NULL)) {
        continue;
      }

      if (PxeBaseCode->Mode->Started &&
          !PxeBaseCode->Mode->UsingIpv6 &&
          PxeBaseCode->Mode->DhcpAckReceived)
      {
        ControllerHandle = HandleBuffer[HandleIndex];
        CopyMem (&StationAddress, &PxeBaseCode->Mode->StationIp.v4, sizeof (StationAddress));
        CopyMem (&SubnetMask, &PxeBaseCode->Mode->SubnetMask.v4, sizeof (SubnetMask));
        FoundPxeHandle = TRUE;
        break;
      }
    }

    FreePool (HandleBuffer);
    if (!FoundPxeHandle) {
      Print (L"InitSocketClient: no PXE handle with IPv4 DHCP state\n");
      return EFI_NOT_FOUND;
    }
  }

  Client->ServiceHandle = ControllerHandle;

  Status = gBS->OpenProtocol (
                  Client->ServiceHandle,
                  &gEfiTcp4ServiceBindingProtocolGuid,
                  (VOID **)&Client->ServiceBinding,
                  ImageHandle,
                  NULL,
                  EFI_OPEN_PROTOCOL_GET_PROTOCOL
                  );
  if (EFI_ERROR (Status)) {
    Print (L"InitSocketClient: Open TCP4 service binding failed: %r\n", Status);
    goto Error;
  }

  Status = Client->ServiceBinding->CreateChild (Client->ServiceBinding, &Client->ChildHandle);
  if (EFI_ERROR (Status)) {
    Print (L"InitSocketClient: TCP4 CreateChild failed: %r\n", Status);
    goto Error;
  }

  Status = gBS->OpenProtocol (
                  Client->ChildHandle,
                  &gEfiTcp4ProtocolGuid,
                  (VOID **)&Client->Tcp4,
                  ImageHandle,
                  NULL,
                  EFI_OPEN_PROTOCOL_GET_PROTOCOL
                  );
  if (EFI_ERROR (Status)) {
    Print (L"InitSocketClient: Open TCP4 protocol failed: %r\n", Status);
    goto Error;
  }

  Status = gBS->CreateEvent (EVT_NOTIFY_SIGNAL, TPL_CALLBACK, Tcp4TokenNotify, NULL, &Client->ConnectEvent);
  if (EFI_ERROR (Status)) {
    goto Error;
  }

  Status = gBS->CreateEvent (EVT_NOTIFY_SIGNAL, TPL_CALLBACK, Tcp4TokenNotify, NULL, &Client->TransmitEvent);
  if (EFI_ERROR (Status)) {
    goto Error;
  }

  Status = gBS->CreateEvent (EVT_NOTIFY_SIGNAL, TPL_CALLBACK, Tcp4TokenNotify, NULL, &Client->ReceiveEvent);
  if (EFI_ERROR (Status)) {
    goto Error;
  }

  Status = gBS->CreateEvent (EVT_NOTIFY_SIGNAL, TPL_CALLBACK, Tcp4TokenNotify, NULL, &Client->CloseEvent);
  if (EFI_ERROR (Status)) {
    goto Error;
  }

  RemoteAddress.Addr[0] = 192;
  RemoteAddress.Addr[1] = 168;
  RemoteAddress.Addr[2] = 70;
  RemoteAddress.Addr[3] = 1;

  ConfigData.TypeOfService                 = 0;
  ConfigData.TimeToLive                    = 64;
  ConfigData.AccessPoint.UseDefaultAddress = FALSE;
  ConfigData.AccessPoint.ActiveFlag        = TRUE;
  ConfigData.AccessPoint.StationPort       = 0;
  ConfigData.AccessPoint.RemotePort        = REMOTE_PORT;
  CopyMem (&ConfigData.AccessPoint.StationAddress, &StationAddress, sizeof (EFI_IPv4_ADDRESS));
  CopyMem (&ConfigData.AccessPoint.SubnetMask, &SubnetMask, sizeof (EFI_IPv4_ADDRESS));
  CopyMem (&ConfigData.AccessPoint.RemoteAddress, &RemoteAddress, sizeof (EFI_IPv4_ADDRESS));

  Status = Client->Tcp4->Configure (Client->Tcp4, &ConfigData);
  if (EFI_ERROR (Status)) {
    Print (L"InitSocketClient: TCP4 configure failed: %r\n", Status);
    goto Error;
  }

  ZeroMem (&Client->ConnectToken, sizeof (Client->ConnectToken));
  Client->ConnectToken.CompletionToken.Event  = Client->ConnectEvent;
  Client->ConnectToken.CompletionToken.Status = EFI_NOT_READY;

  Status = Client->Tcp4->Connect (Client->Tcp4, &Client->ConnectToken);
  if (EFI_ERROR (Status)) {
    Print (L"InitSocketClient: TCP4 connect start failed: %r\n", Status);
    goto Error;
  }

  Status = WaitForTcp4Token (Client->Tcp4, &Client->ConnectToken.CompletionToken);
  if (!EFI_ERROR (Status)) {
    return Status;
  }

  Print (L"InitSocketClient: TCP4 connect completion failed: %r\n", Status);

Error:
  CloseSocketClient (Client);
  return Status;
}

EFI_STATUS
SendCommandPacket (
  IN SOCKET_CLIENT       *Client,
  IN CONST TCP_COMMAND   *Command
  )
{
  EFI_STATUS              Status;
  EFI_TCP4_TRANSMIT_DATA  *TransmitData;
  UINT8                   *PacketBuffer;
  UINTN                   AllocationSize;
  UINTN                   PayloadLength;
  VOID                    *PayloadBuffer;

  if ((Client == NULL) || (Client->Tcp4 == NULL) || (Command == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  if ((Command->Type == 0) || (Command->Type >= TcpMaxCommand)) {
    return EFI_INVALID_PARAMETER;
  }

  PayloadLength = 0;
  PayloadBuffer = NULL;
  if (Command->Payload != NULL) {
    PayloadLength = Command->PayloadSize;
    PayloadBuffer = Command->Payload;
  } else if (Command->Text != NULL) {
    if ((Command->Type == TcpConnectSession) || (Command->Type == TcpOutputText)) {
      PayloadLength = StrSize (Command->Text);
      if (PayloadLength >= sizeof (CHAR16)) {
        PayloadLength -= sizeof (CHAR16);
      } else {
        PayloadLength = 0;
      }
      PayloadBuffer = Command->Text;
    } else {
      CHAR8  *AsciiPayload;

      PayloadLength = StrLen (Command->Text) + 1;
      AsciiPayload  = AllocateZeroPool (PayloadLength);
      if (AsciiPayload == NULL) {
        return EFI_OUT_OF_RESOURCES;
      }

      Status = UnicodeStrToAsciiStrS (Command->Text, AsciiPayload, PayloadLength);
      if (EFI_ERROR (Status)) {
        FreePool (AsciiPayload);
        return Status;
      }

      PayloadLength = AsciiStrLen (AsciiPayload);
      PayloadBuffer = AsciiPayload;
    }
  }

  AllocationSize = COMMAND_HEADER_SIZE + PayloadLength;
  PacketBuffer   = AllocateZeroPool (AllocationSize);
  TransmitData   = AllocateZeroPool (sizeof (EFI_TCP4_TRANSMIT_DATA));
  if ((PacketBuffer == NULL) || (TransmitData == NULL)) {
    if (PacketBuffer != NULL) {
      FreePool (PacketBuffer);
    }

    if (TransmitData != NULL) {
      FreePool (TransmitData);
    }

    return EFI_OUT_OF_RESOURCES;
  }

  PacketBuffer[0] = (UINT8)Command->Type;
  PacketBuffer[1] = (UINT8)(PayloadLength & 0xFF);
  PacketBuffer[2] = (UINT8)((PayloadLength >> 8) & 0xFF);
  if (PayloadLength > 0) {
    CopyMem (PacketBuffer + COMMAND_HEADER_SIZE, PayloadBuffer, PayloadLength);
  }

  TransmitData->Push                            = TRUE;
  TransmitData->Urgent                          = FALSE;
  TransmitData->DataLength                      = (UINT32)AllocationSize;
  TransmitData->FragmentCount                   = 1;
  TransmitData->FragmentTable[0].FragmentLength = (UINT32)AllocationSize;
  TransmitData->FragmentTable[0].FragmentBuffer = PacketBuffer;

  ZeroMem (&Client->TransmitToken, sizeof (Client->TransmitToken));
  Client->TransmitToken.CompletionToken.Event  = Client->TransmitEvent;
  Client->TransmitToken.CompletionToken.Status = EFI_NOT_READY;
  Client->TransmitToken.Packet.TxData          = TransmitData;

  Status = Client->Tcp4->Transmit (Client->Tcp4, &Client->TransmitToken);
  if (!EFI_ERROR (Status)) {
    Status = WaitForTcp4Token (Client->Tcp4, &Client->TransmitToken.CompletionToken);
  }

  if ((Command->Payload == NULL) &&
      (Command->Text != NULL) &&
      (Command->Type != TcpConnectSession) &&
      (Command->Type != TcpOutputText) &&
      (PayloadBuffer != NULL))
  {
    FreePool (PayloadBuffer);
  }

  FreePool (PacketBuffer);
  FreePool (TransmitData);
  return Status;
}

STATIC
EFI_STATUS
ReceiveExactBytes (
  IN  SOCKET_CLIENT  *Client,
  OUT UINT8          *Buffer,
  IN  UINTN          BufferSize
  )
{
  EFI_STATUS             Status;
  EFI_TCP4_RECEIVE_DATA  *ReceiveData;
  UINT8                  ChunkBuffer[RESPONSE_CHUNK_SIZE];
  UINTN                  AllocationSize;
  UINTN                  TotalRead;
  UINTN                  CopyLength;
  UINTN                  StashCopyLength;

  if ((Client == NULL) || (Client->Tcp4 == NULL) || (Buffer == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  AllocationSize = sizeof (EFI_TCP4_RECEIVE_DATA);
  ReceiveData    = AllocateZeroPool (AllocationSize);
  if (ReceiveData == NULL) {
    return EFI_OUT_OF_RESOURCES;
  }

  TotalRead = 0;
  Status    = EFI_SUCCESS;
  while (TotalRead < BufferSize) {
    if (Client->ReceiveStashLength > 0) {
      StashCopyLength = MIN (Client->ReceiveStashLength, BufferSize - TotalRead);
      CopyMem (Buffer + TotalRead, Client->ReceiveStash, StashCopyLength);
      TotalRead += StashCopyLength;

      if (StashCopyLength < Client->ReceiveStashLength) {
        CopyMem (
          Client->ReceiveStash,
          Client->ReceiveStash + StashCopyLength,
          Client->ReceiveStashLength - StashCopyLength
          );
      }

      Client->ReceiveStashLength -= StashCopyLength;
      continue;
    }

    ZeroMem (ReceiveData, AllocationSize);
    ZeroMem (ChunkBuffer, sizeof (ChunkBuffer));

    ReceiveData->UrgentFlag                      = FALSE;
    ReceiveData->DataLength                      = sizeof (ChunkBuffer);
    ReceiveData->FragmentCount                   = 1;
    ReceiveData->FragmentTable[0].FragmentLength = sizeof (ChunkBuffer);
    ReceiveData->FragmentTable[0].FragmentBuffer = ChunkBuffer;

    ZeroMem (&Client->ReceiveToken, sizeof (Client->ReceiveToken));
    Client->ReceiveToken.CompletionToken.Event  = Client->ReceiveEvent;
    Client->ReceiveToken.CompletionToken.Status = EFI_NOT_READY;
    Client->ReceiveToken.Packet.RxData          = ReceiveData;

    Status = Client->Tcp4->Receive (Client->Tcp4, &Client->ReceiveToken);
    if (EFI_ERROR (Status)) {
      break;
    }

    Status = WaitForTcp4Token (Client->Tcp4, &Client->ReceiveToken.CompletionToken);
    if (Status == EFI_CONNECTION_FIN) {
      if (TotalRead == 0) {
        break;
      }

      Status = EFI_DEVICE_ERROR;
      break;
    }

    if (EFI_ERROR (Status)) {
      break;
    }

    if (ReceiveData->DataLength == 0) {
      Status = EFI_DEVICE_ERROR;
      break;
    }

    CopyLength = MIN ((UINTN)ReceiveData->DataLength, BufferSize - TotalRead);
    CopyMem (Buffer + TotalRead, ChunkBuffer, CopyLength);
    TotalRead += CopyLength;

    if ((UINTN)ReceiveData->DataLength > CopyLength) {
      Client->ReceiveStashLength = (UINTN)ReceiveData->DataLength - CopyLength;
      CopyMem (
        Client->ReceiveStash,
        ChunkBuffer + CopyLength,
        Client->ReceiveStashLength
        );
    }
  }

  FreePool (ReceiveData);
  return Status;
}

EFI_STATUS
ReceiveCommandPacket (
  IN  SOCKET_CLIENT  *Client,
  OUT TCP_COMMAND    *Command
  )
{
  EFI_STATUS  Status;
  UINT8       Header[COMMAND_HEADER_SIZE];
  UINT16      PayloadLength;
  CHAR8       *AsciiPayload;
  BOOLEAN     TextPayload;

  if ((Client == NULL) || (Command == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  ZeroMem (Command, sizeof (*Command));

  Status = ReceiveExactBytes (Client, Header, sizeof (Header));
  if (EFI_ERROR (Status)) {
    return Status;
  }

  Command->Type = (COMMAND_TYPE)Header[0];
  PayloadLength = (UINT16)(Header[1] | (Header[2] << 8));
  Command->PayloadSize = PayloadLength;

  if (PayloadLength == 0) {
    return EFI_SUCCESS;
  }

  AsciiPayload = AllocateZeroPool ((UINTN)PayloadLength + 1);
  if (AsciiPayload == NULL) {
    Print (L"Failed to allocate memory for payload\n");
    return EFI_OUT_OF_RESOURCES;
  }

  Status = ReceiveExactBytes (Client, (UINT8 *)AsciiPayload, PayloadLength);
  if (EFI_ERROR (Status)) {
    Print (L"Failed to receive payload data\n");
    FreePool (AsciiPayload);
    return Status;
  }

  Command->Payload = AllocateCopyPool ((UINTN)PayloadLength, AsciiPayload);
  if (Command->Payload == NULL) {
    Print (L"Failed to allocate memory for command payload\n");
    FreePool (AsciiPayload);
    return EFI_OUT_OF_RESOURCES;
  }

  TextPayload = (BOOLEAN)(
                  (Command->Type == TcpSendText) ||
                  (Command->Type == TcpEchoText) ||
                  (Command->Type == TcpConnectSession) ||
                  (Command->Type == TcpOutputText)
                  );
  if (!TextPayload) {
    FreePool (AsciiPayload);
    return EFI_SUCCESS;
  }

  Command->Text = AllocateZeroPool (((UINTN)PayloadLength + 1) * sizeof (CHAR16));
  if (Command->Text == NULL) {
    Print (L"Failed to allocate memory for command text\n");
    FreePool (AsciiPayload);
    FreePool (Command->Payload);
    Command->Payload = NULL;
    return EFI_OUT_OF_RESOURCES;
  }

  Status = AsciiStrToUnicodeStrS (AsciiPayload, Command->Text, (UINTN)PayloadLength + 1);
  FreePool (AsciiPayload);
  if (EFI_ERROR (Status)) {
    Print (L"Failed to convert payload to Unicode string\n");
    FreeCommandPacket (Command);
    return Status;
  }

  return EFI_SUCCESS;
}

VOID
FreeCommandPacket (
  IN OUT TCP_COMMAND  *Command
  )
{
  if (Command == NULL) {
    return;
  }

  if (Command->Text != NULL) {
    FreePool (Command->Text);
    Command->Text = NULL;
  }

  if (Command->Payload != NULL) {
    FreePool (Command->Payload);
    Command->Payload = NULL;
  }

  Command->PayloadSize = 0;
}

EFI_STATUS
ReceiveAndPrintResponse (
  IN SOCKET_CLIENT  *Client
  )
{
  EFI_STATUS             Status;
  EFI_TCP4_RECEIVE_DATA  *ReceiveData;
  CHAR8                  *ResponseBuffer;
  UINTN                  AllocationSize;

  if ((Client == NULL) || (Client->Tcp4 == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  AllocationSize = sizeof (EFI_TCP4_RECEIVE_DATA) + sizeof (EFI_TCP4_FRAGMENT_DATA) * 0;
  ReceiveData    = AllocateZeroPool (AllocationSize);
  ResponseBuffer = AllocateZeroPool (RESPONSE_CHUNK_SIZE + 1);
  if ((ReceiveData == NULL) || (ResponseBuffer == NULL)) {
    if (ReceiveData != NULL) {
      FreePool (ReceiveData);
    }

    if (ResponseBuffer != NULL) {
      FreePool (ResponseBuffer);
    }

    return EFI_OUT_OF_RESOURCES;
  }

  while (TRUE) {
    ZeroMem (ReceiveData, AllocationSize);
    ZeroMem (ResponseBuffer, RESPONSE_CHUNK_SIZE + 1);

    ReceiveData->UrgentFlag                      = FALSE;
    ReceiveData->DataLength                      = RESPONSE_CHUNK_SIZE;
    ReceiveData->FragmentCount                   = 1;
    ReceiveData->FragmentTable[0].FragmentLength = RESPONSE_CHUNK_SIZE;
    ReceiveData->FragmentTable[0].FragmentBuffer = ResponseBuffer;

    ZeroMem (&Client->ReceiveToken, sizeof (Client->ReceiveToken));
    Client->ReceiveToken.CompletionToken.Event  = Client->ReceiveEvent;
    Client->ReceiveToken.CompletionToken.Status = EFI_NOT_READY;
    Client->ReceiveToken.Packet.RxData          = ReceiveData;

    Status = Client->Tcp4->Receive (Client->Tcp4, &Client->ReceiveToken);
    if (EFI_ERROR (Status)) {
      break;
    }

    Status = WaitForTcp4Token (Client->Tcp4, &Client->ReceiveToken.CompletionToken);
    if (Status == EFI_CONNECTION_FIN) {
      Status = EFI_SUCCESS;
      break;
    }

    if (EFI_ERROR (Status)) {
      break;
    }

    ResponseBuffer[ReceiveData->DataLength] = '\0';
    Print (L"%a", ResponseBuffer);

    if (ReceiveData->DataLength < RESPONSE_CHUNK_SIZE) {
      break;
    }
  }

  FreePool (ResponseBuffer);
  FreePool (ReceiveData);
  return Status;
}

VOID
CloseSocketClient (
  IN SOCKET_CLIENT  *Client
  )
{
  if (Client == NULL) {
    return;
  }

  if (Client->Tcp4 != NULL) {
    ZeroMem (&Client->CloseToken, sizeof (Client->CloseToken));
    Client->CloseToken.CompletionToken.Event  = Client->CloseEvent;
    Client->CloseToken.CompletionToken.Status = EFI_NOT_READY;

    if (!EFI_ERROR (Client->Tcp4->Close (Client->Tcp4, &Client->CloseToken))) {
      WaitForTcp4Token (Client->Tcp4, &Client->CloseToken.CompletionToken);
    }

    Client->Tcp4->Configure (Client->Tcp4, NULL);
  }

  if ((Client->ChildHandle != NULL) && (Client->ServiceBinding != NULL)) {
    Client->ServiceBinding->DestroyChild (Client->ServiceBinding, Client->ChildHandle);
  }

  if (Client->ConnectEvent != NULL) {
    gBS->CloseEvent (Client->ConnectEvent);
  }

  if (Client->TransmitEvent != NULL) {
    gBS->CloseEvent (Client->TransmitEvent);
  }

  if (Client->ReceiveEvent != NULL) {
    gBS->CloseEvent (Client->ReceiveEvent);
  }

  if (Client->CloseEvent != NULL) {
    gBS->CloseEvent (Client->CloseEvent);
  }
}
