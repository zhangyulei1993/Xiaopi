type CacheSession = { accessToken: string; role: "user" | "admin" };

export function readPrivateCache<T>(
  session: CacheSession | null,
  role: CacheSession["role"],
  key: string,
  read: (key: string) => unknown,
  onRead?: (present: boolean) => void,
): T | undefined {
  if (!session || session.role !== role || read(key + ":owner") !== session.accessToken) return;
  const value = read(key) as T | undefined;
  onRead?.(Boolean(value));
  return value;
}

export function writePrivateCache(
  session: CacheSession | null,
  role: CacheSession["role"],
  key: string,
  value: unknown,
  write: (key: string, value: unknown) => void,
) {
  if (!session || session.role !== role) return;
  write(key + ":owner", "");
  write(key, value);
  write(key + ":owner", session.accessToken);
}
