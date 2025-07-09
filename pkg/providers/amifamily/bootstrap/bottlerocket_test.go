package bootstrap

import "testing"

func TestBottlerocket_Script(t *testing.T) {
	var userData = `
[settings.kubernetes]
kube-api-qps = 30
`
	b := Bottlerocket{
		Options: Options{
			CustomUserData: &userData,
		},
	}
	b.Script()
}
