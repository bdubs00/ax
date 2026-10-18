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
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"github.com/google/ax/pkg/apis/v1alpha1"
)

// Default ports of the egress protocols, when a rule names none.
const (
	defaultHTTPPort  = 80
	defaultHTTPSPort = 443
	defaultTLSPort   = 443
)

// egressRules turns the egress section of a task into the rules of its
// actor's egress policy on Substrate. A rule without ports gets the default
// port of its protocol. The API server validated the section when the task
// was created, so every rule names one protocol and at least one hostname.
func egressRules(spec *v1alpha1.EgressSpec) []*ateapipb.EgressRule {
	rules := make([]*ateapipb.EgressRule, 0, len(spec.GetRules()))
	for _, r := range spec.GetRules() {
		switch {
		case r.GetHttp() != nil:
			rules = append(rules, &ateapipb.EgressRule{Http: &ateapipb.HTTPRule{
				Hostnames: r.GetHttp().GetHostnames(), Ports: egressPorts(r.GetHttp(), defaultHTTPPort),
			}})
		case r.GetHttps() != nil:
			rules = append(rules, &ateapipb.EgressRule{Https: &ateapipb.HTTPSRule{
				Hostnames: r.GetHttps().GetHostnames(), Ports: egressPorts(r.GetHttps(), defaultHTTPSPort),
			}})
		case r.GetTls() != nil:
			rules = append(rules, &ateapipb.EgressRule{TlsPassthrough: &ateapipb.TLSPassthroughRule{
				Hostnames: r.GetTls().GetHostnames(), Ports: egressPorts(r.GetTls(), defaultTLSPort),
			}})
		}
	}
	return rules
}

// egressPorts selects the ports of a rule, or the default port of its
// protocol when the rule names none.
func egressPorts(h *v1alpha1.EgressHosts, def int32) *ateapipb.Ports {
	ports := h.GetPorts()
	if len(ports) == 0 {
		ports = []int32{def}
	}
	return &ateapipb.Ports{Numbers: ports}
}
