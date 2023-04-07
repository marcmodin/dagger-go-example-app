package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
)

// Set the versions of containers used in the pipeline
var (
	goVersion         = "1.20"
	gosyftVersion     = "v0.76.0"
	goreleaserVersion = "v1.16.2"

	// app info
	appName      = "dagger-go-example-app"
	appBuildPath = "dist"

	err      error
	res      string
	is_local bool
)

func init() {
	flag.BoolVar(&is_local, "local", false, "whether to run locally")
	flag.Parse()
}

func main() {

	// Check if a task argument is provided
	if len(flag.Args()) == 0 {
		fmt.Println("Missing argument. Expected either 'pull-request' or 'release'.")
		os.Exit(1)
	}

	// Check if the task argument is valid
	task := flag.Arg(0)
	if task != "pull-request" && task != "release" {
		fmt.Println("Invalid argument. Expected either 'pull-request' or 'release'.")
		os.Exit(1)
	}

	// Dagger client context
	ctx := context.Background()

	// Create a Dagger client
	client, err := dagger.Connect(ctx)
	if err != nil {
		panic(err)
	}

	// Always close the client when done or on error.
	defer func() {
		log.Printf("Closing Dagger client...")
		client.Close()
	}()

	log.Println("Connected to Dagger")

	// Run the corresponding task.
	switch task {
	case "pull-request":
		res, err = pullrequest(ctx, client)
	case "release":
		res, err = release(ctx, client)
	}

	// Handle any errors that occurred during the task execution.
	if err != nil {
		// log.Fatalf("Error %s: %+v\n", task, err)
		panic(fmt.Sprintf("Error %s: %+v\n", task, err))

	}

	log.Println(res)
}

// Pull Request Task: Runs tests and builds the binary
//
// `example: go run ci/dagger.go pull-request `
func pullrequest(ctx context.Context, client *dagger.Client) (string, error) {

	// Get the source code from host directory and exclude files
	directory := client.Host().Directory(".", dagger.HostDirectoryOpts{
		Exclude: []string{"dist", "vendor", ".git", "ci/*", "go.work"},
	})

	// Create a go container with the source code mounted
	golang := client.Container().
		From(fmt.Sprintf("golang:%s-alpine", goVersion)).
		WithMountedDirectory("/src", directory).WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", client.CacheVolume("gomod")).
		WithEnvVariable("CGO_ENABLED", "0")

	// Run go tests
	_, err := golang.WithExec([]string{"go", "test", "./..."}).
		Stderr(ctx)

	if err != nil {
		return "", err
	}

	log.Println("Tests passed successfully!")

	// Run go build to check if the binary compiles
	_, err = golang.WithExec([]string{"go", "build", "-o", fmt.Sprintf("%v/%s", appBuildPath, appName), "."}).Stderr(ctx)

	if err != nil {
		return "", err
	}

	return "Build passed successfully!", nil
}

// Release Task: Runs GoReleaser to creates a Github release
//
// `example: go run ci/dagger.go release flags[-l,--local]`
func release(ctx context.Context, client *dagger.Client) (string, error) {

	// Set the Github token from the host environment as a secret
	token := client.SetSecret("github_token", os.Getenv("GITHUB_TOKEN"))

	// Get the source code from host directory
	directory := client.Host().Directory(".")

	// Export the syft binary from the syft container as a file
	syft := client.Container().From(fmt.Sprintf("anchore/syft:%s", gosyftVersion)).
		WithMountedCache("/go/pkg/mod", client.CacheVolume("gomod")).
		File("/syft")

	// Create the GoReleaser container with the syft binary mounted
	goreleaser := client.Container().From(fmt.Sprintf("goreleaser/goreleaser:%s", goreleaserVersion)).
		WithFile("/bin/syft", syft).
		WithMountedDirectory("/src", directory).WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", client.CacheVolume("gomod")).
		WithEnvVariable("TINI_SUBREAPER", "true").
		WithSecretVariable("GITHUB_TOKEN", token)

	if !is_local {
		// Run Github Release when event is push and ref is a tag
		_, err := goreleaser.WithExec([]string{"--clean"}).Stderr(ctx)

		if err != nil {
			return "", err
		}

		log.Println("Released successfully!")

	} else {
		// If --local is set, run local release with snapshot and publish container to registry for testing
		local := goreleaser.WithExec([]string{"--snapshot", "--clean"})

		_, err := local.Stderr(ctx)

		if err != nil {
			return "", err
		}

		log.Println("Build with snapshot completed successfully!")

		// Retrieve the built linux binary file from the container
		dist := local.Directory(appBuildPath)

		// Export the dist directory when running locally
		_, err = dist.Export(ctx, appBuildPath)

		if err != nil {
			return "", err
		}

		log.Printf("Exported %v to local successfully!", appBuildPath)

		// Retrieve the built linux binary file from the container
		binary := dist.File(fmt.Sprintf("%s_linux_amd64", appName))

		// Publish
		// package the binary into a alpine container for publishing
		publish := client.Container().From("alpine:latest").
			WithFile(fmt.Sprintf("/bin/%v", appName), binary).
			WithWorkdir("/bin").
			WithEntrypoint([]string{fmt.Sprintf("/bin/%v", appName)})

		container_uri, err := publish.Publish(ctx, fmt.Sprintf("ttl.sh/%s:5m", appName))

		if err != nil {
			return "", err
		}

		log.Printf("Published to registry successfully! \n %s ", container_uri)
	}

	return "Release completed successfully!", nil
}
