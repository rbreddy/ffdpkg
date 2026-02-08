package main

import (
	"fmt"
	"os/exec"
	"regexp"
)

func verify(filepath, signaturepath string) (string, error) {
	fmt.Println(filepath, signaturepath)
	cmd := exec.Command("gpg", "--verify", signaturepath, filepath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("signature check failed: %w", err)
	}

	re := regexp.MustCompile(`using [A-Z]+ key ([0-9A-F]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return "", fmt.Errorf("cannot parse key fingerprint from gpg output")
	}
	fingerprint := matches[1]

	return fingerprint, nil
}
