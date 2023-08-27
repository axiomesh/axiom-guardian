package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/axiomesh/axiom-kit/log"
	"github.com/axiomesh/guardian"
	"github.com/axiomesh/guardian/core"
	"github.com/axiomesh/guardian/repo"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"
)

func start(ctx *cli.Context) error {
	p, err := getRootPath(ctx)
	if err != nil {
		return err
	}
	r, err := repo.Load(p)
	if err != nil {
		return err
	}

	err = log.Initialize(
		log.WithReportCaller(r.Config.Log.ReportCaller),
		log.WithPersist(true),
		log.WithFilePath(filepath.Join(r.Config.RepoRoot, repo.LogsDirName)),
		log.WithFileName(r.Config.Log.Filename),
		log.WithMaxAge(r.Config.Log.MaxAge),
		log.WithRotationTime(r.Config.Log.RotationTime),
	)
	if err != nil {
		return fmt.Errorf("log initialize: %w", err)
	}

	printVersion()

	client, err := ethclient.DialContext(ctx.Context, r.Config.DialUrl)
	if err != nil {
		return err
	}

	guardian, err := core.NewGuardian(ctx.Context, r.Config, client)
	if err != nil {
		return fmt.Errorf("new guardian error: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	handleShutdown(guardian, &wg)

	if err := guardian.Start(); err != nil {
		return fmt.Errorf("start guardian failed: %w", err)
	}

	fmt.Println("=============Guardian is ready=============")

	wg.Wait()

	return nil
}

func printVersion() {
	fmt.Printf("Guardian version: %s-%s-%s\n", guardian.CurrentVersion, guardian.CurrentBranch, guardian.CurrentCommit)
	fmt.Printf("App build date: %s\n", guardian.BuildDate)
	fmt.Printf("System version: %s\n", guardian.Platform)
	fmt.Printf("Golang version: %s\n", guardian.GoVersion)
	fmt.Println()
}

func handleShutdown(node *core.Guardian, wg *sync.WaitGroup) {
	var stop = make(chan os.Signal, 2)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)

	go func() {
		<-stop
		fmt.Println("received interrupt signal, shutting down...")
		if err := node.Stop(); err != nil {
			panic(err)
		}
		wg.Done()
		os.Exit(0)
	}()
}
