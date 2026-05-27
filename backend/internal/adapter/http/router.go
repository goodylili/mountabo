package http

import nethttp "net/http"

// NewRouter maps the GitHub connection endpoints the local UI calls:
//
//	POST   /api/github/exchange      exchange an OAuth code for a stored token
//	GET    /api/github/status        report the connected account, if any
//	GET    /api/github/repos         list the connected account's repositories
//	GET    /api/github/ports         detect a repo's published ports (compose/Dockerfile)
//	GET    /api/github/tree          list a repo's file tree (directory/file picker)
//	GET    /api/github/env-example    variable names from a repo's .env.example
//	GET    /api/github/run-steps     latest deploy run's jobs + steps (live status)
//	GET    /api/github/job-logs      one job's full log (what each step printed)
//	DELETE /api/github/token         forget the stored token
//	POST   /api/servers              add a server (probe specs over SSH)
//	GET    /api/servers              list added servers
//	GET    /api/servers/{id}/setup   run the bootstrap, streaming progress (SSE)
//	GET    /api/servers/{id}/ports   list ports already listening on the server
//	GET    /api/servers/{id}/logs    the deployed app's recent logs (over SSH)
//	GET    /api/servers/domains/preview      render a domain's nginx config + script (no side effects)
//	GET    /api/servers/{id}/domains/add     point a domain at an app port, streaming (SSE)
//	GET    /api/servers/{id}/domains/remove  tear a domain's nginx + TLS down, streaming (SSE)
//	POST   /api/deploy/preview       generate the workflow + deploy.sh + secrets (no side effects)
//	POST   /api/servers/{id}/deploy  configure deployment of a repo, streaming (SSE)
//	GET    /api/deployments          deploy history (configured deployments + their Actions runs)
//	DELETE /api/servers/{id}         remove a server and destroy its secrets
func NewRouter(gh *GitHubHandler, sv *ServersHandler, dep *DeployHandler, mon *MonitorHandler) *nethttp.ServeMux {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("POST /api/github/exchange", gh.Exchange)
	mux.HandleFunc("GET /api/github/status", gh.Status)
	mux.HandleFunc("GET /api/github/repos", gh.Repos)
	mux.HandleFunc("GET /api/github/ports", gh.Ports)
	mux.HandleFunc("GET /api/github/tree", gh.Tree)
	mux.HandleFunc("GET /api/github/env-example", gh.EnvExample)
	mux.HandleFunc("GET /api/github/run-steps", gh.RunSteps)
	mux.HandleFunc("GET /api/github/job-logs", gh.JobLogs)
	mux.HandleFunc("DELETE /api/github/token", gh.Disconnect)

	mux.HandleFunc("POST /api/servers", sv.Add)
	mux.HandleFunc("GET /api/servers", sv.List)
	mux.HandleFunc("GET /api/servers/options", sv.Options)
	mux.HandleFunc("GET /api/servers/{id}/setup", sv.Setup)
	mux.HandleFunc("GET /api/servers/{id}/ports", sv.Ports)
	mux.HandleFunc("GET /api/servers/{id}/metrics", sv.Metrics)
	mux.HandleFunc("GET /api/servers/{id}/logs", sv.Logs)
	mux.HandleFunc("GET /api/servers/{id}/options", sv.ApplyOptions)
	mux.HandleFunc("GET /api/servers/domains/preview", sv.DomainsPreview)
	mux.HandleFunc("GET /api/servers/{id}/domains/add", sv.AddDomain)
	mux.HandleFunc("GET /api/servers/{id}/domains/remove", sv.RemoveDomain)
	mux.HandleFunc("POST /api/deploy/preview", dep.Preview)
	mux.HandleFunc("POST /api/servers/{id}/deploy", dep.Deploy)
	mux.HandleFunc("GET /api/deployments", mon.Deployments)
	mux.HandleFunc("DELETE /api/servers/{id}", sv.Delete)
	return mux
}
