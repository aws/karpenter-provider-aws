Operatorpkg is a set of packages used to develop Kubernetes operators at AWS. It contains opinions on top of existing projects like https://github.com/kubernetes/apimachinery and https://github.com/kubernetes-sigs/controller-runtime. In many cases, we plan to mature packages in operatorpkg before commiting them upstream.

We strive to maintain a relatively minimal dependency footprint, but some dependencies are necessary to provide value. 

## Maintainers

Maintainers are limited to AWS employees, but we may consider external contributions and bug fixes. This project is maintained in service of a set of projects well known to the maintainers. We will not consider feature requests unless they are in direct support of these projects. For example, we are delighted to accept contributions from the community behind https://github.com/kubernetes-sigs/karpenter. Before depending on this package, please speak with the maintainers.

## Versioning

* We respect the standards defined at https://semver.org/.
* We model releases using github tags and create branches for each minor version.
* We use dependabot to keep dependencies up to date.
* We do not guarantee that patches will be backported to minor versions.
* SEMVER4: Major version zero (0.y.z) is for initial development. Anything MAY change at any time. The public API SHOULD NOT be considered stable.
