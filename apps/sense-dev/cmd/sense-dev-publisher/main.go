package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/deploymentobserver"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisherbridge"
)

func main() {
	data := flag.String("data", "", "controller state directory (publisher user needs read/write)")
	decision := flag.String("decision", "", "live independent Release Gate decision ID")
	repo := flag.String("repo", "", "trusted local repository with approved commit")
	token := flag.String("token-file", "", "private repository-scoped GitHub token file")
	packageToken := flag.String("package-token-file", "", "owner classic read:packages-only mode0600 token for package metadata and GHCR pull")
	askpass := flag.String("askpass", "", "trusted Git askpass executable for branch push")
	socket := flag.String("serve-socket", "", "private Unix socket for authenticated controller; owns no state lock")
	bridgeAuth := flag.String("controller-auth-file", "", "publisher copy of dedicated bridge authentication secret")
	bundleTransfer := flag.Bool("commit-transfer", false, "accept fixed credentialless bundles and return verified merged graphs")
	socketGroup := flag.Int("socket-group", -1, "optional shared controller/publisher Unix group ID")
	observerPlan := flag.String("observer-plan", "", "fixed private canary observation plan")
	observerKubeconfig := flag.String("observer-kubeconfig", "", "dedicated read-only mode0600 kubeconfig")
	observerKubectl := flag.String("observer-kubectl", "kubectl", "trusted host kubectl executable")
	observerToken := flag.String("observer-token-file", "", "private repository-limited GET-only GitHub observation token")
	observerPackageToken := flag.String("observer-package-token-file", "", "separate owner classic read:packages-only mode0600 observation token")
	flag.Parse()
	if *socket != "" {
		if *data != "" || *decision != "" || *bridgeAuth == "" || *token == "" || *packageToken == "" {
			log.Fatal("daemon requires bridge auth and separate repository/package tokens; data and decision are forbidden")
		}
		var observe publisherbridge.ObservationFunc
		if *observerPlan != "" || *observerKubeconfig != "" || *observerToken != "" || *observerPackageToken != "" {
			if *observerPlan == "" || *observerKubeconfig == "" || *observerToken == "" || *observerPackageToken == "" {
				log.Fatal("observer plan, read-only kubeconfig, repository and package observation tokens required together")
			}
			plan, err := deploymentobserver.LoadPlan(*observerPlan)
			if err != nil {
				log.Fatal(err)
			}
			observer := deploymentobserver.Observer{Sources: deploymentobserver.HostSources{Kube: deploymentobserver.KubeCLI{Binary: *observerKubectl, Kubeconfig: *observerKubeconfig}, ReleaseSource: deploymentobserver.GitHubRelease{TokenFile: *observerToken, PackageTokenFile: *observerPackageToken}}}
			observe = func(ctx context.Context, head, revision string, spec core.ImageDeploymentSpec) (core.DeploymentReceipt, error) {
				return observer.Observe(ctx, plan, head, revision, spec)
			}
		}
		var transfers publisherbridge.Transfers
		if *bundleTransfer {
			if *repo == "" || *askpass == "" {
				log.Fatal("commit transfer requires fixed trusted repository and askpass")
			}
			transfers = publisherbridge.Transfers{RepoPath: *repo, Export: func(ctx context.Context, task, base, revision string) (committransfer.Bundle, error) {
				return committransfer.FetchMerged(ctx, task, base, revision, *askpass)
			}}
		}
		handler, err := publisherbridge.HandlerWithTransfers(*bridgeAuth, publisher.GitHub{RepoPath: *repo, TokenFile: *token, PackageTokenFile: *packageToken, Askpass: *askpass}, observe, transfers, publisher.GitHub{RepoPath: *repo, TokenFile: *token, PackageTokenFile: *packageToken, Askpass: *askpass}.ImageEvidence)
		if err != nil {
			log.Fatal(err)
		}
		listener, err := publisherbridge.Listen(*socket, *socketGroup)
		if err != nil {
			log.Fatal(err)
		}
		defer listener.Close()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 2 * time.Minute, MaxHeaderBytes: 8192}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	}
	if *bundleTransfer || *bridgeAuth != "" || *socketGroup != -1 || *observerPlan != "" || *observerKubeconfig != "" || *observerToken != "" || *observerPackageToken != "" {
		log.Fatal("bridge options require serve-socket")
	}
	if *data == "" || *decision == "" || *token == "" || *packageToken == "" {
		log.Fatal("data, decision and separate repository/package token files are required")
	}
	store, err := core.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := store.RunPublish(ctx, *decision, publisher.GitHub{RepoPath: *repo, TokenFile: *token, PackageTokenFile: *packageToken, Askpass: *askpass})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "%s %s %s\n", result.ID, result.Status, result.ExternalID)
}
