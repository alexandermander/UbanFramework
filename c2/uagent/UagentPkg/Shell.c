#include "Uagent.h"

STATIC
EFI_STATUS
ReadCommandLine (
  OUT CHAR16  *Buffer,
  IN  UINTN   BufferChars
  );

STATIC VOID    *mUploadedAppBuffer = NULL;
STATIC UINTN   mUploadedAppSize    = 0;
STATIC CHAR16  *mUploadedAppName   = NULL;

STATIC
EFI_STATUS
DuplicateResponse (
  IN  CONST CHAR16  *Source,
  OUT CHAR16        **Response
  )
{
  UINTN  Size;

  if ((Source == NULL) || (Response == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  Size      = StrSize (Source);
  *Response = AllocateCopyPool (Size, Source);
  if (*Response == NULL) {
    return EFI_OUT_OF_RESOURCES;
  }

  return EFI_SUCCESS;
}

STATIC
VOID
ClearUploadedApp (
  VOID
  )
{
  if (mUploadedAppBuffer != NULL) {
    FreePool (mUploadedAppBuffer);
    mUploadedAppBuffer = NULL;
  }

  if (mUploadedAppName != NULL) {
    FreePool (mUploadedAppName);
    mUploadedAppName = NULL;
  }

  mUploadedAppSize = 0;
}

STATIC
EFI_STATUS
UploadEfiApp (
  IN  CONST TCP_COMMAND  *Command,
  OUT CHAR16             **Response
  )
{
  EFI_STATUS  Status;
  UINT8       FileNameLength;
  UINTN       FileSize;
  CHAR8       *AsciiFileName;

  if ((Command == NULL) || (Response == NULL) || (Command->Payload == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  *Response = NULL;
  if (Command->PayloadSize < 2) {
    return DuplicateResponse (L"Invalid EFI upload payload\n", Response);
  }

  FileNameLength = ((UINT8 *)Command->Payload)[0];
  if ((FileNameLength == 0) || (Command->PayloadSize <= (UINTN)FileNameLength)) {
    return DuplicateResponse (L"Invalid EFI upload filename\n", Response);
  }

  FileSize = Command->PayloadSize - 1 - FileNameLength;
  if (FileSize == 0) {
    return DuplicateResponse (L"Uploaded EFI image is empty\n", Response);
  }

  AsciiFileName = AllocateZeroPool ((UINTN)FileNameLength + 1);
  if (AsciiFileName == NULL) {
    return EFI_OUT_OF_RESOURCES;
  }

  CopyMem (AsciiFileName, (UINT8 *)Command->Payload + 1, FileNameLength);

  ClearUploadedApp ();

  mUploadedAppBuffer = AllocateCopyPool (FileSize, (UINT8 *)Command->Payload + 1 + FileNameLength);
  if (mUploadedAppBuffer == NULL) {
    FreePool (AsciiFileName);
    return EFI_OUT_OF_RESOURCES;
  }

  mUploadedAppName = AllocateZeroPool (((UINTN)FileNameLength + 1) * sizeof (CHAR16));
  if (mUploadedAppName == NULL) {
    FreePool (AsciiFileName);
    ClearUploadedApp ();
    return EFI_OUT_OF_RESOURCES;
  }

  Status = AsciiStrToUnicodeStrS (AsciiFileName, mUploadedAppName, (UINTN)FileNameLength + 1);
  FreePool (AsciiFileName);
  if (EFI_ERROR (Status)) {
    ClearUploadedApp ();
    return DuplicateResponse (L"Uploaded filename must be ASCII text\n", Response);
  }

  mUploadedAppSize = FileSize;
  return DuplicateResponse (L"EFI app uploaded and ready to run\n", Response);
}

STATIC
EFI_STATUS
ExecuteUploadedEfiApp (
  IN  EFI_HANDLE  ImageHandle,
  OUT CHAR16      **Response
  )
{
  EFI_STATUS  LoadStatus;
  EFI_STATUS  StartStatus;
  EFI_STATUS  UnloadStatus;
  EFI_HANDLE  ChildImageHandle;
  UINTN       ResponseSize;
  CHAR16      *DynamicResponse;
  CONST CHAR16  *AppName;

  if (Response == NULL) {
    return EFI_INVALID_PARAMETER;
  }

  *Response = NULL;
  if ((mUploadedAppBuffer == NULL) || (mUploadedAppSize == 0)) {
    return DuplicateResponse (L"No uploaded EFI app available\n", Response);
  }

  AppName           = (mUploadedAppName != NULL) ? mUploadedAppName : L"<unnamed>";
  ChildImageHandle = NULL;
  LoadStatus       = gBS->LoadImage (
                            FALSE,
                            ImageHandle,
                            NULL,
                            mUploadedAppBuffer,
                            mUploadedAppSize,
                            &ChildImageHandle
                            );
  if (EFI_ERROR (LoadStatus)) {
    ResponseSize    = sizeof (L"LoadImage failed for : \n") + StrSize (AppName) + sizeof (CHAR16) * 16;
    DynamicResponse = AllocateZeroPool (ResponseSize);
    if (DynamicResponse == NULL) {
      return EFI_OUT_OF_RESOURCES;
    }

    UnicodeSPrint (
      DynamicResponse,
      ResponseSize,
      L"LoadImage failed for %s: %r\n",
      AppName,
      LoadStatus
      );
    *Response = DynamicResponse;
    return EFI_SUCCESS;
  }

  StartStatus  = gBS->StartImage (ChildImageHandle, NULL, NULL);
  if (EFI_ERROR (StartStatus)) {
    UnloadStatus = gBS->UnloadImage (ChildImageHandle);
  } else {
    UnloadStatus = EFI_SUCCESS;
  }
  ResponseSize    = sizeof (L"Executed : \n") + StrSize (AppName) + sizeof (CHAR16) * 32;
  DynamicResponse = AllocateZeroPool (ResponseSize);
  if (DynamicResponse == NULL) {
    return EFI_OUT_OF_RESOURCES;
  }

  if (EFI_ERROR (StartStatus)) {
    UnicodeSPrint (
      DynamicResponse,
      ResponseSize,
      L"Execution failed for %s: %r (UnloadImage: %r)\n",
      AppName,
      StartStatus,
      UnloadStatus
      );
  } else {
    UnicodeSPrint (
      DynamicResponse,
      ResponseSize,
      L"Executed %s: %r\n",
      AppName,
      StartStatus
      );
  }

  *Response = DynamicResponse;
  return EFI_SUCCESS;
}

STATIC
EFI_STATUS
ExecuteRemoteInstruction (
  IN  CONST CHAR16  *CommandLine,
  OUT CHAR16        **Response,
  OUT BOOLEAN       *Disconnect
  )
{
  CHAR16  *DynamicResponse;
  UINTN   ResponseSize;

  if ((CommandLine == NULL) || (Response == NULL) || (Disconnect == NULL)) {
    return EFI_INVALID_PARAMETER;
  }

  *Disconnect = FALSE;
  *Response   = NULL;

  if ((StrCmp (CommandLine, L"help") == 0) || (StrCmp (CommandLine, L"?") == 0)) {
    return DuplicateResponse (
             L"Commands: help, status, echo <text>, disconnect, reboot\n",
             Response
             );
  }

  if (StrCmp (CommandLine, L"status") == 0) {
    return DuplicateResponse (L"Uagent remote session active\n", Response);
  }

  if (StrCmp (CommandLine, L"disconnect") == 0) {
    *Disconnect = TRUE;
    return DuplicateResponse (L"Disconnecting remote session\n", Response);
  }

  if (StrCmp (CommandLine, L"reboot") == 0) {
    gRT->ResetSystem (EfiResetCold, EFI_SUCCESS, 0, NULL);
    return DuplicateResponse (L"Reboot requested\n", Response);
  }

  if (StrnCmp (CommandLine, L"echo ", 5) == 0) {
    ResponseSize = StrSize (CommandLine + 5) + sizeof (CHAR16);
    DynamicResponse = AllocateZeroPool (ResponseSize);
    if (DynamicResponse == NULL) {
      return EFI_OUT_OF_RESOURCES;
    }

    UnicodeSPrint (DynamicResponse, ResponseSize, L"%s\n", CommandLine + 5);
    *Response = DynamicResponse;
    return EFI_SUCCESS;
  }

  ResponseSize = StrSize (CommandLine) + sizeof (L"Unknown remote command: \n");
  DynamicResponse = AllocateZeroPool (ResponseSize);
  if (DynamicResponse == NULL) {
    return EFI_OUT_OF_RESOURCES;
  }

  UnicodeSPrint (
    DynamicResponse,
    ResponseSize,
    L"Unknown remote command: %s\n",
    CommandLine
    );
  *Response = DynamicResponse;
  return EFI_SUCCESS;
}

EFI_STATUS
RunRemoteSession (
  IN EFI_HANDLE  ImageHandle
  )
{
  EFI_STATUS    Status;
  SOCKET_CLIENT Client;
  TCP_COMMAND   Incoming;
  TCP_COMMAND   Outgoing;
  CHAR16        *Response;
  BOOLEAN       Disconnect;

  Status = InitSocketClient (ImageHandle, &Client);
  if (EFI_ERROR (Status)) {
    return Status;
  }

  SetUagentActiveClient (&Client);
  Status = InstallUagentDebugProtocol ();
  if (EFI_ERROR (Status) && (Status != EFI_ALREADY_STARTED)) {
    SetUagentActiveClient (NULL);
    CloseSocketClient (&Client);
    return Status;
  }

  Outgoing.Type = TcpConnectSession;
  Outgoing.Text = L"Uagent remote session ready";
  Outgoing.Payload = NULL;
  Outgoing.PayloadSize = 0;
  Status        = SendCommandPacket (&Client, &Outgoing);
  if (EFI_ERROR (Status)) {
    UninstallUagentDebugProtocol ();
    SetUagentActiveClient (NULL);
    CloseSocketClient (&Client);
    return Status;
  }

  while (TRUE) {
    Status = ReceiveCommandPacket (&Client, &Incoming);
    if (Status == EFI_CONNECTION_FIN) {
      Status = EFI_SUCCESS;
      break;
    }

    if (EFI_ERROR (Status)) {
      break;
    }

    Response   = NULL;
    Disconnect = FALSE;

  if (Incoming.Type == TcpDisconnectSession) {
      Disconnect = TRUE;
      Status     = DuplicateResponse (L"Server requested disconnect\n", &Response);
    } else if (Incoming.Type == TcpGetApps) {
      Status = DuplicateResponse (L"No local app registry implemented\n", &Response);
    } else if (Incoming.Type == TcpEchoText) {
      if ((Incoming.Text == NULL) || (StrLen (Incoming.Text) == 0)) {
        Status = DuplicateResponse (L"\n", &Response);
      } else {
        Status = DuplicateResponse (Incoming.Text, &Response);
      }
    } else if (Incoming.Type == TcpPushEfiApp) {
      Status = UploadEfiApp (&Incoming, &Response);
    } else if (Incoming.Type == TcpExecuteEfiApp) {
      Status = ExecuteUploadedEfiApp (ImageHandle, &Response);
    } else if (Incoming.Type == TcpSendText) {
      Status = ExecuteRemoteInstruction (Incoming.Text, &Response, &Disconnect);
    } else {
      Status = DuplicateResponse (L"Unsupported remote command type\n", &Response);
    }

    if (!EFI_ERROR (Status) && (Response != NULL)) {
      Outgoing.Type = TcpOutputText;
      Outgoing.Text = Response;
      Outgoing.Payload = NULL;
      Outgoing.PayloadSize = 0;
      Status        = SendCommandPacket (&Client, &Outgoing);
    }

    if (Response != NULL) {
      FreePool (Response);
    }

    FreeCommandPacket (&Incoming);

    if (EFI_ERROR (Status) || Disconnect) {
      if (!EFI_ERROR (Status) && Disconnect) {
        Status = EFI_SUCCESS;
      }

      break;
    }
  }

  UninstallUagentDebugProtocol ();
  SetUagentActiveClient (NULL);
  CloseSocketClient (&Client);
  ClearUploadedApp ();
  return Status;
}

STATIC
VOID
PrintHelp (
  VOID
  )
{
  Print (L"Commands:\n");
  Print (L"  help           Show this help\n");
  Print (L"  connect        Start a manual interactive server session\n");
  Print (L"  exit           Leave the CLI\n");
}

STATIC
EFI_STATUS
ReadCommandLine (
  OUT CHAR16  *Buffer,
  IN  UINTN   BufferChars
  )
{
  EFI_STATUS     Status;
  EFI_INPUT_KEY  Key;
  UINTN          Length;
  UINTN          EventIndex;

  if ((Buffer == NULL) || (BufferChars < 2)) {
    return EFI_INVALID_PARAMETER;
  }

  Buffer[0] = L'\0';
  Length    = 0;

  while (TRUE) {
    Status = gBS->WaitForEvent (1, &gST->ConIn->WaitForKey, &EventIndex);
    if (EFI_ERROR (Status)) {
      return Status;
    }

    Status = gST->ConIn->ReadKeyStroke (gST->ConIn, &Key);
    if (EFI_ERROR (Status)) {
      continue;
    }

    if (Key.UnicodeChar == CHAR_CARRIAGE_RETURN) {
      Print (L"\n");
      Buffer[Length] = L'\0';
      return EFI_SUCCESS;
    }

    if (Key.UnicodeChar == CHAR_BACKSPACE) {
      if (Length > 0) {
        Length--;
        Buffer[Length] = L'\0';
        Print (L"\b \b");
      }

      continue;
    }

    if ((Key.UnicodeChar >= L' ') && (Length + 1 < BufferChars)) {
      Buffer[Length++] = Key.UnicodeChar;
      Buffer[Length]   = L'\0';
      Print (L"%c", Key.UnicodeChar);
    }
  }
}

EFI_STATUS
RunShell (
  IN EFI_HANDLE  ImageHandle
  )
{
  EFI_STATUS  Status;
  CHAR16      Command[COMMAND_BUF_SIZE];

  Print (L"Uagent v%s\n", UAGENT_VERSION);
  Print (L"Type 'help' for commands.\n");

  while (TRUE) {
    Print (L"ushell>");
    Status = ReadCommandLine (Command, ARRAY_SIZE (Command));
    if (EFI_ERROR (Status)) {
      Print (L"Read command failed: %r\n", Status);
      return Status;
    }

    if ((StrCmp (Command, L"exit") == 0) || (StrCmp (Command, L"quit") == 0)) {
      return EFI_SUCCESS;
    }

    if ((StrCmp (Command, L"help") == 0) || (StrCmp (Command, L"?") == 0)) {
      PrintHelp ();
      continue;
    }

    if (StrCmp (Command, L"connect") == 0) {
      Status = RunRemoteSession (ImageHandle);
      if (EFI_ERROR (Status)) {
        Print (L"connect failed: %r\n", Status);
      } else {
        Print (L"Remote session closed.\n");
      }

      continue;
    }

    if (StrLen (Command) == 0) {
      continue;
    }

    Print (L"Unknown command: %s\n", Command);
  }
}
