package configprovider

import (
	"fmt"
	"net/url"
)

// BuildConfigProvider returns a ConfigProvider appropriate for the given source URL.
// The source URL must have a scheme, and the supported schemes are:
// - `file`. To use configuration from the filesystem: `file:///path/to/file/or/directory`.
// - `imds`. To use configuration from the instance's user data: `imds://user-data`.
func BuildConfigProvider(rawConfigSourceURL string) (ConfigProvider, error) {
	parsedURL, err := url.Parse(rawConfigSourceURL)
	if err != nil {
		return nil, err
	}
	switch parsedURL.Scheme {
	case "imds":
		return NewUserDataConfigProvider(), nil
	case "file":
		source := getURLWithoutScheme(parsedURL)
		return NewFileConfigProvider(source), nil
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}
}

func getURLWithoutScheme(url *url.URL) string {
	return fmt.Sprintf("%s%s", url.Host, url.Path)
}
