package protocol

const MsgTypeHello = 0x50

type MsgHello struct {
	Protocol string
	Version  uint32
}

const MsgTypeError = 0x51

type MsgError struct {
	Message string
}

const MsgTypeOk = 0x52

type MsgOk struct {
}

const MsgTypeDialAuthority = 0x53

type MsgDialAuthority struct {
	Site uint32
}

type TargetPopulation struct {
	Species string
	Min     uint32
	Max     uint32
}

const MsgTypeTargetPopulations = 0x54

type MsgTargetPopulations struct {
	Site            uint32
	PopulationCount uint32 `zmarshal:"length:Populations"`
	Populations     []TargetPopulation
}

const ActionCull = byte(0x90)
const ActionConserve = byte(0xa0)
const MsgTypeCreatePolicy = 0x55

type MsgCreatePolicy struct {
	Species string
	Action  byte
}

const MsgTypeDeletePolicy = 0x56

type MsgDeletePolicy struct {
	Policy uint32
}

const MsgTypePolicyResult = 0x57

type MsgPolicyResult struct {
	Policy uint32
}

type SitePopulation struct {
	Species string
	Count   uint32
}

const MsgTypeSiteVisit = 0x58

type MsgSiteVisit struct {
	Site            uint32
	PopulationCount uint32 `zmarshal:"length:Populations"`
	Populations     []SitePopulation
}
