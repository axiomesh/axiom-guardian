package repo

import (
	"time"
)

type Config struct {
	RepoRoot  string    `mapstructure:"-" toml:"-"`
	DialUrl   string    `mapstructure:"dial_url" toml:"dial_url"`
	AxiomPath string    `mapstructure:"axiom_path" toml:"axiom_path"`
	Log       Log       `mapstructure:"log" toml:"log"`
	Subscribe Subscribe `mapstructure:"subscribe" toml:"subscribe"`
}

type Log struct {
	Level        string        `mapstructure:"level" toml:"level"`
	Filename     string        `mapstructure:"filename" toml:"filename"`
	ReportCaller bool          `mapstructure:"report_caller" toml:"report_caller"`
	MaxAge       time.Duration `mapstructure:"max_age" toml:"max_age"`
	RotationTime time.Duration `mapstructure:"rotation_time" toml:"rotation_time"`
}

type Subscribe struct {
	// beginning of the queried range, 1 means genesis block
	FromBlock uint64 `mapstructure:"from_block" toml:"from_block"`
	// end of the range, 0 means latest block
	ToBlock   uint64   `mapstructure:"to_block" toml:"to_block"`
	Addresses []string `mapstructure:"addresses" toml:"addresses"`
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position AND B in second position
	// {{A}, {B}}         matches topic A in first position AND B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position AND (C OR D) in second position
	Topics [][]string `mapstructure:"topics" toml:"topics"`
}

func DefaultConfig(repoRoot string) *Config {
	return &Config{
		RepoRoot: repoRoot,
		DialUrl:  "ws://localhost:9991",
		AxiomPath: "~/.axiom",
		Log: Log{
			Level:        "info",
			Filename:     "guardian.log",
			ReportCaller: false,
			MaxAge:       30 * 24 * time.Hour,
			RotationTime: 24 * time.Hour,
		},
		Subscribe: Subscribe{
			FromBlock: 1,
			ToBlock:   0,
			Addresses: []string{NodeManagerContractAddr},
			// first position is vote method signature's 32 Byte hash, second postion is {} to mean any topic, third is proposal type's hash for update axiom
			Topics: [][]string{{"0xe6bfc3cff2e28bc2ab583f413a459f93526e55a1a46c944572150de96997c84e"}, {}, {"0x0000000000000000000000000000000000000000000000000000000000000001"}},
		},
	}
}
