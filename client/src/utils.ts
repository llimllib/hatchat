/**
 * Format a timestamp for display.
 * - Today: "3:45 PM"
 * - This year: "Jan 31, 3:45 PM"
 * - Older: "Jan 31, 2024, 3:45 PM"
 */
export function formatTimestamp(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();

  const isToday =
    date.getDate() === now.getDate() &&
    date.getMonth() === now.getMonth() &&
    date.getFullYear() === now.getFullYear();

  const isThisYear = date.getFullYear() === now.getFullYear();

  const timeStr = date.toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });

  if (isToday) {
    return timeStr;
  }

  const dateOptions: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
  };

  if (!isThisYear) {
    dateOptions.year = "numeric";
  }

  const dateStr = date.toLocaleDateString("en-US", dateOptions);
  return `${dateStr}, ${timeStr}`;
}

/**
 * Format a timestamp for hover tooltip (full date and time)
 */
export function formatTimestampFull(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleString("en-US", {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    hour12: true,
  });
}

/**
 * Format a date for display (just the date, no time)
 */
export function formatDate(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

/**
 * Format a "last seen" timestamp for display.
 * - Just now (< 1 min): "Active just now"
 * - Minutes ago: "Active 5m ago"
 * - Hours ago: "Active 3h ago"
 * - Days ago: "Active 2d ago"
 * - Longer: "Active Jan 31"
 * - Never/unknown: "Offline"
 */
export function formatLastSeen(isoString: string | undefined): string {
  if (!isoString) {
    return "Offline";
  }

  const date = new Date(isoString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMinutes < 1) {
    return "Active just now";
  }
  if (diffMinutes < 60) {
    return `Active ${diffMinutes}m ago`;
  }
  if (diffHours < 24) {
    return `Active ${diffHours}h ago`;
  }
  if (diffDays < 7) {
    return `Active ${diffDays}d ago`;
  }

  // Older - show date
  const dateStr = date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
  return `Active ${dateStr}`;
}

/**
 * Get initials from a username for avatar placeholder
 */
export function getInitials(username: string): string {
  // Split by common separators and take first letter of first two parts
  const parts = username.split(/[\s._-]+/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  // Just use first two letters of username
  return username.slice(0, 2).toUpperCase();
}

/**
 * Generate a consistent color from a string (for avatar backgrounds)
 */
export function stringToColor(str: string): string {
  // List of pleasant colors for avatars
  const colors = [
    "#e91e63", // pink
    "#9c27b0", // purple
    "#673ab7", // deep purple
    "#3f51b5", // indigo
    "#2196f3", // blue
    "#03a9f4", // light blue
    "#00bcd4", // cyan
    "#009688", // teal
    "#4caf50", // green
    "#8bc34a", // light green
    "#ff9800", // orange
    "#ff5722", // deep orange
    "#795548", // brown
    "#607d8b", // blue grey
  ];

  // Simple hash function
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }

  return colors[Math.abs(hash) % colors.length];
}
