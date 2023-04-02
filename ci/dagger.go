package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	// "os/exec"

	"dagger.io/dagger"
)

// Set the versions of containers used in the pipeline
const (
	// https://hub.docker.com/_/golang
	goVersion = "1.20"

	// https://github.com/golangci/golangci-lint/releases
	golangciLintVersion = "1.52.2"

	// app info
	appName      = "dagger-go-example-app"
	appVersion   = "0.0.1"
	appBuildPath = "dist/"
)

var (
	token = os.Getenv("GITHUB_TOKEN")
	event = os.Getenv("GITHUB_EVENT")
	ref   = os.Getenv("GITHUB_REF")
)

func init() {
	// parse flags
	flag.Parse()
}

func main() {
	ctx := context.Background()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	log.Println("Connected to Dagger")

	fmt.Println(ref)
	fmt.Println(event)

	if event == "push" && strings.Contains(ref, "/tags/v") {
		fmt.Println("Push event detected")
	} else if event == "pull_request" {
		fmt.Println("Pull request event detected")
	} else {
		fmt.Println("Unknown event detected")
	}

	// Get the source code from host directory
	directory := client.Host().Directory(".", dagger.HostDirectoryOpts{
		// Exclude: []string{
		// 	"LICENSE",
		// 	"README.md",
		// 	"go.sum",
		// 	"go.work",
		// 	"ci/*",
		// },
	})

	// Release

	if event == "push" && strings.Contains(ref, "/tags/v") {
		fmt.Println("Push event detected")

		// Export the syft binary from the container as a file
		syft := client.Container().From("anchore/syft:latest").File("/syft")

		// Run GoReleaser with Syft
		release, err := client.Container().From("goreleaser/goreleaser:latest").
			WithFile("/bin/syft", syft).
			WithMountedDirectory("/src", directory).WithWorkdir("/src").
			WithEnvVariable("TINI_SUBREAPER", "true").
			WithEnvVariable("GITHUB_TOKEN", token).
			WithExec([]string{"--clean"}).
			Stdout(ctx)

		if err != nil {
			panic(err)
		}
		fmt.Println(release)

		// output := release.Directory(appBuildPath)

		// // write contents of container build/ directory to the host
		// _, err = output.Export(ctx, appBuildPath)

		// if err != nil {
		// 	panic(err)
		// }

		log.Println("Release completed successfully!")
	}
	// // Lint
	// lint, err := client.Container().
	// 	From(fmt.Sprintf("golangci/golangci-lint:v%s-alpine", golangciLintVersion)).
	// 	WithMountedCache("/go/pkg/mod", client.CacheVolume("gomod")).
	// 	WithMountedDirectory("/src", directory).WithWorkdir("/src").
	// 	WithExec([]string{"golangci-lint", "run", "--color", "always", "--timeout", "2m"}).
	// 	ExitCode(ctx)

	// if err != nil {
	// 	panic(err)
	// }

	// if lint != 0 {
	// 	panic(err)
	// }

	// log.Println("Linter passed successfully!")

	// // Test
	// golang := client.Container().
	// 	From(fmt.Sprintf("golang:%s-alpine", goVersion)).
	// 	WithMountedDirectory("/src", directory).WithWorkdir("/src").
	// 	WithMountedCache("/go/pkg/mod", client.CacheVolume("gomod")).
	// 	WithEnvVariable("CGO_ENABLED", "0")

	// test, err := golang.WithExec([]string{"go", "test", "./..."}).
	// 	ExitCode(ctx)

	// if err != nil {
	// 	panic(err)
	// }

	// if test != 0 {
	// 	panic(err)
	// }

	// log.Println("Tests passed successfully!")

	// // Build
	// builder := golang.WithExec([]string{"go", "build", "-o", fmt.Sprintf("%v/%s", appBuildPath, appName), "."})

	// build, err := builder.ExitCode(ctx)

	// if err != nil {
	// 	panic(err)
	// }

	// if build != 0 {
	// 	panic(err)
	// }

	// log.Println("Built binary successfully!")

	// // Publish
	// // package the binary into a alpine container for publishing
	// publish := client.Container().From("alpine:latest").
	// 	WithFile(fmt.Sprintf("/bin/%v", appName), builder.File(fmt.Sprintf("/src/%v/%s", appBuildPath, appName))).
	// 	WithWorkdir("/bin").
	// 	WithEntrypoint([]string{fmt.Sprintf("/bin/%v", appName)})

	// image_uri, err := publish.Publish(ctx, fmt.Sprintf("ttl.sh/%s-%s:5m", appName, appVersion))

	// if err != nil {
	// 	panic(err)
	// }

	// log.Printf("Published successfully! \n %s ", image_uri)

}
