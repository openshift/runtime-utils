package registries

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	apicfgv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/diff"
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
			res := scopeIsNestedInsideScope(tt.subScope, tt.superScope)
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
	} {
		t.Run(fmt.Sprintf("%#v", tt.scope), func(t *testing.T) {
			res := IsValidRegistriesConfScope(tt.scope)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestRDMContainsARealMirror(t *testing.T) {
	const source = "source.example.com"

	for _, tt := range []struct {
		mirrors  []apicfgv1.Mirror
		expected bool
	}{
		{[]apicfgv1.Mirror{}, false},                                  // No mirrors listed
		{[]apicfgv1.Mirror{"mirror.local"}, true},                     // A single real mirror
		{[]apicfgv1.Mirror{source}, false},                            // The source only
		{[]apicfgv1.Mirror{source, source, source}, false},            // Source only, repeated
		{[]apicfgv1.Mirror{"mirror.local", source}, true},             // Both
		{[]apicfgv1.Mirror{source, "mirror.local"}, true},             // Both
		{[]apicfgv1.Mirror{"m1.local", "m2.local", "m3.local"}, true}, // Multiple real mirrors
	} {
		t.Run(fmt.Sprintf("%#v", tt.mirrors), func(t *testing.T) {
			set := apicfgv1.RepositoryDigestMirrors{
				Source:  source,
				Mirrors: tt.mirrors,
			}
			res := rdmContainsARealMirror(&set)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestMergedMirrorSets(t *testing.T) {
	for _, c := range []struct {
		name   string
		input  [][]apicfgv1.RepositoryDigestMirrors
		result []apicfgv1.RepositoryDigestMirrors
	}{
		{
			name:   "Empty",
			input:  [][]apicfgv1.RepositoryDigestMirrors{},
			result: []apicfgv1.RepositoryDigestMirrors{},
		},
		{
			name: "Irrelevant singletons",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{
					{Source: "a.example.com", Mirrors: nil},
					{Source: "b.example.com", Mirrors: []apicfgv1.Mirror{}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{},
		},
		// The registry names below start with an irrelevant letter, usually counting from the end of the alphabet, to verify that
		// the result is based on the order in the Sources array and is not just alphabetically-sorted.
		{
			name: "Separate mirror sets",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{
					{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net", "x3.example.net"}},
				},
				{
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "y2.example.com", "x3.example.com"}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{
				{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "y2.example.com", "x3.example.com"}},
				{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net", "x3.example.net"}},
			},
		},
		{
			name: "Sets with a shared element - strict order",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{
					{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net"}},
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "y2.example.com"}},
				},
				{
					{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"y2.example.net", "x3.example.net"}},
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"y2.example.com", "x3.example.com"}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{
				{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "y2.example.com", "x3.example.com"}},
				{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net", "x3.example.net"}},
			},
		},
		{
			// This is not technically impossible, and it could be in principle used to set up last-fallback mirrors that
			// are only accessed if the source is not available.
			// WARNING: The order in this case is unspecified by the ICP specification, and may change at any time;
			// this test case only ensures that the corner case is handled reasonably, and that the output is stable
			// (i.e. the operator does not cause unnecessary changes in output objects.)
			name: "Source included in mirrors",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "source.example.com", "y2.example.com"}},
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"source.example.com", "y2.example.com", "x3.example.com"}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{
				{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.com", "source.example.com", "y2.example.com", "x3.example.com"}},
			},
		},
		{
			// Worst case of the above: _only_ the source included in mirrors, even perhaps several times.
			name: "Mirrors includes only source",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{
					{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"source.example.com"}},
					{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"source.example.net", "source.example.net", "source.example.net"}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{},
		},
		// More complex mirror set combinations are mostly tested in TestTopoGraph
		{
			name: "Example",
			input: [][]apicfgv1.RepositoryDigestMirrors{
				{ // Vendor-provided default configuration
					{Source: "source.vendor.com", Mirrors: []apicfgv1.Mirror{"registry2.vendor.com"}},
				},
				{ // Vendor2-provided default configuration
					{Source: "source.vendor2.com", Mirrors: []apicfgv1.Mirror{"registry1.vendor2.com", "registry2.vendor2.com"}},
				},
				{ // Admin-configured local mirrors:
					{Source: "source.vendor.com", Mirrors: []apicfgv1.Mirror{"local-mirror.example.com"}},
					// Opposite order of the vendor’s mirrors.
					// WARNING: The order in this case is unspecified by the ICP specification, and may change at any time;
					// this test case only ensures that the corner case is handled reasonably, and that the output is stable
					// (i.e. the operator does not cause unnecessary changes in output objects.)
					{Source: "source.vendor2.com", Mirrors: []apicfgv1.Mirror{"local-mirror2.example.com", "registry2.vendor2.com", "registry1.vendor2.com"}},
				},
			},
			result: []apicfgv1.RepositoryDigestMirrors{
				{Source: "source.vendor.com", Mirrors: []apicfgv1.Mirror{"local-mirror.example.com", "registry2.vendor.com"}},
				{Source: "source.vendor2.com", Mirrors: []apicfgv1.Mirror{"local-mirror2.example.com", "registry1.vendor2.com", "registry2.vendor2.com"}},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := []*apicfgv1.ImageContentPolicy{}
			for _, rdms := range c.input {
				in = append(in, &apicfgv1.ImageContentPolicy{
					Spec: apicfgv1.ImageContentPolicySpec{
						RepositoryDigestMirrors: rdms,
					},
				})
			}
			res, err := mergedMirrorSets(in)
			if err != nil {
				t.Errorf("Error %v", err)
				return
			}
			if !reflect.DeepEqual(res, c.result) {
				t.Errorf("Result %#v, expected %#v", res, c.result)
				return
			}
		})
	}
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
		icpRules          []*apicfgv1.ImageContentPolicy
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
			name:     "insecure+blocked prefixes with wildcard entries",
			insecure: []string{"insecure.com", "*.insecure-example.com", "*.insecure.blocked-example.com"},
			blocked:  []string{"blocked.com", "*.blocked.insecure-example.com", "*.blocked-example.com"},
			icpRules: []*apicfgv1.ImageContentPolicy{
				{
					Spec: apicfgv1.ImageContentPolicySpec{
						RepositoryDigestMirrors: []apicfgv1.RepositoryDigestMirrors{ // other.com is neither insecure nor blocked
							{Source: "insecure.com/ns-i1", Mirrors: []apicfgv1.Mirror{"blocked.com/ns-b1", "other.com/ns-o1"}},
							{Source: "blocked.com/ns-b/ns2-b", Mirrors: []apicfgv1.Mirror{"other.com/ns-o2", "insecure.com/ns-i2"}},
							{Source: "other.com/ns-o3", Mirrors: []apicfgv1.Mirror{"insecure.com/ns-i2", "blocked.com/ns-b/ns3-b", "foo.insecure-example.com/bar"}},
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
						Blocked:            true,
						MirrorByDigestOnly: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "other.com/ns-o2"},
							{Location: "insecure.com/ns-i2", Insecure: true},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "insecure.com/ns-i1",
							Insecure: true,
						},
						MirrorByDigestOnly: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "blocked.com/ns-b1"},
							{Location: "other.com/ns-o1"},
						},
					},

					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "other.com/ns-o3",
						},
						MirrorByDigestOnly: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "insecure.com/ns-i2", Insecure: true},
							{Location: "blocked.com/ns-b/ns3-b"},
							{Location: "foo.insecure-example.com/bar", Insecure: true},
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
			name: "allowMirrorsByTags with mirrors",
			icpRules: []*apicfgv1.ImageContentPolicy{
				{
					Spec: apicfgv1.ImageContentPolicySpec{
						RepositoryDigestMirrors: []apicfgv1.RepositoryDigestMirrors{
							{Source: "source.example.com", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net"}},
						},
					},
				},
				{
					Spec: apicfgv1.ImageContentPolicySpec{
						RepositoryDigestMirrors: []apicfgv1.RepositoryDigestMirrors{
							{Source: "source.example.net", Mirrors: []apicfgv1.Mirror{"z1.example.net", "y2.example.net"}, AllowMirrorByTags: true},
						},
					},
				},
			},
			want: sysregistriesv2.V2RegistriesConf{
				UnqualifiedSearchRegistries: []string{"registry.access.redhat.com", "docker.io"},
				Registries: []sysregistriesv2.Registry{
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "source.example.com",
						},
						MirrorByDigestOnly: true,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "z1.example.net"},
							{Location: "y2.example.net"},
						},
					},
					{
						Endpoint: sysregistriesv2.Endpoint{
							Location: "source.example.net",
						},
						MirrorByDigestOnly: false,
						Mirrors: []sysregistriesv2.Endpoint{
							{Location: "z1.example.net"},
							{Location: "y2.example.net"},
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
			err = EditRegistriesConfig(&config, tt.insecure, tt.blocked, tt.icpRules)
			if err != nil {
				t.Errorf("updateRegistriesConfig() error = %v", err)
				return
			}
			// This assumes a specific order of Registries entries, which does not actually matter; ideally, this would
			// sort the two arrays before comparing, but right now hard-coding the order works well enough.
			if !reflect.DeepEqual(config, tt.want) {
				t.Errorf("updateRegistriesConfig() Diff:\n %s", diff.ObjectGoPrintDiff(tt.want, config))
			}
			// Ensure that the generated configuration is actually valid.
			buf := bytes.Buffer{}
			err = toml.NewEncoder(&buf).Encode(config)
			require.NoError(t, err)
			registriesConf, err := ioutil.TempFile("", "registries.conf")
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
}
