interface AvatarProps {
  name: string;
  avatarUrl?: string;
  size?: "sm" | "md" | "lg";
}

export function Avatar({ name, avatarUrl, size = "md" }: AvatarProps) {
  const initial = name.trim().slice(0, 1).toUpperCase() || "B";

  return (
    <span className={`avatar avatar-${size}`} aria-hidden="true">
      {avatarUrl ? <img src={avatarUrl} alt="" /> : <span>{initial}</span>}
    </span>
  );
}
