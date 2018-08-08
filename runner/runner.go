package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/codebuild"
)

type Runner struct {
	ProjectName    string
	LogGroupName   string
	Region         string
	Env            []string
	SourceType     string
	SourceLocation string
	NoArtifacts    bool
	Config         *aws.Config
}

func New() *Runner {
	return &Runner{
		Region: os.Getenv("AWS_REGION"),
		Config: aws.NewConfig(),
	}
}

func (r *Runner) Run() error {
	sess := session.Must(session.NewSession(r.Config))
	svc := codebuild.New(sess)

	input := &codebuild.StartBuildInput{
		ProjectName:                  aws.String(r.ProjectName),
		EnvironmentVariablesOverride: []*codebuild.EnvironmentVariable{},
	}

	if len(r.Env) > 0 {
		for _, env := range r.Env {
			log.Printf("Setting env %s for this build", env)
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				log.Printf("Encountered invalid env %q", env)
				continue
			}
			input.EnvironmentVariablesOverride = append(input.EnvironmentVariablesOverride, &codebuild.EnvironmentVariable{
				Name:  aws.String(parts[0]),
				Value: aws.String(parts[1]),
			})
		}
	}

	if r.NoArtifacts {
		log.Printf("Disabling artifacts for this build")
		input.ArtifactsOverride = &codebuild.ProjectArtifacts{
			Type: aws.String("NO_ARTIFACTS"),
		}
	}

	if r.SourceType != "" {
		log.Printf("Setting source type of %q for this build", r.SourceType)
		input.SourceTypeOverride = aws.String(r.SourceType)
	}

	if r.SourceLocation != "" {
		log.Printf("Setting source location of %q for this build", r.SourceLocation)
		input.SourceLocationOverride = aws.String(r.SourceLocation)
	}

	log.Printf("Creating a build for %s", r.ProjectName)
	startBuildOutput, err := svc.StartBuild(input)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Build %s started", *startBuildOutput.Build.Id)
	logs, err := waitForLogs(ctx, svc, *startBuildOutput.Build.Id)
	if err != nil {
		return err
	}

	buildComplete := make(chan codebuild.Build)
	errs := make(chan error)
	logEvents := make(chan cloudwatchlogs.FilteredLogEvent)

	lw := &logWatcher{
		LogGroup:      *logs.GroupName,
		LogStreamName: *logs.StreamName,
		Svc:           cloudwatchlogs.New(sess),
		Printer: func(event *cloudwatchlogs.FilteredLogEvent, c context.CancelFunc) {
			logEvents <- *event
		},
	}

	log.Printf("Watching %s/%s", *logs.GroupName, *logs.StreamName)
	go func() {
		if err := lw.Watch(ctx); err != nil {
			errs <- err
		}
	}()

	log.Printf("Waiting for build to complete")
	go func() {
		build, err := waitForBuildComplete(ctx, svc, *startBuildOutput.Build.Id)
		if err != nil {
			errs <- err
		} else {
			buildComplete <- build
		}
	}()

	for {
		select {
		case event := <-logEvents:
			fmt.Print(*event.Message)
		case err := <-errs:
			cancel()
			return err
		case build := <-buildComplete:
			cancel()
			log.Printf("Build finished with status %s", *build.BuildStatus)
			buildErr := fmt.Errorf("Build finished with status %s", *build.BuildStatus)

			switch *build.BuildStatus {
			case "FAILED":
				return exitError{buildErr, 1}
			case "FAULT":
				return exitError{buildErr, 2}
			case "STOPPED":
				return exitError{buildErr, 3}
			case "TIMED_OUT":
				return exitError{buildErr, 4}
			}
			return nil
		}
	}
}

type exitError struct {
	error
	exitCode int
}

func (ee *exitError) ExitCode() int {
	return ee.exitCode
}

func waitForLogs(ctx context.Context, svc *codebuild.CodeBuild, buildID string) (codebuild.LogsLocation, error) {
	for {
		select {
		case <-time.After(10 * time.Second):
			getBuildOutput, err := svc.BatchGetBuildsWithContext(ctx, &codebuild.BatchGetBuildsInput{
				Ids: []*string{
					aws.String(buildID),
				},
			})
			if err != nil {
				return codebuild.LogsLocation{}, err
			}

			if len(getBuildOutput.Builds) == 0 {
				return codebuild.LogsLocation{}, errors.New("No builds found")
			}

			if getBuildOutput.Builds[0].Logs != nil {
				return *getBuildOutput.Builds[0].Logs, nil
			}

		case <-ctx.Done():
			return codebuild.LogsLocation{}, errors.New("Context done")
		}
	}
}

func waitForBuildComplete(ctx context.Context, svc *codebuild.CodeBuild, buildID string) (codebuild.Build, error) {
	for {
		select {
		case <-time.After(10 * time.Second):
			getBuildOutput, err := svc.BatchGetBuildsWithContext(ctx, &codebuild.BatchGetBuildsInput{
				Ids: []*string{
					aws.String(buildID),
				},
			})
			if err != nil {
				return codebuild.Build{}, err
			}

			if len(getBuildOutput.Builds) == 0 {
				return codebuild.Build{}, errors.New("No builds found")
			}

			if *getBuildOutput.Builds[0].BuildComplete {
				return *getBuildOutput.Builds[0], nil
			}

		case <-ctx.Done():
			return codebuild.Build{}, errors.New("Context done")
		}
	}
}
