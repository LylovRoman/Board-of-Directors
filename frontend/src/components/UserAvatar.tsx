import { useState } from "react";

export function UserAvatar(props: { name: string; avatarUrl?: string; size?: "small" | "medium" | "large" }) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const avatarUrl = props.avatarUrl?.trim();
  const shouldShowImage = Boolean(avatarUrl && failedUrl !== avatarUrl);
  const initial = Array.from(props.name.trim())[0]?.toUpperCase() ?? "?";

  return (
      <span className={`user-avatar avatar-${props.size ?? "medium"}`} aria-hidden="true">
      {shouldShowImage ? (
          <img src={avatarUrl} alt="" onError={() => setFailedUrl(avatarUrl ?? null)} />
      ) : (
          <span>{initial}</span>
      )}
    </span>
  );
}
