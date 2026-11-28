package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/auth"
	"ruff.io/tio/config"
)

func TestTopicAcl(t *testing.T) {
	cases := []struct {
		name   string
		mode   string
		supers []config.UserPassword
		user   string
		topic  string
		write  bool
		bound  []string
		want   bool
	}{
		// superuser and inline (empty username) are allow-all
		{"superuser publish all", "dual", []config.UserPassword{{Name: "a"}}, "a", "$SYS/x", true, nil, true},
		{"superuser subscribe all", "dual", []config.UserPassword{{Name: "a"}}, "a", "tio/other/y", false, nil, true},
		{"inline publish all", "dual", nil, "", "tio/+/x", true, nil, true},
		{"inline subscribe all", "dual", nil, "", "#", false, nil, true},

		// dual mode: simple topics under tio/{id}/#
		{"dual publish up", "dual", nil, "dev1", "tio/dev1/up/x", true, nil, true},
		{"dual publish down denied", "dual", nil, "dev1", "tio/dev1/down/x", true, nil, false},
		{"dual publish data", "dual", nil, "dev1", "tio/dev1/data", true, nil, true},
		{"dual publish custom", "dual", nil, "dev1", "tio/dev1/custom/a", true, nil, true},
		{"dual subscribe down", "dual", nil, "dev1", "tio/dev1/down/x", false, nil, true},
		{"dual subscribe up denied", "dual", nil, "dev1", "tio/dev1/up/x", false, nil, false},
		{"dual subscribe data", "dual", nil, "dev1", "tio/dev1/data", false, nil, true},
		{"dual subscribe custom", "dual", nil, "dev1", "tio/dev1/custom/a", false, nil, true},

		// dual mode: legacy topics under $iothub/{things,user/things}/{id}/#
		{"dual publish shadow", "dual", nil, "dev1", "$iothub/things/dev1/shadows/get", true, nil, true},
		{"dual subscribe shadow", "dual", nil, "dev1", "$iothub/things/dev1/shadows/get", false, nil, true},
		{"dual publish user thing", "dual", nil, "dev1", "$iothub/user/things/dev1/x", true, nil, true},
		{"dual subscribe user thing", "dual", nil, "dev1", "$iothub/user/things/dev1/x", false, nil, true},
		{"dual subscribe presence event", "dual", nil, "dev1", "$iothub/events/things/dev1/presence", false, nil, true},
		{"dual publish presence event denied", "dual", nil, "dev1", "$iothub/events/things/dev1/presence", true, nil, false},

		// denied: root wildcard, $SYS, other things, wildcards at the thingId level
		{"deny root hash", "dual", nil, "dev1", "#", false, nil, false},
		{"deny sys", "dual", nil, "dev1", "$SYS/x", true, nil, false},
		{"deny other thing", "dual", nil, "dev1", "$iothub/things/dev2/x", true, nil, false},
		{"deny thing wildcard", "dual", nil, "dev1", "$iothub/things/+/x", false, nil, false},
		{"deny simple wildcard", "dual", nil, "dev1", "tio/+/x", false, nil, false},
		{"deny other simple thing", "dual", nil, "dev1", "tio/dev2/up/x", true, nil, false},

		// mode gating: legacy rejects simple topics and vice versa
		{"legacy denies simple", "legacy", nil, "dev1", "tio/dev1/up/x", true, nil, false},
		{"legacy allows legacy", "legacy", nil, "dev1", "$iothub/things/dev1/x", true, nil, true},
		{"simple denies legacy", "simple", nil, "dev1", "$iothub/things/dev1/x", false, nil, false},
		{"simple allows simple", "simple", nil, "dev1", "tio/dev1/up/x", true, nil, true},

		// gateway binding extends the ACL to bound things
		{"gateway bound thing", "dual", nil, "gw", "$iothub/things/dev2/x", true, []string{"dev2"}, true},
		{"gateway unbound thing", "dual", nil, "gw", "$iothub/things/dev3/x", true, []string{"dev2"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bg := &stubBindingGetter{boundIds: c.bound}
			aclFn := auth.TopicAcl(bg, c.supers, c.mode)
			require.Equal(t, c.want, aclFn("cid", c.user, c.topic, c.write))
		})
	}
}

func TestTopicAclDual(t *testing.T) {
	bg := &stubBindingGetter{}
	aclFn := auth.TopicAcl(bg, nil, "dual")

	require.True(t, aclFn("c", "c", "tio/c/up/shadow_get", true), "dual mode must allow simple topics")
	require.False(t, aclFn("c", "c", "tio/c/down/shadow_get_reply", true), "device still cannot publish down")
	require.True(t, aclFn("c", "c", "$iothub/things/c/shadow/get", true), "dual mode must keep legacy topics")
	require.False(t, aclFn("c", "c", "#", false), "root wildcard is never allowed")
	require.False(t, aclFn("c", "c", "$iothub/things/other/x", true), "cannot access another thing")
}

type stubBindingGetter struct {
	boundIds []string
}

func (s *stubBindingGetter) IsBoundGateway(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (s *stubBindingGetter) GetBoundThingIds(_ context.Context, _ string) ([]string, error) {
	return s.boundIds, nil
}
