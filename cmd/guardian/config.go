package main

import (
	"fmt"
	"os"

	"github.com/axiomesh/guardian/repo"
	"github.com/urfave/cli/v2"
)

var configCMD = &cli.Command{
	Name:  "config",
	Usage: "The config manage commands",
	Subcommands: []*cli.Command{
		{
			Name:   "generate",
			Usage:  "Generate default config",
			Action: generate,
		},
		{
			Name:   "show",
			Usage:  "Show the complete config processed by the environment variable",
			Action: show,
		},
		{
			Name:   "check",
			Usage:  "Check if the config file is valid",
			Action: check,
		},
		{
			Name:   "rewrite-with-env",
			Usage:  "Rewrite config with env",
			Action: rewriteWithEnv,
		},
	},
}

func generate(ctx *cli.Context) error {
	p, err := getRootPath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Open(p); err == nil {
		fmt.Println("guardian repo already exists")
		return nil
	}

	err = os.MkdirAll(p, 0755)
	if err != nil {
		return err
	}

	defaultConfig := repo.DefaultConfig(p)
	
	repo := &repo.Repo{
		Config: defaultConfig,
	}

	if err := repo.Flush(); err != nil {
		return err
	}

	fmt.Printf("initializing guardian at %s\n", p)
	return nil
}

func show(ctx *cli.Context) error {
	p, err := getRootPath(ctx)
	if err != nil {
		return err
	}
	existConfig := repo.Exist(p)
	if !existConfig {
		fmt.Println("guardian repo not exist")
		return nil
	}

	r, err := repo.Load(p)
	if err != nil {
		return err
	}
	str, err := repo.MarshalConfig(r.Config)
	if err != nil {
		return err
	}
	fmt.Println(str)
	return nil
}

func check(ctx *cli.Context) error {
	p, err := getRootPath(ctx)
	if err != nil {
		return err
	}
	existConfig := repo.Exist(p)
	if !existConfig {
		fmt.Println("guardian repo not exist")
		return nil
	}

	_, err = repo.Load(p)
	if err != nil {
		fmt.Println("config file format error, please check:", err)
		os.Exit(1)
		return nil
	}

	return nil
}

func rewriteWithEnv(ctx *cli.Context) error {
	p, err := getRootPath(ctx)
	if err != nil {
		return err
	}
	existConfig := repo.Exist(p)
	if !existConfig {
		fmt.Println("guardian repo not exist")
		return nil
	}

	r, err := repo.Load(p)
	if err != nil {
		return err
	}
	if err := r.Flush(); err != nil {
		return err
	}
	return nil
}

func getRootPath(ctx *cli.Context) (string, error) {
	p := ctx.String("repo")

	var err error
	if p == "" {
		p, err = repo.LoadRepoRootFromEnv(p)
		if err != nil {
			return "", err
		}
	}
	return p, nil
}
