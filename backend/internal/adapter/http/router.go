package http

import nethttp "net/http"

// NewRouter maps the GitHub connection endpoints the local UI calls:
//
//	POST   /api/github/exchange  exchange an OAuth code for a stored token
//	GET    /api/github/status    report the connected account, if any
//	GET    /api/github/repos     list the connected account's repositories
//	DELETE /api/github/token     forget the stored token
func NewRouter(gh *GitHubHandler) *nethttp.ServeMux {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("POST /api/github/exchange", gh.Exchange)
	mux.HandleFunc("GET /api/github/status", gh.Status)
	mux.HandleFunc("GET /api/github/repos", gh.Repos)
	mux.HandleFunc("DELETE /api/github/token", gh.Disconnect)
	return mux
}
