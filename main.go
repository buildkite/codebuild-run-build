package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/buildkite/codebuild-run-build/runner"
	"github.com/urfave/cli"
)

var (
	Version string
)

func main() {
	app := cli.NewApp()
	app.Name = "codebuild-run-build"
	app.Usage = "Run a build on CodeBuild and tail the output from Cloudwatch"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Show debugging information",
		},
		cli.StringFlag{
			Name:  "project-name, n",
			Usage: "Project name",
		},
		cli.BoolFlag{
			Name:  "no-artifacts",
			Usage: "Disable artifacts for this build",
		},
		cli.StringFlag{
			Name:  "source-type-override",
			Usage: "Override the Source Type for this build",
		},
		cli.StringFlag{
			Name:  "source-location-override",
			Usage: "Override the Source Location for this build",
		},
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "Additional environment",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		requireFlagValue(ctx, "project-name")

		if !ctx.Bool("debug") {
			log.SetOutput(ioutil.Discard)
		}

		r := runner.New()
		r.ProjectName = ctx.String("project-name")
		r.Env = ctx.StringSlice("env")
		r.SourceType = ctx.String("source-type-override")
		r.SourceLocation = ctx.String("source-location-override")
		r.NoArtifacts = ctx.Bool("no-artifacts")

		if err := r.Run(); err != nil {
			if ec, ok := err.(cli.ExitCoder); ok {
				return ec
			}
			fmt.Println(err)
			os.Exit(1)
		}
		return nil
	}

	app.Run(os.Args)
}

func requireFlagValue(ctx *cli.Context, name string) {
	if ctx.String(name) == "" {
		fmt.Printf("ERROR: Required flag %q isn't set\n\n", name)
		cli.ShowAppHelpAndExit(ctx, 1)
	}
}
