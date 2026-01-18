package kubelet

import (
	"os/exec"
	"regexp"
)

func GetKubeletVersion() (string, error) {
	rawVersion, err := GetKubeletVersionRaw()
	if err != nil {
		return "", err
	}
	version := parseSemVer(*rawVersion)
	return version, nil
}

func GetKubeletVersionRaw() (*string, error) {
	output, err := exec.Command("kubelet", "--version").Output()
	if err != nil {
		return nil, err
	}
	rawVersion := string(output)
	return &rawVersion, nil
}

var semVerRegex = regexp.MustCompile(`v[0-9]+\.[0-9]+.[0-9]+`)

func parseSemVer(rawVersion string) string {
	return semVerRegex.FindString(rawVersion)
}
