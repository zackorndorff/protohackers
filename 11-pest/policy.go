package main

import (
	"z10f.com/golang/protohackers/11/protocol"
)

type PolicyAction int

const (
	Cull PolicyAction = iota
	Conserve
)

type Policy struct {
	Action   PolicyAction
	RemoteId uint32
}

type PolicySet struct {
	Policies map[string]Policy
}

func NewPolicySet() *PolicySet {
	return &PolicySet{
		Policies: make(map[string]Policy),
	}
}

func (ps *PolicySet) Subset(other *PolicySet) bool {
	for species, mypolicy := range ps.Policies {
		if otherpolicy, ok := other.Policies[species]; !ok || mypolicy.Action != otherpolicy.Action {
			return false
		}
	}
	return true
}

func (ps *PolicySet) Equal(other *PolicySet) bool {
	return ps.Subset(other) && other.Subset(ps)
}

type PolicyDiff struct {
	Deletions []uint32
	Additions map[string]Policy
}

func (ps *PolicySet) Diff(desired *PolicySet) PolicyDiff {
	pd := PolicyDiff{
		Deletions: make([]uint32, 0),
		Additions: make(map[string]Policy),
	}

	// remove now-defunct policies that do not exist in desired
	for species, oldpolicy := range ps.Policies {
		if _, ok := desired.Policies[species]; !ok {
			pd.Deletions = append(pd.Deletions, oldpolicy.RemoteId)
		}
	}

	for species, dpolicy := range desired.Policies {
		oldpolicy, ok := ps.Policies[species]
		if !ok {
			pd.Additions[species] = dpolicy
		} else if oldpolicy.Action != dpolicy.Action {
			pd.Deletions = append(pd.Deletions, oldpolicy.RemoteId)
			pd.Additions[species] = dpolicy
		}
	}
	return pd
}

func (ps *PolicySet) ApplyDiff(pd PolicyDiff) {
	for _, del := range pd.Deletions {
		for species, policy := range ps.Policies {
			if policy.RemoteId == del {
				delete(ps.Policies, species)
				break
			}
		}
	}

	for species, policy := range pd.Additions {
		ps.Policies[species] = policy
	}
}

func ComputePolicy(target []protocol.TargetPopulation, observe []protocol.SitePopulation) *PolicySet {
	ps := NewPolicySet()
	observemap := make(map[string]uint32)
	for _, observepop := range observe {
		observemap[observepop.Species] = observepop.Count
	}

	for _, targetpop := range target {
		// TODO check for dupes
		observed, ok := observemap[targetpop.Species]
		if !ok {
			observed = 0
		}
		if observed < targetpop.Min {
			ps.Policies[targetpop.Species] = Policy{Action: Conserve}
		} else if observed > targetpop.Max {
			ps.Policies[targetpop.Species] = Policy{Action: Cull}
		}
	}
	return ps
}
