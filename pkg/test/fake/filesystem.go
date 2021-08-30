package fake

import (
	"fmt"
	"path/filepath"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/spf13/afero"
)

func Filesystem() afero.Fs {
	fakeFS := afero.NewMemMapFs()
	if err := fakeFS.MkdirAll(filepath.Dir(v1alpha3.InClusterCABundlePath), 0755); err != nil {
		panic(fmt.Sprintf("unable to make directory for %s: %v", v1alpha3.InClusterCABundlePath, err))
	}
	if err := afero.WriteFile(fakeFS, v1alpha3.InClusterCABundlePath, []byte("fake CA Bundle data"), 0644); err != nil {
		panic(fmt.Sprintf("unable to write file %s: %v", v1alpha3.InClusterCABundlePath, err))
	}
	return fakeFS
}
