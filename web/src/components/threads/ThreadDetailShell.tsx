import type { ReactNode } from "react";

interface ThreadDetailShellProps {
  header: ReactNode;
  errorBanner?: ReactNode;
  invitePickerDialog: ReactNode;
  messageList: ReactNode;
  composer: ReactNode;
  sidebar: ReactNode;
}

export function ThreadDetailShell({
  header,
  errorBanner,
  invitePickerDialog,
  messageList,
  composer,
  sidebar,
}: ThreadDetailShellProps) {
  return (
    <div className="flex h-full flex-col overflow-hidden">
      {header}
      {errorBanner}
      {invitePickerDialog}
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex-1 overflow-y-auto px-5 py-4">{messageList}</div>
          {composer}
        </div>
        <div className="w-80 shrink-0 border-l">{sidebar}</div>
      </div>
    </div>
  );
}
