package http

import nethttp "net/http"

// NewRouter maps the GitHub connection endpoints the local UI calls:
//
//	POST   /api/github/exchange      exchange an OAuth code for a stored token
//	GET    /api/github/status        report the connected account, if any
//	GET    /api/github/repos         list the connected account's repositories
//	GET    /api/github/ports         detect a repo's published ports (compose/Dockerfile)
//	GET    /api/github/tree          list a repo's file tree (directory/file picker)
//	DELETE /api/github/token         forget the stored token
//	POST   /api/servers              add a server (probe specs over SSH)
//	GET    /api/servers              list added servers
//	GET    /api/servers/{id}/setup   run the bootstrap, streaming progress (SSE)
//	GET    /api/servers/{id}/ports   list ports already listening on the server
//	POST   /api/servers/{id}/deploy  configure deployment of a repo, streaming (SSE)
//	DELETE /api/servers/{id}         remove a server and destroy its secrets
func NewRouter(gh *GitHubHandler, sv *ServersHandler, dep *DeployHandler) *nethttp.ServeMux {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("POST /api/github/exchange", gh.Exchange)
	mux.HandleFunc("GET /api/github/status", gh.Status)
	mux.HandleFunc("GET /api/github/repos", gh.Repos)
	mux.HandleFunc("GET /api/github/ports", gh.Ports)
	mux.HandleFunc("GET /api/github/tree", gh.Tree)
	mux.HandleFunc("DELETE /api/github/token", gh.Disconnect)

	mux.HandleFunc("POST /api/servers", sv.Add)
	mux.HandleFunc("GET /api/servers", sv.List)
	mux.HandleFunc("GET /api/servers/options", sv.Options)
	mux.HandleFunc("GET /api/servers/{id}/setup", sv.Setup)
	mux.HandleFunc("GET /api/servers/{id}/ports", sv.Ports)
	mux.HandleFunc("GET /api/servers/{id}/options", sv.ApplyOptions)
	mux.HandleFunc("POST /api/servers/{id}/deploy", dep.Deploy)
	mux.HandleFunc("DELETE /api/servers/{id}", sv.Delete)
	return mux
}
