package github

import "testing"

func TestParseComposePorts(t *testing.T) {
	compose := `
services:
  web:
    ports:
      - "${FRONTEND_PORT:-3000}:3000"
  api:
    ports:
      - "$API_PORT:8080/tcp"
  db:
    ports:
      - "5432:5432"
  cache:
    ports:
      - target: 6379
        published: "${REDIS_PORT}"
  worker:
    ports:
      - "9000"
`
	ports, err := parseComposePorts(compose)
	if err != nil {
		t.Fatalf("parseComposePorts: %v", err)
	}

	// services are sorted: api, cache, db, web, worker
	by := map[string]int{}
	for i, p := range ports {
		by[p.Service] = i
	}
	if len(ports) != 5 {
		t.Fatalf("got %d ports, want 5: %+v", len(ports), ports)
	}

	web := ports[by["web"]]
	if web.EnvVar != "FRONTEND_PORT" || web.Host != "3000" || web.Container != "3000" || !web.Editable {
		t.Errorf("web port wrong: %+v", web)
	}

	api := ports[by["api"]]
	if api.EnvVar != "API_PORT" || api.Container != "8080" || !api.Editable {
		t.Errorf("api port (bare $VAR + /tcp) wrong: %+v", api)
	}

	db := ports[by["db"]]
	if db.EnvVar != "" || db.Host != "5432" || db.Editable {
		t.Errorf("db literal port should be read-only: %+v", db)
	}

	cache := ports[by["cache"]]
	if cache.EnvVar != "REDIS_PORT" || cache.Container != "6379" || !cache.Editable {
		t.Errorf("cache long-form port wrong: %+v", cache)
	}

	worker := ports[by["worker"]]
	if worker.Container != "9000" || worker.Host != "" || worker.Editable {
		t.Errorf("worker container-only port wrong: %+v", worker)
	}
}

func TestParseDockerfileExpose(t *testing.T) {
	dockerfile := `
FROM golang:1.22
EXPOSE 8080 9090/tcp
RUN echo "not a port"
EXPOSE 3000
`
	ports := parseDockerfileExpose(dockerfile)
	if len(ports) != 3 {
		t.Fatalf("got %d ports, want 3: %+v", len(ports), ports)
	}
	for _, p := range ports {
		if p.Editable || p.EnvVar != "" {
			t.Errorf("EXPOSE ports must be read-only: %+v", p)
		}
	}
	if ports[0].Container != "8080" || ports[1].Container != "9090" || ports[2].Container != "3000" {
		t.Errorf("unexpected EXPOSE container ports: %+v", ports)
	}
}

func TestParseComposePorts_NoServices(t *testing.T) {
	ports, err := parseComposePorts("version: '3'\n")
	if err != nil {
		t.Fatalf("parseComposePorts: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports, got %+v", ports)
	}
}
