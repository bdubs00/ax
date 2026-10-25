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
	"fmt"

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"github.com/google/ax/pkg/apis/v1alpha1"
)

// credentialURI names a Kubernetes Secret for Substrate's k8s.io credential
// provider, which resolves it in the egress gateway. The provider lets an
// actor resolve only the namespaces its atespace is granted.
func credentialURI(namespace string, ref *v1alpha1.SecretEnvSource) string {
	return fmt.Sprintf("ate-secret://k8s.io/default/%s/%s/%s", namespace, ref.GetName(), ref.GetKey())
}

// egressRules turns spec.egress.hosts and the secret_refs of spec.env into the
// rules of the task's actor egress policy on Substrate, one rule per host: a
// host without a scheme is a TLS passthrough rule, "http://" an HTTP rule,
// and "https://" an HTTPS rule, each on the port of the host.
//
// A secret_ref adds a header replacement to the rules of its inject.host,
// with a credential URI of the Secret in secretNamespace. The gateway can only
// replace a header in a request it reads, so an HTTP or HTTPS entry of that
// host keeps its protocol and port, a TLS entry becomes an HTTPS rule on its
// port, and a host without an entry gets an HTTPS rule on port 443.
//
// The API server validated the hosts when the task was created, so a host
// that does not parse here is an error of the stored task, and the caller
// fails the task with it.
func egressRules(spec *v1alpha1.TaskSpec, secretNamespace string) ([]*ateapipb.EgressRule, error) {
	injections := map[string][]*ateapipb.CredentialHeader{}
	var injectedHosts []string
	for _, e := range spec.GetEnv() {
		ref := e.GetSecretRef()
		if ref == nil {
			continue
		}
		host := ref.GetInject().GetHost()
		if _, ok := injections[host]; !ok {
			injectedHosts = append(injectedHosts, host)
		}
		injections[host] = append(injections[host], &ateapipb.CredentialHeader{
			Header:        ref.GetInject().GetHeader(),
			Prefix:        ref.GetInject().GetPrefix(),
			CredentialUri: credentialURI(secretNamespace, ref),
		})
	}
	effects := func(host string) *ateapipb.HttpRuleEffects {
		if headers := injections[host]; len(headers) > 0 {
			return &ateapipb.HttpRuleEffects{ReplaceHeaders: headers}
		}
		return nil
	}

	listed := map[string]bool{}
	rules := make([]*ateapipb.EgressRule, 0, len(spec.GetEgress().GetHosts())+len(injectedHosts))
	for i, h := range spec.GetEgress().GetHosts() {
		e, err := v1alpha1.ParseEgressHost(h)
		if err != nil {
			return nil, fmt.Errorf("spec.egress.hosts[%d]: %w", i, err)
		}
		listed[e.Host] = true
		hosts, ports := []string{e.Host}, &ateapipb.Ports{Numbers: []int32{e.Port}}
		switch {
		case e.Protocol == v1alpha1.EgressHTTP:
			rules = append(rules, &ateapipb.EgressRule{Http: &ateapipb.HTTPRule{Hostnames: hosts, Ports: ports, Effects: effects(e.Host)}})
		case e.Protocol == v1alpha1.EgressHTTPS || injections[e.Host] != nil:
			rules = append(rules, &ateapipb.EgressRule{Https: &ateapipb.HTTPSRule{Hostnames: hosts, Ports: ports, Effects: effects(e.Host)}})
		default:
			rules = append(rules, &ateapipb.EgressRule{TlsPassthrough: &ateapipb.TLSPassthroughRule{Hostnames: hosts, Ports: ports}})
		}
	}
	for _, host := range injectedHosts {
		if listed[host] {
			continue
		}
		rules = append(rules, &ateapipb.EgressRule{Https: &ateapipb.HTTPSRule{
			Hostnames: []string{host},
			Ports:     &ateapipb.Ports{Numbers: []int32{443}},
			Effects:   effects(host),
		}})
	}
	return rules, nil
}
