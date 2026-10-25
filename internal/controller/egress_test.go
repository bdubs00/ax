// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"testing"

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"github.com/google/ax/pkg/apis/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func secretEnv(name, secret, host, header, prefix string) *v1alpha1.EnvVar {
	return &v1alpha1.EnvVar{Name: name, SecretRef: &v1alpha1.SecretEnvSource{
		Name:   secret,
		Key:    "token",
		Inject: &v1alpha1.CredentialInjection{Host: host, Header: header, Prefix: prefix},
	}}
}

// A secret_ref puts a header replacement on the rules of its host: an HTTP
// entry keeps its protocol and port, a TLS entry becomes an HTTPS rule, and a
// host without an entry gets an HTTPS rule on port 443.
func TestEgressRules_SecretRef(t *testing.T) {
	spec := &v1alpha1.TaskSpec{
		Egress: &v1alpha1.EgressSpec{Hosts: []string{
			"http://ansible-mcp.fw-fleet.svc.cluster.local:8083",
			"github.com",
			"pypi.org",
		}},
		Env: []*v1alpha1.EnvVar{
			{Name: "PLAIN", Value: "x"},
			secretEnv("MCP_TOKEN", "ansible-mcp", "ansible-mcp.fw-fleet.svc.cluster.local", "Authorization", "Bearer "),
			secretEnv("MCP_KEY", "ansible-mcp-key", "ansible-mcp.fw-fleet.svc.cluster.local", "X-Api-Key", ""),
			secretEnv("GITHUB_TOKEN", "github", "github.com", "Authorization", "token "),
			secretEnv("STRIPE_KEY", "stripe", "api.stripe.com", "Authorization", "Bearer "),
		},
	}
	got, err := egressRules(spec, "fw-fleet")
	if err != nil {
		t.Fatalf("egressRules: %v", err)
	}
	cred := func(header, prefix, secret string) *ateapipb.CredentialHeader {
		return &ateapipb.CredentialHeader{Header: header, Prefix: prefix, CredentialUri: "ate-secret://k8s.io/default/fw-fleet/" + secret + "/token"}
	}
	want := []*ateapipb.EgressRule{
		{Http: &ateapipb.HTTPRule{
			Hostnames: []string{"ansible-mcp.fw-fleet.svc.cluster.local"},
			Ports:     &ateapipb.Ports{Numbers: []int32{8083}},
			Effects: &ateapipb.HttpRuleEffects{ReplaceHeaders: []*ateapipb.CredentialHeader{
				cred("Authorization", "Bearer ", "ansible-mcp"),
				cred("X-Api-Key", "", "ansible-mcp-key"),
			}},
		}},
		{Https: &ateapipb.HTTPSRule{
			Hostnames: []string{"github.com"},
			Ports:     &ateapipb.Ports{Numbers: []int32{443}},
			Effects:   &ateapipb.HttpRuleEffects{ReplaceHeaders: []*ateapipb.CredentialHeader{cred("Authorization", "token ", "github")}},
		}},
		{TlsPassthrough: &ateapipb.TLSPassthroughRule{
			Hostnames: []string{"pypi.org"},
			Ports:     &ateapipb.Ports{Numbers: []int32{443}},
		}},
		{Https: &ateapipb.HTTPSRule{
			Hostnames: []string{"api.stripe.com"},
			Ports:     &ateapipb.Ports{Numbers: []int32{443}},
			Effects:   &ateapipb.HttpRuleEffects{ReplaceHeaders: []*ateapipb.CredentialHeader{cred("Authorization", "Bearer ", "stripe")}},
		}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rules, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("rule %d:\n got %v\nwant %v", i, got[i], want[i])
		}
	}
}

// A task without egress hosts and without a secret_ref gets no rules, so the
// reconciler creates no policy for it.
func TestEgressRules_None(t *testing.T) {
	got, err := egressRules(&v1alpha1.TaskSpec{Env: []*v1alpha1.EnvVar{{Name: "A", Value: "x"}}}, "default")
	if err != nil || len(got) != 0 {
		t.Fatalf("egressRules = %v, %v; want no rules", got, err)
	}
}
