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

// egressRules turns spec.egress.hosts of a task into the rules of its actor's
// egress policy on Substrate, one rule per host: a host without a scheme is a
// TLS passthrough rule, "http://" an HTTP rule, and "https://" an HTTPS rule,
// each on the port of the host. The API server validated the hosts when the
// task was created, so a host that does not parse here is an error of the
// stored task, and the caller fails the task with it.
func egressRules(spec *v1alpha1.EgressSpec) ([]*ateapipb.EgressRule, error) {
	rules := make([]*ateapipb.EgressRule, 0, len(spec.GetHosts()))
	for i, h := range spec.GetHosts() {
		e, err := v1alpha1.ParseEgressHost(h)
		if err != nil {
			return nil, fmt.Errorf("spec.egress.hosts[%d]: %w", i, err)
		}
		hosts, ports := []string{e.Host}, &ateapipb.Ports{Numbers: []int32{e.Port}}
		switch e.Protocol {
		case v1alpha1.EgressHTTP:
			rules = append(rules, &ateapipb.EgressRule{Http: &ateapipb.HTTPRule{Hostnames: hosts, Ports: ports}})
		case v1alpha1.EgressHTTPS:
			rules = append(rules, &ateapipb.EgressRule{Https: &ateapipb.HTTPSRule{Hostnames: hosts, Ports: ports}})
		default:
			rules = append(rules, &ateapipb.EgressRule{TlsPassthrough: &ateapipb.TLSPassthroughRule{Hostnames: hosts, Ports: ports}})
		}
	}
	return rules, nil
}
