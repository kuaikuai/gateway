/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package elastic

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/gateway/common"
)

type BulkResponseProcess struct {
	id        string
	config    *Config
	retryFlow *common.FilterFlow
}

func (this *BulkResponseProcess) Name() string {
	return "bulk_response_process"
}

func (this *BulkResponseProcess) Filter(ctx *fasthttp.RequestCtx) {
	path := string(ctx.PhantomURI().Path())
	if string(ctx.Request.Header.Method()) != "POST" || !util.ContainStr(path, "_bulk") {
		return
	}

	if ctx.Response.StatusCode() == http.StatusOK || ctx.Response.StatusCode() == http.StatusCreated {
		var resbody = ctx.Response.GetRawBody()
		requestBytes := ctx.Request.GetRawBody()

		successItems := elastic.AcquireBulkBuffer()
		nonRetryableItems := elastic.AcquireBulkBuffer()
		retryableItems := elastic.AcquireBulkBuffer()

		var containError bool
		var bulkResults *elastic.BulkResult

		defer func() {
			elastic.ReturnBulkBuffer(successItems)
			elastic.ReturnBulkBuffer(nonRetryableItems)
			elastic.ReturnBulkBuffer(retryableItems)

		}()

		label := util.MapStr{}

		containError, _, bulkResults = elastic.HandleBulkResponse(&ctx.Request, &ctx.Response, label, requestBytes, resbody, successItems, nonRetryableItems, retryableItems, this.config.BulkResponseParseConfig, this.config.RetryRules)

		if bulkResults != nil {
			ctx.Set("bulk_response_status", bulkResults)
		}

		//stats only, skip further process
		if this.config.StatsOnly {
			return
		}

		if containError {

			url := ctx.Request.PhantomURI().String()
			if rate.GetRateLimiter("bulk_error", url, 1, 1, 5*time.Second).Allow() {
				log.Error("error in bulk requests,", url, ",", ctx.Response.StatusCode(), ",invalid:", nonRetryableItems.GetMessageCount(), ",failure:", retryableItems.GetMessageCount(), ",", util.SubString(util.UnsafeBytesToString(resbody), 0, this.config.MessageTruncateSize))
			}

			if len(this.config.TagsOnAnyError) > 0 {
				ctx.UpdateTags(this.config.TagsOnAnyError, nil)
			}

			if nonRetryableItems.GetMessageCount() > 0 {

				if this.config.InvalidQueue != "" {
					nonRetryableItems.SafetyEndWithNewline()
					bytes := ctx.Request.OverrideBodyEncode(nonRetryableItems.GetMessageBytes(), true)
					queue.Push(queue.GetOrInitConfig(this.config.InvalidQueue), bytes)
				}

				if len(this.config.TagsOnPartialInvalid) > 0 {
					ctx.UpdateTags(this.config.TagsOnPartialInvalid, nil)
				}

				if successItems.GetMessageCount() == 0 && retryableItems.GetMessageCount() == 0 {
					if len(this.config.TagsOnAllInvalid) > 0 {
						ctx.UpdateTags(this.config.TagsOnAllInvalid, nil)
					}
				}
			}

			if retryableItems.GetMessageCount() > 0 {

				if this.config.FailureQueue != "" {
					retryableItems.SafetyEndWithNewline()
					if retryableItems.GetMessageSize() == 0 || len(retryableItems.GetMessageBytes()) == 0 {
						log.Error("invalid retryable items, size should not be 0, but is 0,", retryableItems.GetMessageCount())
					}

					bytes := ctx.Request.OverrideBodyEncode(retryableItems.GetMessageBytes(), true)

					if this.config.PartialFailureRetry && this.retryFlow != nil {
						ctx.AddFlowProcess("retry_flow:" + this.retryFlow.ID)
						this.retryFlow.Process(ctx)
					}

					queue.Push(queue.GetOrInitConfig(this.config.FailureQueue), bytes)
				}

				if len(this.config.TagsOnPartialFailure) > 0 {
					ctx.UpdateTags(this.config.TagsOnPartialFailure, nil)
				}

				if successItems.GetMessageCount() == 0 && nonRetryableItems.GetMessageCount() == 0 {
					if len(this.config.TagsOnAllFailure) > 0 {
						ctx.UpdateTags(this.config.TagsOnAllFailure, nil)
					}
				}
			}

			if successItems.GetMessageCount() > 0 {

				if this.config.SuccessQueue != "" {
					successItems.SafetyEndWithNewline()
					bytes := ctx.Request.OverrideBodyEncode(successItems.GetMessageBytes(), true)
					queue.Push(queue.GetOrInitConfig(this.config.SuccessQueue), bytes)
				}

				if len(this.config.TagsOnPartialSuccess) > 0 {
					ctx.UpdateTags(this.config.TagsOnPartialSuccess, nil)
				}
			}

			//出错不继续交由后续流程，直接结束处理
			if !this.config.ContinueOnAnyError {
				ctx.Finished()
				return
			}
		} else {
			//没有错误，标记处理完成
			if len(this.config.TagsOnAllSuccess) > 0 {
				ctx.UpdateTags(this.config.TagsOnAllSuccess, nil)
			}

			if this.config.SuccessQueue != "" {
				queue.Push(queue.GetOrInitConfig(this.config.SuccessQueue), ctx.Request.Encode())
			}

			if !this.config.ContinueOnSuccess {
				ctx.Finished()
				return
			}
		}
	} else {

		if len(this.config.TagsOnNone2xx) > 0 {
			ctx.UpdateTags(this.config.TagsOnNone2xx, nil)
		}

		if this.config.FailureQueue != "" {
			if this.config.RetryRules.Retryable(ctx.Response.StatusCode(), string(ctx.Response.GetRawBody())) {
				bytes := ctx.Request.Encode()
				if len(bytes) == 0 {
					log.Error("retryable items, size:", len(bytes))
				}
				queue.Push(queue.GetOrInitConfig(this.config.FailureQueue), bytes)
			}
		}

		if !this.config.ContinueOnAllError {
			ctx.Finished()
			return
		}
	}
}

type Config struct {
	StatsOnly    bool   `config:"stats_only"`
	SuccessQueue string `config:"success_queue"`
	InvalidQueue string `config:"invalid_queue"`
	FailureQueue string `config:"failure_queue"`

	MessageTruncateSize int `config:"message_truncate_size"`

	PartialFailureRetry                 bool `config:"partial_failure_retry"`               //是否主动重试，只有部分失败的请求，避免大量没有意义的 409
	PartialFailureMaxRetryTimes         int  `config:"partial_failure_max_retry_times"`     //是否主动重试，只有部分失败的请求，避免大量没有意义的 409
	PartialFailureRetryDelayLatencyInMs int  `config:"partial_failure_retry_latency_in_ms"` //是否主动重试，只有部分失败的请求，避免大量没有意义的 409

	ContinueOnAllError bool `config:"continue_on_all_error"` //没有拿到响应，整个请求都失败是否继续处理后续 flow
	ContinueOnAnyError bool `config:"continue_on_any_error"` //拿到响应，出现任意请求失败是否都继续 flow 还是结束处理
	ContinueOnSuccess  bool `config:"continue_on_success"`   //所有请求都成功

	TagsOnAllSuccess []string `config:"tag_on_all_success"` //所有请求都成功，没有失败
	TagsOnNone2xx    []string `config:"tag_on_none_2xx"`    //整个 bulk 请求非 200 或者 201 返回

	//bulk requests
	TagsOnAllInvalid []string `config:"tag_on_all_invalid"` //所有请求都是非法请求的情况
	TagsOnAllFailure []string `config:"tag_on_all_failure"` //所有失败的请求都是失败请求的情况

	TagsOnAnyError       []string `config:"tag_on_any_error"`       //请求里面包含任意失败或者非法请求的情况
	TagsOnPartialSuccess []string `config:"tag_on_partial_success"` //包含部分成功的情况
	TagsOnPartialFailure []string `config:"tag_on_partial_failure"` //包含部分失败的情况，可以重试
	TagsOnPartialInvalid []string `config:"tag_on_partial_invalid"` //包含部分非法请求的情况，无需重试的请求

	RetryFlow  string             `config:"retry_flow"`
	RetryRules elastic.RetryRules `config:"retry_rules"`

	BulkResponseParseConfig elastic.BulkResponseParseConfig `config:"response_handle"`
}

func init() {
	pipeline.RegisterFilterPluginWithConfigMetadata("bulk_response_process", NewBulkResponseValidate, &Config{})
}

func NewBulkResponseValidate(c *config.Config) (pipeline.Filter, error) {
	cfg := Config{
		MessageTruncateSize: 1024,
		RetryRules:          elastic.RetryRules{Retry429: true, Default: true, Retry4xx: false},
		BulkResponseParseConfig: elastic.BulkResponseParseConfig{
			BulkResultMessageMaxRequestBodyLength:  10 * 1024,
			BulkResultMessageMaxResponseBodyLength: 10 * 1024,
			OutputBulkStats:                        true,
			IncludeIndexStats:                      true,
			IncludeActionStats:                     true,
			IncludeErrorDetails:                    true,
			MaxItemOfErrorDetailsCount:             50,
		},
	}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the filter configuration : %s", err)
	}
	runner := BulkResponseProcess{
		config: &cfg}

	runner.id = util.GetUUID()

	if runner.config.RetryFlow != "" && runner.config.PartialFailureRetry {
		flow := common.MustGetFlow(runner.config.RetryFlow)
		runner.retryFlow = &flow
	}

	return &runner, nil
}
