package core

import (
	"log"
)

type Road uint16
type Plate string
type Limit uint16 // mph
type Speed uint16 // mph * 100
type Day uint32
type Timestamp uint32 // seconds since epoch

type RegisterRoad struct {
	Road  Road
	Limit Limit
}

type PlateObservation struct {
	Plate     Plate
	Timestamp Timestamp
	Road      Road
	Mile      uint16
}

type Dispatcher struct {
	Roads      []Road
	SendTicket chan *Ticket
}

type Ticket struct {
	Plate      Plate
	Road       Road
	Mile1      uint16
	Timestamp1 Timestamp
	Mile2      uint16
	Timestamp2 Timestamp
	Speed      Speed
}

type State struct {
	Dispatchers map[Road][]*Dispatcher
	Cars        map[Plate]map[Road][]*PlateObservation
	RoadLimits  map[Road]Limit // mph

	TicketQueue             map[Road][]*Ticket
	TicketIssuedForCarOnDay map[Plate]map[Day]interface{}

	RecordObservation    chan *PlateObservation
	RegisterRoad         chan *RegisterRoad
	RegisterDispatcher   chan *Dispatcher
	UnregisterDispatcher chan *Dispatcher
	Shutdown             chan interface{} // for testing
}

func DayFromTimestamp(timestamp Timestamp) Day {
	return Day(timestamp / 86400)
}

func registerRoad(s *State, rroad *RegisterRoad) {
	if oldlimit, ok := s.RoadLimits[rroad.Road]; ok {
		if rroad.Limit == oldlimit {
			log.Printf("core: registering already registered road %d with same limit\n", rroad.Road)
		} else {
			log.Printf("core: trying to reregister road %d with different limit %d (current is %d)", rroad.Road, rroad.Limit, oldlimit)
		}
	} else {
		log.Printf("core: registering new road %d with limit %d", rroad.Road, rroad.Limit)
		s.RoadLimits[rroad.Road] = rroad.Limit
	}
}

func checkCompliance(limit Limit, obs *PlateObservation, other *PlateObservation) *Ticket {
	if obs.Mile == other.Mile {
		return nil
	}
	var first_ts, second_ts Timestamp
	var first_m, second_m uint16
	var first, second *PlateObservation
	if obs.Timestamp == other.Timestamp {
		log.Println("core: warn: duplicated timestamp")
		return nil
	} else if obs.Timestamp >= other.Timestamp {
		first_ts = other.Timestamp
		second_ts = obs.Timestamp
		first = other
		second = obs
	} else {
		first_ts = obs.Timestamp
		second_ts = other.Timestamp
		first = obs
		second = other
	}
	if obs.Mile >= other.Mile {
		first_m = other.Mile
		second_m = obs.Mile
	} else {
		first_m = obs.Mile
		second_m = other.Mile
	}

	dt := uint32(second_ts - first_ts)             // unit: seconds
	dp := uint32(second_m-first_m) * 100 * 60 * 60 //lalala // unit: miles*100
	speed := Speed((dp / dt))                      // unit: miles*100 per hour

	if speed >= (Speed(limit*100) + 50) {
		return &Ticket{
			Plate:      obs.Plate,
			Road:       obs.Road,
			Speed:      speed,
			Mile1:      first.Mile,
			Timestamp1: first.Timestamp,
			Mile2:      second.Mile,
			Timestamp2: second.Timestamp,
		}
	} else {
		return nil
	}
}

func (s *State) issueTicket(t *Ticket) {
	pdays, ok := s.TicketIssuedForCarOnDay[t.Plate]
	if !ok {
		pdays = make(map[Day]interface{})
		s.TicketIssuedForCarOnDay[t.Plate] = pdays
	}
	_, issuedToday1 := pdays[DayFromTimestamp(t.Timestamp1)]
	_, issuedToday2 := pdays[DayFromTimestamp(t.Timestamp2)]

	if issuedToday1 || issuedToday2 {
		// Can't issue more than one ticket per day, drop ticket.
		return
	}

	// Mark used days (these may be the same)
	pdays[DayFromTimestamp(t.Timestamp1)] = struct{}{}
	pdays[DayFromTimestamp(t.Timestamp2)] = struct{}{}

	displist, ok := s.Dispatchers[t.Road]
	if ok && len(displist) > 0 {
		log.Printf("core: sending ticket without queue %+v", t)
		displist[0].SendTicket <- t
	} else {
		// Don't have a dispatcher, so we queue the ticket
		log.Printf("core: queueing ticket %+v", t)
		rqueue, ok := s.TicketQueue[t.Road]
		if !ok {
			rqueue = []*Ticket{}
		}
		s.TicketQueue[t.Road] = append(rqueue, t)
	}
}

// responsible for recording an observation and calling issueTicket for all
// relevant tickets
func recordObservation(s *State, obs *PlateObservation) {
	log.Printf("core: handling observation %+v\n", obs)
	// find relevant list of observations
	roadlist, ok := s.Cars[obs.Plate]
	if !ok {
		roadlist = make(map[Road][]*PlateObservation)
		s.Cars[obs.Plate] = roadlist
	}
	obslist, ok := roadlist[obs.Road]
	if !ok {
		obslist = []*PlateObservation{}
	}

	limit := s.RoadLimits[obs.Road]

	// Check compliance and issue tickets if necessary
	for _, oobs := range obslist {
		ticket := checkCompliance(limit, obs, oobs)
		if ticket != nil {
			s.issueTicket(ticket)
		}
	}

	// Finally, add new observation to list of observations
	roadlist[obs.Road] = append(obslist, obs)
}

func registerDispatcher(s *State, rdisp *Dispatcher) {
	for _, road := range rdisp.Roads {
		displist, ok := s.Dispatchers[road]
		if !ok {
			displist = []*Dispatcher{}
		}
		s.Dispatchers[road] = append(displist, rdisp)
		if queued, ok := s.TicketQueue[road]; ok {
			for _, ticket := range queued {
				log.Printf("core: Sending ticket from queue %+v", ticket)
				rdisp.SendTicket <- ticket
			}
			delete(s.TicketQueue, road)
		}
	}
}

func unregisterDispatcher(s *State, udisp *Dispatcher) {
	for _, road := range udisp.Roads {
		displist := s.Dispatchers[road]
		for i, disp := range displist {
			if disp == udisp {
				displist[i] = displist[len(displist)-1]
				displist[len(displist)-1] = nil
				s.Dispatchers[road] = displist[:len(displist)-1]
				log.Printf("core: successfully unregistered dispatcher for road %d", road)
				break
			}
		}
	}
}

func NewState() *State {
	return &State{
		Dispatchers:             make(map[Road][]*Dispatcher),
		Cars:                    make(map[Plate]map[Road][]*PlateObservation),
		RoadLimits:              make(map[Road]Limit),
		TicketQueue:             make(map[Road][]*Ticket),
		TicketIssuedForCarOnDay: make(map[Plate]map[Day]interface{}),
		RecordObservation:       make(chan *PlateObservation),
		RegisterRoad:            make(chan *RegisterRoad),
		RegisterDispatcher:      make(chan *Dispatcher),
		UnregisterDispatcher:    make(chan *Dispatcher),
		Shutdown:                make(chan interface{}),
	}
}

func (s *State) MainLoop() {
	for {
		select {
		case obs := <-s.RecordObservation:
			recordObservation(s, obs)
		case rroad := <-s.RegisterRoad:
			registerRoad(s, rroad)
		case rdisp := <-s.RegisterDispatcher:
			registerDispatcher(s, rdisp)
		case udisp := <-s.UnregisterDispatcher:
			unregisterDispatcher(s, udisp)
		case <-s.Shutdown:
			return
		}
	}
}
