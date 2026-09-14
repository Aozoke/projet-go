package redis

type CommandType string

const (
	CommandSet    CommandType = "SET"
	CommandGet    CommandType = "GET"
	CommandWhere  CommandType = "GET_WHERE"
	CommandDelete CommandType = "DELETE"
	CommandAll    CommandType = "ALL"
)

type FilterField string

const (
	FilterKey   FilterField = "key"
	FilterValue FilterField = "value"
)

type FilterOperator string

const (
	OperatorEquals         FilterOperator = "equals"
	OperatorContains       FilterOperator = "contains"
	OperatorGreaterThan    FilterOperator = ">"
	OperatorGreaterOrEqual FilterOperator = ">="
	OperatorLessThan       FilterOperator = "<"
	OperatorLessOrEqual    FilterOperator = "<="
)

type Command struct {
	Type        CommandType
	Key         string
	Value       string
	TTLSeconds  int64
	FilterField FilterField
	Operator    FilterOperator
	FilterValue string
}

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Operation struct {
	Type      CommandType `json:"type"`
	Key       string      `json:"key"`
	Value     string      `json:"value,omitempty"`
	ExpiresAt int64       `json:"expiresAt,omitempty"`
}

// StoredValue est la valeur réellement gardée en RAM et dans le snapshot.
// ExpiresAt vaut 0 quand la clé n'a pas de TTL.
type StoredValue struct {
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

type Snapshot map[string]StoredValue

type Result struct {
	OK      bool    `json:"ok"`
	Value   string  `json:"value,omitempty"`
	Entries []Entry `json:"entries,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type BatchResult struct {
	Results []Result    `json:"results"`
	Writes  []Operation `json:"writes"`
}
