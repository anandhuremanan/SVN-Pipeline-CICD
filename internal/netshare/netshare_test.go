package netshare_test

import (
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/netshare"
)

func TestShareRoot(t *testing.T) {
	cases := []struct {
		path string
		root string
		unc  bool
	}{
		{`\\SERVER\Builds`, `\\SERVER\Builds`, true},
		{`\\SERVER\Builds\MyApp\Build-0001`, `\\SERVER\Builds`, true},
		{`//10.0.0.5/share/sub`, `\\10.0.0.5\share`, true},
		{`  \\SERVER\Builds\  `, `\\SERVER\Builds`, true},
		{`\\SERVER`, "", false},
		{`\\SERVER\`, "", false},
		{`D:\Builds`, "", false},
		{``, "", false},
	}

	for _, c := range cases {
		root, unc := netshare.ShareRoot(c.path)
		if root != c.root || unc != c.unc {
			t.Errorf("ShareRoot(%q) = (%q, %v), want (%q, %v)", c.path, root, unc, c.root, c.unc)
		}
	}
}
