package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

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
		cli.StringFlag{
			Name:  "env-file",
			Usage: "Additional environment in a file in KEY=\"value\" form",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		requireFlagValue(ctx, "project-name")

		if !ctx.Bool("debug") {
			log.SetOutput(ioutil.Discard)
		}

		r := runner.New()
		r.ProjectName = ctx.String("project-name")
		r.SourceType = ctx.String("source-type-override")
		r.SourceLocation = ctx.String("source-location-override")
		r.NoArtifacts = ctx.Bool("no-artifacts")

		if ctx.IsSet("env-file") {
			env, err := readEnvFile(ctx.String("env-file"))
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			r.Env = append(r.Env, env...)
		}

		if ctx.IsSet("env") {
			for _, s := range ctx.StringSlice("env") {
				env, err := parseEnv(s)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				r.Env = append(r.Env, env)
			}
		}

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

func parseEnv(s string) (runner.Env, error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return runner.Env{}, fmt.Errorf("Failed to parse env %q", s)
	}
	// parse as json to handle quotes and special chars
	var marshaled string
	err := json.Unmarshal([]byte(parts[1]), &marshaled)
	if err != nil {
		return runner.Env{}, err
	}
	return runner.Env{Name: parts[0], Value: marshaled}, nil
}

func readEnvFile(fileName string) ([]runner.Env, error) {
	var envs []runner.Env

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		env, err := parseEnv(scanner.Text())
		if err != nil {
			return nil, err
		}
		envs = append(envs, env)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return envs, nil
}
