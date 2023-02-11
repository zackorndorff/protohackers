package core

import (
	"log"
	"testing"
	"time"
)

type SimpleTestCase struct {
	Limit Limit
	Mile1 uint16
	Ts1   Timestamp
	Mile2 uint16
	Ts2   Timestamp
	Speed Speed
}

// Don't reuse limits or we'll have a problem
var SimpleTicketCases []SimpleTestCase = []SimpleTestCase{
	{Limit: 1, Mile1: 1, Ts1: 1, Mile2: 2, Ts2: 20, Speed: 18947},
	{Limit: 2, Mile1: 2, Ts1: 20, Mile2: 1, Ts2: 1, Speed: 18947},
	{Limit: 0x64, Mile1: 0x649, Mile2: 0x653, Ts1: 70786, Ts2: 71086, Speed: 120 * 100},
}

func TestTicket(t *testing.T) {
	state := NewState()
	go state.MainLoop()
	defer func() {
		state.Shutdown <- struct{}{}
	}()
	for _, c := range SimpleTicketCases {
		road := Road(c.Limit)
		state.RegisterRoad <- &RegisterRoad{
			Limit: c.Limit,
			Road:  Road(c.Limit),
		}
		state.RecordObservation <- &PlateObservation{
			Plate:     "foo",
			Timestamp: c.Ts1,
			Road:      road,
			Mile:      c.Mile1,
		}

		state.RecordObservation <- &PlateObservation{
			Plate:     "foo",
			Timestamp: c.Ts2,
			Road:      road,
			Mile:      c.Mile2,
		}
		tickets := make(chan *Ticket)

		disp := &Dispatcher{
			Roads:      []Road{road},
			SendTicket: tickets,
		}
		log.Println("registering dispatcher")
		state.RegisterDispatcher <- disp
		select {
		case ticket := <-tickets:
			if (c.Ts1 <= c.Ts2 &&
				(ticket.Timestamp1 != c.Ts1 || ticket.Timestamp2 != c.Ts2)) ||
				(c.Ts2 < c.Ts1 &&
					(ticket.Timestamp1 != c.Ts2 || ticket.Timestamp2 != c.Ts1)) {
				t.Errorf("Wrong timestamp in ticket (case %+v)", c)
			}
			if ticket.Road != road {
				t.Errorf("Wrong road in ticket (case %+v)", c)
			}
			if ticket.Speed != c.Speed {
				t.Errorf("Wrong speed in ticket (got %d, expected %d) [units: mph * 100]", ticket.Speed, c.Speed)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Did not get a ticket (case %+v)", c)
		}
		log.Println("unregistering dispatcher")
		state.UnregisterDispatcher <- disp
		close(tickets)
	}
}

func TestNoTicket(t *testing.T) {
	state := NewState()
	go state.MainLoop()
	defer func() {
		state.Shutdown <- struct{}{}
	}()
	state.RegisterRoad <- &RegisterRoad{
		Limit: 1,
		Road:  10,
	}
	state.RecordObservation <- &PlateObservation{
		Plate:     "foo",
		Timestamp: 1,
		Road:      10,
		Mile:      1,
	}

	state.RecordObservation <- &PlateObservation{
		Plate:     "foo",
		Timestamp: 1 + 60*60,
		Road:      10,
		Mile:      2,
	}
	tickets := make(chan *Ticket)

	state.RegisterDispatcher <- &Dispatcher{
		Roads:      []Road{10, 100, 1000},
		SendTicket: tickets,
	}
	select {
	case <-tickets:
		t.Error("Got ticket, shouldn't have")
	case <-time.After(1 * time.Second):
	}
}
