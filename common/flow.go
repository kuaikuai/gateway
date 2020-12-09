package common

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/lib/fasthttp"
	"reflect"
	"strings"
)

/*
# Rules：
METHOD 			PATH 						FLOW
GET 			/							name=cache_first flow =[ get_cache >> forward >> set_cache ]
GET				/_cat/*item					name=forward flow=[forward]
POST || PUT		/:index/_doc/*id			name=forward flow=[forward]
POST || PUT		/:index/_bulk || /_bulk 	name=async_indexing_via_translog flow=[ save_translog ]
POST || GET		/*index/_search				name=cache_first flow=[ get_cache >> forward >> set_cache ]
POST || PUT		/:index/_bulk || /_bulk 	name=async_dual_writes flow=[ save_translog{name=dual_writes_id1, retention=7days, max_size=10gb} ]
POST || PUT		/:index/_bulk || /_bulk 	name=sync_dual_writes flow=[ mirror_forward ]
GET				/audit/*operations			name=secured_audit_access flow=[ basic_auth >> flow{name=cache_first} ]
*/

type FilterFlow struct {
	ID string
	Filters []RequestFilter
}

//func NewFilterFlow(name string, filters ...func(ctx *fasthttp.RequestCtx)) FilterFlow {
//	flow := FilterFlow{FlowName: name, Filters: filters}
//	return flow
//}

func (flow *FilterFlow) JoinFilter(filter ...RequestFilter) *FilterFlow {
	for _,v:=range filter{
		flow.Filters=append(flow.Filters,v)
	}
	return flow
}

func (flow *FilterFlow) JoinFlows(flows ...FilterFlow) *FilterFlow {
	for _, v := range flows {
		for _, x := range v.Filters {
			flow.Filters = append(flow.Filters, x)
		}
	}
	return flow
}

func (flow *FilterFlow) ToString() string {
	str:=strings.Builder{}
	has:=false
	for _,v:=range flow.Filters{
		if has{
			str.WriteString(" > ")
		}
		str.WriteString(v.Name())
		has=true
	}
	return str.String()
}

func (flow *FilterFlow) Process(ctx *fasthttp.RequestCtx) {
	for _, v := range flow.Filters {
		if !ctx.ShouldContinue(){
			if global.Env().IsDebug{
				log.Debugf("filter [%v] not continued",v.Name())
			}
			break
		}
		if global.Env().IsDebug{
			log.Debugf("processing filter [%v]",v.Name())
		}
		v.Process(ctx)
	}
}

func MustGetFlow(flowID string) FilterFlow {
	v, ok := flows[flowID]
	if ok {
		return v
	}

	v=FilterFlow{}
	cfg:=GetFlowConfig(flowID)
	for _,z:=range cfg.Filters{
		f:= GetFilterInstanceWithConfig(&z)
		v.JoinFilter(f)
	}
	flows[flowID]=v
	return v
}


func GetFlowProcess(flowID string) func(ctx *fasthttp.RequestCtx) {
	flow := MustGetFlow(flowID)
	return flow.Process
}

func JoinFlows(flowID ...string) FilterFlow {
	flow := FilterFlow{}
	for _, v := range flowID {
		temp := MustGetFlow(v)
		flow.JoinFlows(flow, temp)
	}
	return flow
}

func GetFilter(name string) RequestFilter {
	v, ok := filters[name]
	if !ok{
		panic(errors.Errorf("filter [%s] not found",name))
	}
	return v
}


func GetFilterInstanceWithConfig(cfg *FilterConfig) RequestFilter {

		log.Tracef("get filter instance [%v]", cfg.Name)

		filter:=GetFilter(cfg.Type)
		t := reflect.ValueOf(filter).Type()
		v := reflect.New(t).Elem()

		f := v.FieldByName("Data")
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
			f.Set(reflect.ValueOf(cfg.Parameters))
		}
		return v.Interface().(RequestFilter)
}

var filters map[string]RequestFilter = make(map[string]RequestFilter)
var flows map[string]FilterFlow = make(map[string]FilterFlow)

var filterConfigs map[string]FilterConfig = make(map[string]FilterConfig)
var routingRules map[string]RuleConfig = make(map[string]RuleConfig)
var flowConfigs map[string]FlowConfig = make(map[string]FlowConfig)
var routerConfigs map[string]RouterConfig = make(map[string]RouterConfig)

func RegisterFilter(filter RequestFilter) {
	log.Trace("register filter: ",filter.Name())
	filters[filter.Name()] = filter
}

func RegisterFlow(flow FilterFlow) {
	flows[flow.ID] = flow
}

func RegisterFlowConfig(flow FlowConfig) {
	flowConfigs[flow.Name] = flow
}

func RegisterRoutingRule(rule RuleConfig) {
	routingRules[rule.ID] = rule
}
func RegisterRouterConfig(config RouterConfig) {
	routerConfigs[config.Name] = config
}

func GetRouter(name string) RouterConfig {
	v,ok:=  routerConfigs[name]
	if !ok{
		panic(errors.Errorf("router [%s] not found",name))
	}
	return v
}

func GetRule(name string) RuleConfig {
	v,ok:= routingRules[name]
	if !ok{
		panic(errors.Errorf("rule [%s] not found",name))
	}
	return v
}

func GetFlowConfig(name string) FlowConfig {
	v,ok:= flowConfigs[name]
	if !ok{
		panic(errors.Errorf("flow [%s] not found",name))
	}
	return v
}
