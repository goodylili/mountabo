import { backendURL } from "@/lib/servers";

// UptimeKumaAdmin is the credential pair mountabo generated for a server's
// Uptime Kuma instance, seeded directly into UK's SQLite (UK has no setup
// HTTP route). hasAdmin = false means the operator hasn't run "set up admin"
// yet, so the panel shows a CTA instead of credentials.
export type UptimeKumaAdmin =
  | { hasAdmin: false }
  | { hasAdmin: true; username: string; password: string };

// fetchUptimeKumaAdmin reads the stored admin credentials. Returns
// {hasAdmin:false} on any failure so the panel still renders sensibly when the
// backend is unreachable.
export async function fetchUptimeKumaAdmin(
  serverId: string,
  signal?: AbortSignal,
): Promise<UptimeKumaAdmin> {
  try {
    const resp = await fetch(
      `${backendURL()}/api/servers/${encodeURIComponent(serverId)}/dashboard/uptime-kuma/admin`,
      { signal, cache: "no-store" },
    );
    if (!resp.ok) return { hasAdmin: false };
    return (await resp.json()) as UptimeKumaAdmin;
  } catch {
    return { hasAdmin: false };
  }
}

// resetUptimeKumaAdmin asks the backend to generate fresh credentials, seed
// them into UK's database from inside the container, and return them. Returns
// null on any failure so the caller can surface an error.
export async function resetUptimeKumaAdmin(
  serverId: string,
): Promise<UptimeKumaAdmin | null> {
  try {
    const resp = await fetch(
      `${backendURL()}/api/servers/${encodeURIComponent(serverId)}/dashboard/uptime-kuma/admin/reset`,
      { method: "POST" },
    );
    if (!resp.ok) return null;
    return (await resp.json()) as UptimeKumaAdmin;
  } catch {
    return null;
  }
}
