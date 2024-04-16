package registries

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	apicfgv1 "github.com/openshift/api/config/v1"
	apioperatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeIsNestedInsideScope(t *testing.T) {
	for _, tt := range []struct {
		subScope, superScope string
		expected             bool
	}{
		{"quay.io", "example.com", false},                   // Host mismatch
		{"quay.io", "quay.io", true},                        // Host match
		{"quay.io:443", "quay.io", false},                   // Port mismatch (although reg is a prefix of scope)
		{"quay.io:443", "quay.io:444", false},               // Port mismatch
		{"quay.io.example.com", "quay.io", false},           // Host mismatch (although reg is a prefix of scope)
		{"quay.io2", "quay.io", false},                      // Host mismatch (although reg is a prefix of scope)
		{"quay.io/ns1", "quay.io", true},                    // Valid namespace
		{"quay.io/ns1/ns2/ns3", "quay.io", true},            // Valid namespace
		{"quay.io/ns1/ns2/ns3", "not-quay.io", false},       // Host mismatch
		{"bar/example.foo", "*.foo", false},                 // Wildcards only match host names
		{"example/bar.foo/quay.io", "*.foo", false},         // Wildcard does not match hostname
		{"example/bar.foo:400", "*.foo", false},             // Wildcard does not match hostname
		{"foo.example.com", "*.example.com", true},          // subScope matches superScope
		{"*.foo.example.com", "*.example.com", true},        // subScope matches superScope
		{"foo.example.com/bar", "*.example.com", true},      // subScope matches superScope
		{"foo.registry.com", "*.example.com", false},        // subScope does not match superScope
		{"foo.example.com", "**.example.com", false},        // subScope does not match superScope
		{"foo.example.com", "example.*.com", false},         // subScope does not match superScope
		{"foo.example.com", "*.example.com/foo/bar", false}, // subScope does not match superScope
		{"foo.example.com:443/bar/baz", "*.example.com", true},
		{"foo.example.com:443/bar/baz", "*.example.com/bar/baz", false},
		{"foo.example.com", "*example.com", false},
		{"foo.example.com", "*/example.com", false},
	} {
		t.Run(fmt.Sprintf("%#v, %#v", tt.subScope, tt.superScope), func(t *testing.T) {
			res := ScopeIsNestedInsideScope(tt.subScope, tt.superScope)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestIsValidRegistriesConfScope(t *testing.T) {
	for _, tt := range []struct {
		scope    string
		expected bool
	}{
		{"example.com", true},                // Valid registry
		{"*.example.com", true},              // Valid wildcard
		{"**.example.com", false},            // Invalid wildcard entry
		{"example.*.com", false},             // Invalid wildcard entry
		{"*.example.com/foo/bar", false},     // Invalid wildcard entry
		{"*.example.com:foo", false},         // Invalid wildcard entry
		{"*.example.com/foo:sha@bar", false}, // Invalid wildcard entry
		{"*.example.com.*.bar.com", false},   // Invalid wildcard entry
		{"*example.com", false},
		{"*/example.com", false},
		{"*.*example.com", false},
		{"", false}, // Invalid empty string entry
	} {
		t.Run(fmt.Sprintf("%#v", tt.scope), func(t *testing.T) {
			res := IsValidRegistriesConfScope(tt.scope)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestMirrorsContainsARealMirror(t *testing.T) {
	const source = "source.example.com"

	for _, tt := range []struct {
		mirrors  []apicfgv1.ImageMirror
		expected bool
	}{
		{[]apicfgv1.ImageMirror{}, false},                                  // No mirrors listed
		{[]apicfgv1.ImageMirror{"mirror.local"}, true},                     // A single real mirror
		{[]apicfgv1.ImageMirror{source}, false},                            // The source only
		{[]apicfgv1.ImageMirror{source, source, source}, false},            // Source only, repeated
		{[]apicfgv1.ImageMirror{"mirror.local", source}, true},             // Both
		{[]apicfgv1.ImageMirror{source, "mirror.local"}, true},             // Both
		{[]apicfgv1.ImageMirror{"m1.local", "m2.local", "m3.local"}, true}, // Multiple real mirrors
	} {
		t.Run(fmt.Sprintf("%#v", tt.mirrors), func(t *testing.T) {
			res := mirrorsContainsARealMirror(source, tt.mirrors)
			assert.Equal(t, tt.expected, res)
		})
	}
}

var mergedMirrorsetsTestcases = []struct {
	name   string
	input  [][]mergedMirrorSet
	result []mergedMirrorSet
}{
	{
		name:   "Empty",
		input:  [][]mergedMirrorSet{},
		result: []mergedMirrorSet{},
	},
	{
		name: "Irrelevant singletons",
		input: [][]mergedMirrorSet{
			{
				{source: "a.example.com", mirrors: nil},
				{source: "b.example.com", mirrors: []string{}},
			},
		},
		result: []mergedMirrorSet{},
	},
	// The registry names below start with an irrelevant letter, usually counting from the end of the alphabet, to verify that
	// the result is based on the order in the Sources array and is not just alphabetically-sorted.
	{
		name: "Separate mirror sets",
		input: [][]mergedMirrorSet{
			{
				{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net", "x3.example.net"}},
			},
			{
				{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com", "x3.example.com"}},
			},
		},
		result: []mergedMirrorSet{
			{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com", "x3.example.com"}},
			{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net", "x3.example.net"}},
		},
	},
	{
		name: "Separate mirror sets with mirrorSourcePolicy set",
		input: [][]mergedMirrorSet{
			{
				{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net", "x3.example.net"}, mirrorSourcePolicy: apicfgv1.NeverContactSource},
			},
			{
				{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com", "x3.example.com"}, mirrorSourcePolicy: apicfgv1.AllowContactingSource},
			},
		},
		result: []mergedMirrorSet{
			{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com", "x3.example.com"}},
			{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net", "x3.example.net"}, mirrorSourcePolicy: apicfgv1.NeverContactSource},
		},
	},
	{
		name: "Sets with a shared element - strict order",
		input: [][]mergedMirrorSet{
			{
				{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net"}},
				{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com"}},
			},
			{
				{source: "source.example.net", mirrors: []string{"y2.example.net", "x3.example.net"}},
				{source: "source.example.com", mirrors: []string{"y2.example.com", "x3.example.com"}},
			},
		},
		result: []mergedMirrorSet{
			{source: "source.example.com", mirrors: []string{"z1.example.com", "y2.example.com", "x3.example.com"}},
			{source: "source.example.net", mirrors: []string{"z1.example.net", "y2.example.net", "x3.example.net"}},
		},
	},
	{
		// This is not technically impossible, and it could be in principle used to set up last-fallback mirrors that
		// are only accessed if the source is not available.
		// WARNING: The order in this case is unspecified by the ICSP specification, and may change at any time;
		// this test case only ensures that the corner case is handled reasonably, and that the output is stable
		// (i.e. the operator does not cause unnecessary changes in output objects.)
		name: "Source included in mirrors",
		input: [][]mergedMirrorSet{
			{
				{source: "source.example.com", mirrors: []string{"z1.example.com", "source.example.com", "y2.example.com"}},
				{source: "source.example.com", mirrors: []string{"source.example.com", "y2.example.com", "x3.example.com"}},
			},
		},
		result: []mergedMirrorSet{
			{source: "source.example.com", mirrors: []string{"z1.example.com", "source.example.com", "y2.example.com", "x3.example.com"}},
		},
	},
	{
		// Worst case of the above: _only_ the source included in mirrors, even perhaps several times.
		name: "Mirrors includes only source",
		input: [][]mergedMirrorSet{
			{
				{source: "source.example.com", mirrors: []string{"source.example.com"}},
				{source: "source.example.net", mirrors: []string{"source.example.net", "source.example.net", "source.example.net"}},
			},
		},
		result: []mergedMirrorSet{},
	},
	// More complex mirror set combinations are mostly tested in TestTopoGraph
	{
		name: "Example",
		input: [][]mergedMirrorSet{
			{ // Vendor-provided default configuration
				{source: "source.vendor.com", mirrors: []string{"registry2.vendor.com"}},
			},
			{ // Vendor2-provided default configuration
				{source: "source.vendor2.com", mirrors: []string{"registry1.vendor2.com", "registry2.vendor2.com"}},
			},
			{ // Admin-configured local mirrors:
				{source: "source.vendor.com", mirrors: []string{"local-mirror.example.com"}},
				// Opposite order of the vendor’s mirrors.
				// WARNING: The order in this case is unspecified by the ICSP specification, and may change at any time;
				// this test case only ensures that the corner case is handled reasonably, and that the output is stable
				// (i.e. the operator does not cause unnecessary changes in output objects.)
				{source: "source.vendor2.com", mirrors: []string{"local-mirror2.example.com", "registry2.vendor2.com", "registry1.vendor2.com"}},
			},
		},
		result: []mergedMirrorSet{
			{source: "source.vendor.com", mirrors: []string{"local-mirror.example.com", "registry2.vendor.com"}},
			{source: "source.vendor2.com", mirrors: []string{"local-mirror2.example.com", "registry1.vendor2.com", "registry2.vendor2.com"}},
		},
	},
}

func TestMergedICSPMirrorSets(t *testing.T) {
	for _, tc := range mergedMirrorsetsTestcases {
		t.Run(tc.name, func(t *testing.T) {
			in := []*apioperatorsv1alpha1.ImageContentSourcePolicy{}
			for _, items := range tc.input {
				rdms := []apioperatorsv1alpha1.RepositoryDigestMirrors{}
				for _, item := range items {
					if item.mirrorSourcePolicy != "" {
						t.Skip("skip icsp test with mirrorSourcePolicy")
					}
					rdms = append(rdms, apioperatorsv1alpha1.RepositoryDigestMirrors{
						Source:  item.source,
						Mirrors: item.mirrors,
					})
				}
				in = append(in, &apioperatorsv1alpha1.ImageContentSourcePolicy{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: rdms,
					},
				})
			}
			res, err := mergedDigestMirrorSets(nil, in)
			require.Nil(t, err)
			assert.Equal(t, tc.result, res)
		})
	}
}

func TestMergedTagMirrorSets(t *testing.T) {
	for _, tc := range mergedMirrorsetsTestcases {
		t.Run(tc.name, func(t *testing.T) {
			in := []*apicfgv1.ImageTagMirrorSet{}
			for _, items := range tc.input {
				itm := []apicfgv1.ImageTagMirrors{}
				for _, item := range items {
					imgMirrors := []apicfgv1.ImageMirror{}
					for _, m := range item.mirrors {
						imgMirrors = append(imgMirrors, apicfgv1.ImageMirror(m))
					}
					itm = append(itm, apicfgv1.ImageTagMirrors{Source: item.source, Mirrors: imgMirrors, MirrorSourcePolicy: item.mirrorSourcePolicy})
				}
				in = append(in, &apicfgv1.ImageTagMirrorSet{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: itm,
					},
				})
			}
			res, err := mergedTagMirrorSets(in)
			require.Nil(t, err)
			assert.Equal(t, tc.result, res)
		})
	}
}

func TestMergedDigestMirrorSets(t *testing.T) {
	for _, tc := range mergedMirrorsetsTestcases {
		t.Run(tc.name, func(t *testing.T) {
			in := []*apicfgv1.ImageDigestMirrorSet{}
			for _, items := range tc.input {
				idm := []apicfgv1.ImageDigestMirrors{}
				for _, item := range items {
					imgMirrors := []apicfgv1.ImageMirror{}
					for _, m := range item.mirrors {
						imgMirrors = append(imgMirrors, apicfgv1.ImageMirror(m))
					}
					idm = append(idm, apicfgv1.ImageDigestMirrors{Source: item.source, Mirrors: imgMirrors, MirrorSourcePolicy: item.mirrorSourcePolicy})
				}
				in = append(in, &apicfgv1.ImageDigestMirrorSet{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: idm,
					},
				})
			}
			res, err := mergedDigestMirrorSets(in, nil)
			require.Nil(t, err)
			assert.Equal(t, tc.result, res)
		})
	}
}

func TestMirrorsAdjustedForNestedScope(t *testing.T) {
	// Invalid input
	for _, tt := range []struct {
		mirroredScope, subScope string
	}{
		{"mirrored.com", "unrelated.com"},
		{"*.example.com", "*.nested.example.com"},
	} {
		_, err := mirrorsAdjustedForNestedScope(tt.mirroredScope, tt.subScope, []sysregistriesv2.Endpoint{})
		assert.Error(t, err, fmt.Sprintf("%#v", tt))
	}

	// A smoke test for valid input
	res, err := mirrorsAdjustedForNestedScope("example.com", "example.com/subscope",
		[]sysregistriesv2.Endpoint{
			{Location: "mirror-1.com"},
			{Location: "mirror-2.com/nested"},
			{Location: "example.com"}, // We usually add the source the last mirror entry.
		})
	require.NoError(t, err)
	assert.Equal(t, []sysregistriesv2.Endpoint{
		{Location: "mirror-1.com/subscope"},
		{Location: "mirror-2.com/nested/subscope"},
		{Location: "example.com/subscope"},
	}, res)
}

func TestEditRegistriesConfig(t *testing.T) {
	templateConfig := sysregistriesv2.V2RegistriesConf{ // This matches templates/*/01-*-container-runtime/_base/files/container-registries.yaml
		UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
	}
	buf := bytes.Buffer{}
	err := toml.NewEncoder(&buf).Encode(templateConfig)
	require.NoError(t, err)
	templateBytes := buf.Bytes()

	tests := []struct {
		name              string
		insecure, blocked []string
		idmsRules         []*apicfgv1.ImageDigestMirrorSet
		itmsRules         []*apicfgv1.ImageTagMirrorSet
		icspRules         []*apioperatorsv1alpha1.ImageContentSourcePolicy
		want              sysregistriesv2.V2RegistriesConf
	}{
		{
			name: "unchanged",
			want: templateConfig,
		},
		{
			name:     "insecure+blocked",
			insecure: []string{"registry.access.redhat.com", "insecure.com", "common.com"},
			blocked:  []string{"blocked.com", "common.com", "docker.io"},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com",
						},
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "common.com",
							Insecure: true,
						},
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "docker.io",
						},
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry.access.redhat.com",
							Insecure: true,
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com",
							Insecure: true,
						},
					},
				},
			},
		},
		{
			name:      "imageContentSourcePolicy",
			insecure:  []string{"insecure.com", "*.insecure-example.com", "*.insecure.blocked-example.com"},
			blocked:   []string{"blocked.com", "*.blocked.insecure-example.com", "*.blocked-example.com"},
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{},
			icspRules: []*apioperatorsv1alpha1.ImageContentSourcePolicy{
				{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "insecure.com/ns-i1", Mirrors: []string{"blocked.com/ns-b1", "other.com/ns-o1"}},
							{Source: "blocked.com/ns-b/ns2-b", Mirrors: []string{"other.com/ns-o2", "insecure.com/ns-i2"}},
							{Source: "other.com/ns-o3", Mirrors: []string{"insecure.com/ns-i2", "blocked.com/ns-b/ns3-b", "foo.insecure-example.com/bar"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com/ns-b/ns2-b",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o2", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-i1",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "blocked.com/ns-b1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "other.com/ns-o3",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "blocked.com/ns-b/ns3-b", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "foo.insecure-example.com/bar", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com",
						},
						Blocked: true,
					},
					{
						Prefix:  "*.blocked.insecure-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.blocked-example.com",
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com",
							Insecure: true,
						},
					},
					{
						Prefix: "*.insecure-example.com",
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.insecure.blocked-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
				},
			},
		},
		{
			name:     "insecure+blocked prefixes with wildcard entries",
			insecure: []string{"insecure.com", "*.insecure-example.com", "*.insecure.blocked-example.com"},
			blocked:  []string{"blocked.com", "*.blocked.insecure-example.com", "*.blocked-example.com"},
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{ // other.com is neither insecure nor blocked
							{Source: "insecure.com/ns-i1", Mirrors: []apicfgv1.ImageMirror{"blocked.com/ns-b1", "other.com/ns-o1"}},
							{Source: "blocked.com/ns-b/ns2-b", Mirrors: []apicfgv1.ImageMirror{"other.com/ns-o2", "insecure.com/ns-i2"}},
							{Source: "other.com/ns-o3", Mirrors: []apicfgv1.ImageMirror{"insecure.com/ns-i2", "blocked.com/ns-b/ns3-b", "foo.insecure-example.com/bar"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com/ns-b/ns2-b",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o2", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-i1",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "blocked.com/ns-b1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "other.com/ns-o3",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "blocked.com/ns-b/ns3-b", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "foo.insecure-example.com/bar", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com",
						},
						Blocked: true,
					},
					{
						Prefix:  "*.blocked.insecure-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.blocked-example.com",
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com",
							Insecure: true,
						},
					},
					{
						Prefix: "*.insecure-example.com",
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.insecure.blocked-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
				},
			},
		},
		{
			name: "imageTagMirrorSet",
			itmsRules: []*apicfgv1.ImageTagMirrorSet{
				{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: []apicfgv1.ImageTagMirrors{
							{Source: "registry-a.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-a.com"}},
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-b.com"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-a.com",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-tag-1.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-b.com",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-tag-1.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},
				},
			},
		},
		{
			name: "imageDigestMirrorSet + imageTagMirrorSet",
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{
							{Source: "registry-a.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-a.com", "mirror-digest-2.registry-a.com"}},
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-b.com", "mirror-digest-2.registry-b.com"}},
						},
					},
				},
			},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{
				{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: []apicfgv1.ImageTagMirrors{
							{Source: "registry-a.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-a.com", "mirror-tag-2.registry-a.com"}},
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-b.com", "mirror-tag-2.registry-b.com"}},
						},
					},
				},
			},

			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-a.com",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-digest-1.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-digest-2.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag-1.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
							{Location: "mirror-tag-2.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-b.com",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-digest-1.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-digest-2.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag-1.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
							{Location: "mirror-tag-2.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},
				},
			},
		},
		{
			name: "mirrorSourcePolicy",
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{
							{Source: "registry-a.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-a.com", "mirror-digest-2.registry-a.com"}, MirrorSourcePolicy: "NeverContactSource"},
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-b.com", "mirror-digest-2.registry-b.com"}},
							{Source: "registry-c.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-c.com", "mirror-digest-2.registry-c.com"}},
						},
					},
				},
			},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{
				{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: []apicfgv1.ImageTagMirrors{
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-b.com", "mirror-tag-2.registry-b.com"}, MirrorSourcePolicy: "NeverContactSource"},
						},
					},
				},
			},

			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-a.com",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-digest-1.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-digest-2.registry-a.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-b.com",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-digest-1.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-digest-2.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag-1.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
							{Location: "mirror-tag-2.registry-b.com", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "registry-c.com",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror-digest-1.registry-c.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-digest-2.registry-c.com", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
				},
			},
		},

		{
			name:     "insecure+blocked scopes inside a configured mirror",
			insecure: []string{"primary.com/top/insecure"},
			blocked:  []string{"primary.com/top/blocked"},
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{
							{Source: "primary.com/top", Mirrors: []apicfgv1.ImageMirror{"mirror.com/primary"}},
							{Source: "primary.com/top/insecure/more-specific", Mirrors: []apicfgv1.ImageMirror{"mirror.com/more-specific"}},
						},
					},
				},
			},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{
				{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: []apicfgv1.ImageTagMirrors{
							{Source: "primary.com/top", Mirrors: []apicfgv1.ImageMirror{"mirror-tag.com/primary"}},
							{Source: "primary.com/top/insecure/more-specific", Mirrors: []apicfgv1.ImageMirror{"mirror-tag.com/more-specific"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag.com/primary", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/insecure/more-specific",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/more-specific", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag.com/more-specific", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/blocked",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary/blocked", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag.com/primary/blocked", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/insecure",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary/insecure", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "mirror-tag.com/primary/insecure", PullFromMirror: sysregistriesv2.MirrorByTagOnly},
						},
					},
				},
			},
		},
		{
			name:     "insecure+blocked scopes inside a configured mirror in ImageContentSourcePolicy",
			insecure: []string{"primary.com/top/insecure"},
			blocked:  []string{"primary.com/top/blocked"},
			icspRules: []*apioperatorsv1alpha1.ImageContentSourcePolicy{
				{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "primary.com/top", Mirrors: []string{"mirror.com/primary"}},
							{Source: "primary.com/top/insecure/more-specific", Mirrors: []string{"mirror.com/more-specific"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/insecure/more-specific",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/more-specific", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/blocked",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary/blocked", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "primary.com/top/insecure",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "mirror.com/primary/insecure", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
				},
			},
		},
		{
			name:      "imageContentSourcePolicy+imageDigestMirrorSet",
			insecure:  []string{"insecure.com", "*.insecure-example.com", "*.insecure.blocked-example.com"},
			blocked:   []string{"blocked.com", "*.blocked.insecure-example.com", "*.blocked-example.com"},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{},
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{ // other.com is neither insecure nor blocked
							{Source: "insecure.com/ns-idms-i1", Mirrors: []apicfgv1.ImageMirror{"blocked.com/ns-b1", "other.com/ns-o1"}},
							{Source: "blocked.com/ns-idms-b/ns2-b", Mirrors: []apicfgv1.ImageMirror{"other.com/ns-o2", "insecure.com/ns-i2"}},
							{Source: "other.com/ns-idms-o3", Mirrors: []apicfgv1.ImageMirror{"insecure.com/ns-i2", "blocked.com/ns-b/ns3-b", "foo.insecure-example.com/bar"}},
						},
					},
				},
			},
			icspRules: []*apioperatorsv1alpha1.ImageContentSourcePolicy{
				{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "insecure.com/ns-icsp-i1", Mirrors: []string{"blocked.com/ns-b1", "other.com/ns-o1"}},
							{Source: "blocked.com/ns-icsp-b/ns2-b", Mirrors: []string{"other.com/ns-o2", "insecure.com/ns-i2"}},
							{Source: "other.com/ns-icsp-o3", Mirrors: []string{"insecure.com/ns-i2", "blocked.com/ns-b/ns3-b", "foo.insecure-example.com/bar"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com/ns-icsp-b/ns2-b",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o2", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com/ns-idms-b/ns2-b",
						},
						Blocked: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o2", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-icsp-i1",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "blocked.com/ns-b1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-idms-i1",
							Insecure: true,
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "blocked.com/ns-b1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "other.com/ns-icsp-o3",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "blocked.com/ns-b/ns3-b", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "foo.insecure-example.com/bar", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "other.com/ns-idms-o3",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "insecure.com/ns-i2", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "blocked.com/ns-b/ns3-b", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "foo.insecure-example.com/bar", Insecure: true, PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "blocked.com",
						},
						Blocked: true,
					},
					{
						Prefix:  "*.blocked.insecure-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.blocked-example.com",
						Blocked: true,
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com",
							Insecure: true,
						},
					},
					{
						Prefix: "*.insecure-example.com",
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
					{
						Prefix:  "*.insecure.blocked-example.com",
						Blocked: true,
						Endpoint: sysregistriesv2.Endpoint{
							Insecure: true,
						},
					},
				},
			},
		},
		{
			name:      "imageContentSourcePolicy+imageDigestMirrorSet Confict",
			insecure:  []string{},
			blocked:   []string{},
			itmsRules: []*apicfgv1.ImageTagMirrorSet{},
			idmsRules: []*apicfgv1.ImageDigestMirrorSet{
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{ // other.com is neither insecure nor blocked
							{Source: "insecure.com/ns-dupe-i1", Mirrors: []apicfgv1.ImageMirror{"other.com/ns-o1"}},
							{Source: "insecure.com/ns-dupe-i1", Mirrors: []apicfgv1.ImageMirror{"other.com/ns-o3"}},
						},
					},
				},
				{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{ // other.com is neither insecure nor blocked
							{Source: "insecure.com/ns-idms-i1", Mirrors: []apicfgv1.ImageMirror{"other.com/ns-o1"}},
						},
					},
				},
			},
			icspRules: []*apioperatorsv1alpha1.ImageContentSourcePolicy{
				{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "insecure.com/ns-dupe-i1", Mirrors: []string{"other.com/ns-o2"}},
						},
					},
				},
				{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "insecure.com/ns-icsp-i1", Mirrors: []string{"other.com/ns-o1"}},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-dupe-i1",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o2", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
							{Location: "other.com/ns-o3", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-icsp-i1",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-idms-i1",
						},
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o1", PullFromMirror: sysregistriesv2.MirrorByDigestOnly},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config from templateBytes to get a fresh copy we can edit.
			config := sysregistriesv2.V2RegistriesConf{}
			_, err := toml.Decode(string(templateBytes), &config)
			require.NoError(t, err)
			err = EditRegistriesConfig(&config, tt.insecure, tt.blocked, tt.icspRules, tt.idmsRules, tt.itmsRules)
			if err != nil {
				t.Errorf("updateRegistriesConfig() error = %v", err)
				return
			}
			// This assumes a specific order of Registries entries, which does not actually matter; ideally, this would
			// sort the two arrays before comparing, but right now hard-coding the order works well enough.
			require.Equal(t, tt.want, config, tt.name)
			// Ensure that the generated configuration is actually valid.
			buf := bytes.Buffer{}
			err = toml.NewEncoder(&buf).Encode(config)
			require.NoError(t, err)
			registriesConf, err := os.CreateTemp("", "registries.conf")
			require.NoError(t, err)
			_, err = registriesConf.Write(buf.Bytes())
			require.NoError(t, err)
			defer os.Remove(registriesConf.Name())
			_, err = sysregistriesv2.GetRegistries(&types.SystemContext{
				SystemRegistriesConfPath: registriesConf.Name(),
			})
			assert.NoError(t, err)
		})
	}

	successTests := []struct {
		name         string
		idmsNotEmpty bool
		itmsNotEmpty bool
		icspNotEmpty bool
		want         []byte
	}{
		{
			name:         "imageContentSourcePolicy + imageDigestMirrorSet",
			idmsNotEmpty: true,
			icspNotEmpty: true,
			want: []byte(`unqualified-search-registries = ["registry.access.redhat.com", "docker.io"]
short-name-mode = ""

[[registry]]
  prefix = ""
  location = "registry-a.com"

  [[registry.mirror]]
    location = "mirror-icsp-1.registry-a.com"
    pull-from-mirror = "digest-only"

[[registry]]
  prefix = ""
  location = "registry-b.com"

  [[registry.mirror]]
    location = "mirror-digest-1.registry-b.com"
    pull-from-mirror = "digest-only"
`),
		},
		{
			name:         "imageContentSourcePolicy + imageTagMirrorSet",
			itmsNotEmpty: true,
			icspNotEmpty: true,
			want: []byte(`unqualified-search-registries = ["registry.access.redhat.com", "docker.io"]
short-name-mode = ""

[[registry]]
  prefix = ""
  location = "registry-a.com"

  [[registry.mirror]]
    location = "mirror-icsp-1.registry-a.com"
    pull-from-mirror = "digest-only"

[[registry]]
  prefix = ""
  location = "registry-c.com"

  [[registry.mirror]]
    location = "mirror-tag-1.registry-c.com"
    pull-from-mirror = "tag-only"
`),
		},
		{
			name:         "ImageContentSourcePolicy + imageDigestMirrorSet + ImageTagMirrorSet",
			idmsNotEmpty: true,
			itmsNotEmpty: true,
			icspNotEmpty: true,
			want: []byte(`unqualified-search-registries = ["registry.access.redhat.com", "docker.io"]
short-name-mode = ""

[[registry]]
  prefix = ""
  location = "registry-a.com"

  [[registry.mirror]]
    location = "mirror-icsp-1.registry-a.com"
    pull-from-mirror = "digest-only"

[[registry]]
  prefix = ""
  location = "registry-b.com"

  [[registry.mirror]]
    location = "mirror-digest-1.registry-b.com"
    pull-from-mirror = "digest-only"

[[registry]]
  prefix = ""
  location = "registry-c.com"

  [[registry.mirror]]
    location = "mirror-tag-1.registry-c.com"
    pull-from-mirror = "tag-only"
`),
		},
	}
	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config from templateBytes to get a fresh copy we can edit.
			config := sysregistriesv2.V2RegistriesConf{}
			_, err := toml.Decode(string(templateBytes), &config)
			require.NoError(t, err)
			icsps := []*apioperatorsv1alpha1.ImageContentSourcePolicy{}
			idmss := []*apicfgv1.ImageDigestMirrorSet{}
			itmss := []*apicfgv1.ImageTagMirrorSet{}
			if tt.idmsNotEmpty {
				idmss = append(idmss, &apicfgv1.ImageDigestMirrorSet{
					Spec: apicfgv1.ImageDigestMirrorSetSpec{
						ImageDigestMirrors: []apicfgv1.ImageDigestMirrors{
							{Source: "registry-b.com", Mirrors: []apicfgv1.ImageMirror{"mirror-digest-1.registry-b.com"}},
						},
					},
				})
			}
			if tt.itmsNotEmpty {
				itmss = append(itmss, &apicfgv1.ImageTagMirrorSet{
					Spec: apicfgv1.ImageTagMirrorSetSpec{
						ImageTagMirrors: []apicfgv1.ImageTagMirrors{
							{Source: "registry-c.com", Mirrors: []apicfgv1.ImageMirror{"mirror-tag-1.registry-c.com"}},
						},
					},
				})
			}
			if tt.icspNotEmpty {
				icsps = append(icsps, &apioperatorsv1alpha1.ImageContentSourcePolicy{
					Spec: apioperatorsv1alpha1.ImageContentSourcePolicySpec{
						RepositoryDigestMirrors: []apioperatorsv1alpha1.RepositoryDigestMirrors{
							{Source: "registry-a.com", Mirrors: []string{"mirror-icsp-1.registry-a.com"}},
						},
					},
				})
			}
			err = EditRegistriesConfig(&config, nil, nil, icsps, idmss, itmss)
			assert.Nil(t, err)
			buf := bytes.Buffer{}
			err = toml.NewEncoder(&buf).Encode(config)
			require.NoError(t, err)
			assert.Equal(t, buf.Bytes(), tt.want)
		})
	}
}
