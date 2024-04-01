/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/samber/lo"
)

const (
	repoOwner = "aws"
	repoName  = "karpenter-provider-aws"
)

type Release struct {
	Version `json:"version"`
	GitTag  string `json:"gitTag"`
	ECRTag  string `json:"ecrTag"`
}

type Version struct {
	Major int  `json:"major"`
	Minor int  `json:"minor"`
	Patch int  `json:"patch"`
	RC    *int `json:"releaseCandidate,omitempty"`
}

func NewReleaseFromTag(tag string) (Release, error) {
	re, err := regexp.Compile(`v?(\d+)\.(\d+)\.(\d+)(?:-rc\.)?(\d+)?`)
	if err != nil {
		return Release{}, err
	}
	matches := re.FindAllStringSubmatch(tag, -1)
	major, err := strconv.Atoi(matches[0][1])
	if err != nil {
		return Release{}, err
	}
	minor, err := strconv.Atoi(matches[0][2])
	if err != nil {
		return Release{}, err
	}
	patch, err := strconv.Atoi(matches[0][3])
	if err != nil {
		return Release{}, err
	}
	var rc *int
	if matches[0][4] != "" {
		val, err := strconv.Atoi(matches[0][4])
		if err != nil {
			return Release{}, err
		}
		rc = &val
	}
	result := Release{
		Version: Version{
			Major: major,
			Minor: minor,
			Patch: patch,
			RC:    rc,
		},
		GitTag: tag,
		ECRTag: tag,
	}
	if releaseSortFunc(result, Release{
		Version: Version{
			Major: 0,
			Minor: 35,
			Patch: 0,
			RC:    nil,
		},
	}) >= 0 {
		result.ECRTag = strings.TrimLeft(tag, "v")
	}
	return result, nil
}

var releaseSortFunc = func(a, b Release) int {
	if a.Major != b.Major {
		return lo.Ternary(a.Major > b.Major, 1, -1)
	}
	if a.Minor != b.Minor {
		return lo.Ternary(a.Minor > b.Minor, 1, -1)
	}
	if a.Patch != b.Patch {
		return lo.Ternary(a.Patch > b.Patch, 1, -1)
	}
	if a.RC != nil && b.RC != nil {
		return lo.Ternary(*a.RC > *b.RC, 1, -1)
	}
	if a.RC != nil || b.RC != nil {
		return lo.Ternary(a.RC == nil, 1, -1)
	}
	return 0
}

// Invariant: elements must be in descending order
type ReleaseSet []Release

// NewReleaseSetFromTags constructs a VersionSet from tags in the format v{MAJOR}.{MINOR}.{PATCH}-rc.{RC} (rc suffix optional)
func NewReleaseSetFromTags(tags ...string) (ReleaseSet, error) {
	vs := ReleaseSet{}
	for _, tag := range tags {
		version, err := NewReleaseFromTag(tag)
		if err != nil {
			return nil, err
		}
		vs = append(vs, version)
	}
	slices.SortFunc(vs, func(a, b Release) int {
		return -1 * releaseSortFunc(a, b)
	})
	return vs, nil
}

// GetLastMinorReleases returns the latest patch release for the newest n minor versions
func (rs ReleaseSet) GetLastMinorReleases(n int) ReleaseSet {
	last := Release{}
	latest := ReleaseSet{}
	for _, r := range rs {
		if len(latest) == n {
			break
		}
		if last.Major != r.Major || last.Minor != r.Minor {
			latest = append(latest, r)
			last = r
		}
	}
	return latest
}

func GetReleases(ctx context.Context, client *github.Client, owner string, repo string) (ReleaseSet, error) {
	releases := []*github.RepositoryRelease{}
	page := 0
	for {
		results, resp, err := client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
			Page: page,
		})
		if err != nil {
			return nil, err
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
		releases = append(releases, results...)
	}
	return NewReleaseSetFromTags(lo.Map(releases, func(release *github.RepositoryRelease, _ int) string {
		return lo.FromPtr(release.TagName)
	})...)
}

type Options struct {
	LatestCount int
}

func GetOptions() Options {
	opts := Options{}
	flag.IntVar(&opts.LatestCount, "latest", -1, "When specified, only output the latest patch release for the last n minor releases.")
	flag.Parse()
	return opts
}

func main() {
	opts := GetOptions()
	ctx := context.Background()
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	versions, err := GetReleases(ctx, client, repoOwner, repoName)
	if err != nil {
		log.Fatalf("getting releases, %s\n", err)
	}
	if opts.LatestCount != -1 {
		versions = versions.GetLastMinorReleases(opts.LatestCount)
	}
	err = json.NewEncoder(os.Stdout).Encode(versions)
	if err != nil {
		log.Fatalf("encoding releases, %s\n", err)
	}
}
