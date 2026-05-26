import { backendURL } from "@/lib/servers";
import type { Deployment, RunStatus } from "@/lib/data";

type RunResponse = {
  sha: string;
  message: string;
  status: string;
  when: string;
  duration: string;
};

type DeploymentResponse = {
  app: string;
  repo: string;
  branch: string;
  serverId: string;
  url: string;
  status: string;
  lastDeploy: string;
  runs: RunResponse[];
};

// getDeployments reads deploy history from the Go backend: the configured
// deployments enriched with their recent GitHub Actions runs. Server metrics
// (cpu/mem/ping, uptime) aren't probed, so they read as "n/a". Returns [] when
// nothing is deployed or the backend is unreachable, so the monitor shows an
// honest empty state. Called server-side from the monitor page.
export async function getDeployments(): Promise<Deployment[]> {
  try {
    const resp = await fetch(`${backendURL()}/api/deployments`, { cache: "no-store" });
    if (!resp.ok) return [];
    const data = (await resp.json()) as DeploymentResponse[];
    return data.map((d) => ({
      app: d.app,
      repo: d.repo,
      serverId: d.serverId,
      branch: d.branch,
      status: (["live", "idle", "failing"].includes(d.status) ? d.status : "idle") as Deployment["status"],
      url: d.url,
      uptimePct: "n/a",
      lastDeploy: d.lastDeploy,
      metrics: { cpu: "n/a", mem: "n/a", ping: "n/a" },
      runs: d.runs.map((r) => ({
        sha: r.sha,
        message: r.message,
        status: r.status as RunStatus,
        when: r.when,
        duration: r.duration,
      })),
    }));
  } catch {
    return [];
  }
}
