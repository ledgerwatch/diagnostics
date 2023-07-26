package erigon_node

type Versions struct {
	Success        bool
	Error          string
	NodeVersion    uint64
	SupportVersion uint64
	CodeVersion    string
	GitCommit      string
}

type ReorgInDetail struct {
	ParentHash  string
	BlockNum    uint64
	Coinbase    string
	Root        string
	GasLimit    uint64
    GasUsed     uint64
}

type Flags struct {
	Success     bool
	Error       string
	FlagPayload map[string]string
}

type CmdLineArgs struct {
	Success bool
	Error   string
	Args    string
}

type BodyDownload struct {
	Legends       [9]bool
	BlockNum      uint64
	Pre1          []struct{}
	Pre10         []struct{}
	Pre100        []struct{}
	Pre1_000      []struct{}
	Pre10_000     []struct{}
	Pre100_000    []struct{}
	Pre1_000_000  []struct{}
	Pre10_000_000 []struct{}
	States        []SnapshotItem
}

type HeaderDownload struct {
	Legends       [9]bool
	HeaderNum     uint64
	Pre1          []struct{}
	Pre10         []struct{}
	Pre100        []struct{}
	Pre1_000      []struct{}
	Pre10_000     []struct{}
	Pre100_000    []struct{}
	Pre1_000_000  []struct{}
	Pre10_000_000 []struct{}
	States        []SnapshotItem
}

type SnapshotItem struct {
	Id    uint64
	State byte
}

const SuccessLine = "SUCCESS"
