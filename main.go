package main

import (
	"flag"
	"log"
)

func main() {
	cmd := flag.String("cmd", "", "one of fetch, verify, install")
	cacheDir := flag.String("cache", "", "path to user cache directory")
	tarball := flag.String("tarball", "", "path to tarball")
	sig := flag.String("sig", "", "path to signature")
	flag.Parse()

	var err error

	switch *cmd {
	case "fetch":
		if err = fetch(); err != nil {
			log.Fatal("could not fetch: %w", err)
		}

	case "verify":
		if *tarball == "" || *sig == "" {
			log.Fatal("must provide --tarball and --sig for verify")
		}
		_, err = verify(*tarball, *sig)
		if err != nil {
			log.Fatal("verification failed: %w", err)
		}

	case "install":
		if *cacheDir == "" {
			log.Fatal("install requires --cache")
		}
		if err := install(*cacheDir); err != nil {
			log.Fatal("could not install: %w", err)
		}

	default:
		log.Fatal("must specify --cmd=fetch|verify|install")
	}
}
