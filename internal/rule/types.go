package rule

type Result int

const (
	Pass  Result = 0
	Alert Result = 1
)

type AlertEntry struct {
	RuleName    string `json:"rule_name"`
	Description string `json:"description"`
	Message     string `json:"message"`
	Passed      bool   `json:"passed"`
}

type Context struct {
	Throughputs []ThroughputEntry `json:"throughputs,omitempty"`
	Retrans     []RetransEntry    `json:"retransmissions,omitempty"`
	RTT         []RTTEntry        `json:"rtt,omitempty"`
	Flows       []FlowEntry       `json:"flows,omitempty"`
}

type ThroughputEntry struct {
	SrcIP   string `json:"src_ip"`
	DstIP   string `json:"dst_ip"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
	TxBytes uint64 `json:"tx_bytes"`
	RxBytes uint64 `json:"rx_bytes"`
}

type RetransEntry struct {
	SrcIP   string `json:"src_ip"`
	DstIP   string `json:"dst_ip"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
	Count   uint64 `json:"count"`
}

type RTTEntry struct {
	SrcIP   string `json:"src_ip"`
	DstIP   string `json:"dst_ip"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
	AvgUs   uint64 `json:"avg_us"`
	MinUs   uint32 `json:"min_us"`
	MaxUs   uint32 `json:"max_us"`
	Count   uint64 `json:"count"`
}

type FlowEntry struct {
	SrcIP   string `json:"src_ip"`
	DstIP   string `json:"dst_ip"`
	SrcPort uint16 `json:"src_port"`
	DstPort uint16 `json:"dst_port"`
	PID     uint32 `json:"pid"`
	Comm    string `json:"comm"`
}
