// Seed public synthetic captures using the API's limited object role, not root.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"os"
	"strings"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return objects.ErrInvalid
	}
	b, e := os.ReadFile(os.Args[1])
	if e != nil {
		return e
	}
	var creds struct {
		Access string `json:"access_key"`
		Secret string `json:"secret_key"`
	}
	if json.Unmarshal(b, &creds) != nil {
		return objects.ErrInvalid
	}
	s3, e := objects.NewS3(objects.S3Config{App: "konro", Endpoint: "http://127.0.0.1:19300", Bucket: "llmops-dev", Region: "us-east-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: creds.Access, SecretAccessKey: creds.Secret}, nil
	})})
	if e != nil {
		return e
	}
	store, e := objects.NewStore("konro", s3)
	if e != nil {
		return e
	}
	inputs := []map[string]string{}
	expected := map[string]string{}
	for _, item := range []struct{ Key, Title, Ingredient string }{{"onion", "Onion Soup", "onion 1/2"}, {"tomato", "Tomato Soup", "tomato 2"}} {
		body := "Slice and simmer."
		if item.Key == "tomato" {
			body = strings.Repeat("tomato ", 20000)
		}
		raw := []byte(`<html><h1>` + item.Title + `</h1><p>` + body + `</p><script type="application/ld+json">{"@type":"Recipe","name":"` + item.Title + `","recipeYield":"2","recipeIngredient":["` + item.Ingredient + `","water 100 ml"],"recipeInstructions":["Slice.","Simmer."]}</script></html>`)
		digest := sha256.Sum256(raw)
		capture, _, e := store.Capture(context.Background(), objects.CaptureInput{OwnerSubject: "konro-app", OwnerRole: "app", Key: "curate-fixture-v2-" + item.Key, MIME: "text/html", SourceURI: "https://example.invalid/recipes/" + item.Key, SHA256: hex.EncodeToString(digest[:])}, bytes.NewReader(raw))
		if e != nil {
			return e
		}
		verified, e := store.GetArtifact(context.Background(), capture.Object.ObjectID)
		if e != nil || !bytes.Equal(verified.Bytes, raw) {
			return objects.ErrCorrupt
		}
		expected[capture.CaptureID] = capture.Object.SHA256
		inputs = append(inputs, map[string]string{"capture_id": capture.CaptureID, "source_group": "fixture-" + item.Key})
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"inputs": inputs, "sha256_by_capture": expected})
}
