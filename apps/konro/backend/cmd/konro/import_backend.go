package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/llmopsclient"
)

type retainedEndpoint struct {
	Kind             string `json:"kind"`
	URL              string `json:"url"`
	ClientID         string `json:"client_id"`
	TokenURL         string `json:"token_url"`
	ClientSecretFile string `json:"client_secret_file"`
	TokenFile        string `json:"token_file"`
	CAFile           string `json:"ca_file"`
}

func importRouting(ctx context.Context, direct agentclient.Gateway) (importer.Routing, error) {
	kind := envOr("KONRO_IMPORT_BACKEND", "direct")
	if kind != "direct" && kind != "platform" {
		return importer.Routing{}, errors.New("invalid import backend mode")
	}
	legacy := importer.Backend{Kind: "direct", EndpointID: llmopsclient.EndpointID("direct", os.Getenv("KONRO_AGENT_URL"), os.Getenv("KONRO_AGENT_CLIENT_ID")), Client: direct}
	platform, e := llmopsclient.FromEnvironment(ctx)
	if e != nil {
		log.Print("Platform AI接続を利用できません。元の接続への自動切替は行いません")
		platform = nil
	}
	next := importer.Backend{Kind: "platform", EndpointID: llmopsclient.EndpointID("platform", os.Getenv("KONRO_LLMOPS_URL"), os.Getenv("KONRO_LLMOPS_CLIENT_ID")), Client: platform}
	routing := importer.Routing{Current: legacy, LegacyDirect: legacy, Retained: []importer.Backend{next}}
	if kind == "platform" {
		routing.Current = next
	}
	if file := os.Getenv("KONRO_IMPORT_RETAINED_FILE"); file != "" {
		f, e := os.Open(file)
		if e != nil {
			return routing, errors.New("retained import endpoint configuration unavailable")
		}
		defer f.Close()
		var endpoints []retainedEndpoint
		d := json.NewDecoder(io.LimitReader(f, 1<<20))
		d.DisallowUnknownFields()
		if d.Decode(&endpoints) != nil || d.Decode(new(any)) != io.EOF || len(endpoints) > 32 {
			return routing, errors.New("invalid retained import endpoint configuration")
		}
		for _, entry := range endpoints {
			var client agentclient.Gateway
			var err error
			switch entry.Kind {
			case "direct":
				client, err = agentclient.FromConfig(ctx, agentclient.Config{URL: entry.URL, TokenFile: entry.TokenFile, TokenURL: entry.TokenURL, ClientID: entry.ClientID, ClientSecretFile: entry.ClientSecretFile, CAFile: entry.CAFile})
			case "platform":
				client, err = llmopsclient.FromConfig(ctx, llmopsclient.Config{URL: entry.URL, TokenURL: entry.TokenURL, ClientID: entry.ClientID, ClientSecretFile: entry.ClientSecretFile, CAFile: entry.CAFile})
			default:
				return routing, errors.New("invalid retained backend kind")
			}
			if err != nil {
				log.Print("保存済みjob用のAI接続を利用できません。下書きは保持します")
				client = nil
			}
			routing.Retained = append(routing.Retained, importer.Backend{Kind: entry.Kind, EndpointID: llmopsclient.EndpointID(entry.Kind, entry.URL, entry.ClientID), Client: client})
		}
	}
	return routing, nil
}
