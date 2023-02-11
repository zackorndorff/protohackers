package main

import (
	"fmt"
	"log"
	"net"
	"z10f.com/golang/protohackers/11/protocol"
)

type SiteAuthority struct {
	ps        *PolicySet
	visitChan chan *protocol.MsgSiteVisit
}

type Authority struct {
	visits chan *protocol.MsgSiteVisit
	sites  map[uint32]*SiteAuthority
}

func (sa *SiteAuthority) handleVisits() {
	for visit := range sa.visitChan {
		sa.handleVisit(visit)
	}
}

func (auth *Authority) handleVisits() {
	for visit := range auth.visits {
		site := visit.Site
		sa, ok := auth.sites[site]
		if !ok {
			sa = &SiteAuthority{
				ps:        NewPolicySet(),
				visitChan: make(chan *protocol.MsgSiteVisit),
			}
			auth.sites[site] = sa
			go sa.handleVisits()
		}
		sa.visitChan <- visit
	}
}

func (sa *SiteAuthority) getTarget(conn net.Conn, site uint32) ([]protocol.TargetPopulation, error) {
	outMsg := &protocol.MsgDialAuthority{Site: site}
	sendMsg(conn, outMsg)

	u := protocol.NewUnmarshallerForTesting()
	msg, err := u.UnmarshalMessage(conn)
	if err != nil {
		log.Fatalf("error recving target")
	}
	target, ok := msg.(*protocol.MsgTargetPopulations)
	if !ok {
		log.Printf("did not get target, got %+v\n", msg)
		return nil, fmt.Errorf("got %+v rather than target", msg)
	}
	return target.Populations, nil
}

func (sa *SiteAuthority) getPolicySet(site uint32) *PolicySet {
	if sa.ps == nil {
		sa.ps = NewPolicySet()
	}
	return sa.ps
}

func (sa *SiteAuthority) handleVisit(msg *protocol.MsgSiteVisit) {
	u := protocol.NewUnmarshallerForTesting()
	conn, err := net.Dial("tcp", "pestcontrol.protohackers.com:20547")
	if err != nil {
		log.Fatalf("error dialing authority server %s", err)
	}
	defer conn.Close()

	err = sendMsg(conn, &protocol.MsgHello{Protocol: "pestcontrol", Version: 1})
	if err != nil {
		log.Fatalf("error sending hello")
	}

	helloMsg, err := u.UnmarshalMessage(conn)
	if err != nil {
		log.Fatalf("error recving hello")
	}
	hello, ok := helloMsg.(*protocol.MsgHello)
	if !ok {
		log.Fatalf("did not get hello, got %+v\n", hello)
	}

	target, err := sa.getTarget(conn, msg.Site)
	if err != nil {
		return
	}

	newpolicy := ComputePolicy(target, msg.Populations)
	ps := sa.getPolicySet(msg.Site)
	pdiff := ps.Diff(newpolicy)
	log.Println("diff is", pdiff)
	sa.applyDiffToServer(conn, ps, pdiff)
}

func waitForOk(conn net.Conn) {
	u := protocol.NewUnmarshallerForTesting()
	msg, err := u.UnmarshalMessage(conn)
	if err != nil {
		log.Fatalf("error recving ok")
	}
	hello, ok := msg.(*protocol.MsgOk)
	if !ok {
		log.Fatalf("did not get ok, got %+v\n", hello)
	}
}

func waitForResult(conn net.Conn) uint32 {
	u := protocol.NewUnmarshallerForTesting()
	msg, err := u.UnmarshalMessage(conn)
	if err != nil {
		log.Fatalf("error recving result")
	}
	res, ok := msg.(*protocol.MsgPolicyResult)
	if !ok {
		log.Fatalf("did not get result, got %+v\n", res)
	}
	return res.Policy
}

func (sa *SiteAuthority) applyDiffToServer(conn net.Conn, ps *PolicySet, pd PolicyDiff) {
	for _, del := range pd.Deletions {
		msg := &protocol.MsgDeletePolicy{Policy: del}
		err := sendMsg(conn, msg)
		if err != nil {
			log.Fatalf("Error deleting: %s", err)
		}
		waitForOk(conn)
		for species, policy := range ps.Policies {
			if policy.RemoteId == del {
				delete(ps.Policies, species)
				break
			}
		}
	}

	for species, policy := range pd.Additions {
		action := protocol.ActionConserve
		if policy.Action == Cull {
			action = protocol.ActionCull
		}

		msg := &protocol.MsgCreatePolicy{
			Species: species,
			Action:  action,
		}
		err := sendMsg(conn, msg)
		if err != nil {
			log.Fatalf("Error deleting: %s", err)
		}
		policy.RemoteId = waitForResult(conn)
		ps.Policies[species] = policy
	}
}

func (auth *Authority) HandleVisit(msg *protocol.MsgSiteVisit) {
	auth.visits <- msg
}

func NewAuthority() *Authority {
	auth := &Authority{
		visits: make(chan *protocol.MsgSiteVisit),
		sites:  make(map[uint32]*SiteAuthority),
	}
	go auth.handleVisits()
	return auth
}
