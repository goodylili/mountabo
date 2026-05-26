package http

import nethttp "net/http"

// NewRouter maps the GitHub connection endpoints the local UI calls:
//
//	POST   /api/github/exchange      exchange an OAuth code for a stored token
//	GET    /api/github/status        report the connected account, if any
//	GET    /api/github/repos         list the connected account's repositories
//	DELETE /api/github/token         forget the stored token
//	POST   /api/servers              add a server (probe specs over SSH)
//	GET    /api/servers              list added servers
//	GET    /api/servers/{id}/setup   run the bootstrap, streaming progress (SSE)
//	DELETE /api/servers/{id}         remove a server and destroy its secrets
func NewRouter(gh *GitHubHandler, sv *ServersHandler) *nethttp.ServeMux {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("POST /api/github/exchange", gh.Exchange)
	mux.HandleFunc("GET /api/github/status", gh.Status)
	mux.HandleFunc("GET /api/github/repos", gh.Repos)
	mux.HandleFunc("DELETE /api/github/token", gh.Disconnect)

	mux.HandleFunc("POST /api/servers", sv.Add)
	mux.HandleFunc("GET /api/servers", sv.List)
	mux.HandleFunc("GET /api/servers/options", sv.Options)
	mux.HandleFunc("GET /api/servers/{id}/setup", sv.Setup)
	mux.HandleFunc("GET /api/servers/{id}/options", sv.ApplyOptions)
	mux.HandleFunc("DELETE /api/servers/{id}", sv.Delete)
	return mux
}
