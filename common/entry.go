/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package common

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
)

type EntryConfig struct {
	Enabled           bool `config:"enabled"`
	DirtyShutdown     bool `config:"dirty_shutdown"`
	ReduceMemoryUsage bool `config:"reduce_memory_usage"`
	Name              string               `config:"name"`
	ReadTimeout       int                  `config:"read_timeout"`
	WriteTimeout      int                  `config:"write_timeout"`

	ReadBufferSize int   `config:"read_buffer_size"`
	WriteBufferSize int  `config:"write_buffer_size"`

	MaxRequestBodySize    int                  `config:"max_request_body_size"`
	MaxConcurrency    int                  `config:"max_concurrency"`
	TLSConfig         config.TLSConfig     `config:"tls"`
	NetworkConfig     config.NetworkConfig `config:"network"`
	RouterConfigName  string               `config:"router"`
}

type RuleConfig struct {
	ID          string   `config:"id"`
	Description string   `config:"desc"`
	Method      []string `config:"method"`
	PathPattern []string `config:"pattern"`
	Flow        []string `config:"flow"`
}

type FilterConfig struct {
	ID         string                 `config:"id"`
	Name       string                 `config:"name"`
	Parameters map[string]interface{} `config:"parameters"`
}

type RouterConfig struct {
	Name        string       `config:"name"`
	DefaultFlow string       `config:"default_flow"`
	Rules       []RuleConfig `config:"rules"`
	TracingFlow string       `config:"tracing_flow"`
}

type FlowConfig struct {
	Name      string         `config:"name"`
	Filters   []FilterConfig `config:"filter_v1"`
	FiltersV2 pipeline.PluginConfig   `config:"filter"`
}
