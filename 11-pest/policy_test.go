package main

import (
	"testing"
	"z10f.com/golang/protohackers/11/protocol"
)

func TestPolicySmoke(t *testing.T) {
	p1 := NewPolicySet()
	p1.Policies["dog"] = Policy{Action: Cull}
	p1.Policies["cat"] = Policy{Action: Conserve}

	p2 := NewPolicySet()
	p2.Policies["dog"] = Policy{Action: Cull}

	if !p2.Subset(p1) || p1.Subset(p2) {
		t.Errorf("set operations fail")
	}

	p3 := NewPolicySet()
	p3.Policies["dog"] = Policy{Action: Conserve}

	if p2.Subset(p3) || p3.Subset(p2) || p2.Equal(p3) {
		t.Errorf("failed to check action")
	}
}

func TestPolicyDiff(t *testing.T) {
	p1 := NewPolicySet()
	p1.Policies["dog"] = Policy{Action: Cull, RemoteId: 1}
	p1.Policies["cat"] = Policy{Action: Conserve, RemoteId: 2}

	p2 := NewPolicySet()
	p2.Policies["cat"] = Policy{Action: Cull}

	pd := p1.Diff(p2)

	p1.ApplyDiff(pd)

	if !p1.Equal(p2) {
		t.Errorf("diff failed to apply")
	}

}

func TestSmokeComputePolicy(t *testing.T) {
	observations := make([]protocol.SitePopulation, 0)
	observations = append(observations, protocol.SitePopulation{Species: "long-tailed rat", Count: 20})

	target := make([]protocol.TargetPopulation, 0)
	target = append(target, protocol.TargetPopulation{Species: "long-tailed rat", Min: 0, Max: 10})

	ps := ComputePolicy(target, observations)

	expectedps := NewPolicySet()
	expectedps.Policies["long-tailed rat"] = Policy{Action: Cull}

	if !ps.Equal(expectedps) {
		t.Errorf("unexpected policy")
	}
}

func TestMoreComputePolicy(t *testing.T) {
	observations := make([]protocol.SitePopulation, 0)
	observations = append(observations, protocol.SitePopulation{Species: "long-tailed rat", Count: 20})
	observations = append(observations, protocol.SitePopulation{Species: "dog", Count: 40})
	observations = append(observations, protocol.SitePopulation{Species: "anteater", Count: 50})

	target := make([]protocol.TargetPopulation, 0)
	target = append(target, protocol.TargetPopulation{Species: "long-tailed rat", Min: 0, Max: 10})
	target = append(target, protocol.TargetPopulation{Species: "cat", Min: 5, Max: 50})
	target = append(target, protocol.TargetPopulation{Species: "dog", Min: 5, Max: 40})
	target = append(target, protocol.TargetPopulation{Species: "anteater", Min: 60, Max: 99})

	ps := ComputePolicy(target, observations)

	expectedps := NewPolicySet()
	expectedps.Policies["long-tailed rat"] = Policy{Action: Cull}
	expectedps.Policies["cat"] = Policy{Action: Conserve}
	expectedps.Policies["anteater"] = Policy{Action: Conserve}

	if !ps.Equal(expectedps) {
		t.Errorf("unexpected policy")
	}
}
