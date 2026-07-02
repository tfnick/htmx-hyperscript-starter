package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type MonitorResponse struct {
	Settings MonitorSettingsResponse    `json:"settings"`
	Rules    []MonitorAlertRuleResponse `json:"rules"`
	Nodes    []MonitorNodeResponse      `json:"nodes"`
	Bots     []MonitorAlertBotResponse  `json:"bots"`
}

type MonitorSettingsResponse struct {
	SampleIntervalSeconds int    `json:"sample_interval_seconds"`
	MinSampleInterval     int    `json:"min_sample_interval_seconds"`
	LatencyProbeEnabled   bool   `json:"latency_probe_enabled"`
	LatencyProbeURL       string `json:"latency_probe_url"`
	LatencyProbeTimeoutMS int    `json:"latency_probe_timeout_ms"`
	AlertBotChannelID     string `json:"alert_bot_channel_id"`
	DailyAlertLimit       int    `json:"daily_alert_limit"`
	UpdatedAt             string `json:"updated_at"`
}

type MonitorAlertRuleResponse struct {
	MetricKey        string  `json:"metric_key"`
	Enabled          bool    `json:"enabled"`
	ThresholdValue   float64 `json:"threshold_value"`
	SustainedSamples int     `json:"sustained_samples"`
	UpdatedAt        string  `json:"updated_at"`
}

type MonitorAlertBotResponse struct {
	ID           string `json:"id"`
	ChannelCode  string `json:"channel_code"`
	ProviderCode string `json:"provider_code"`
	AdapterKey   string `json:"adapter_key"`
	Enabled      bool   `json:"enabled"`
	IsPrimary    bool   `json:"is_primary"`
}

type MonitorNodeResponse struct {
	NodeID    string                  `json:"node_id"`
	Hostname  string                  `json:"hostname"`
	SampledAt string                  `json:"sampled_at"`
	Stale     bool                    `json:"stale"`
	Latest    MonitorSampleResponse   `json:"latest"`
	History   []MonitorSampleResponse `json:"history"`
}

type MonitorSampleResponse struct {
	SampledAt string                 `json:"sampled_at"`
	CPU       MonitorCPUResponse     `json:"cpu"`
	Memory    MonitorMemoryResponse  `json:"memory"`
	Disk      MonitorDiskResponse    `json:"disk"`
	Latency   MonitorLatencyResponse `json:"latency"`
}

type MonitorCPUResponse struct {
	UsagePercent     float64 `json:"usage_percent"`
	LogicalCores     int     `json:"logical_cores"`
	PhysicalCores    int     `json:"physical_cores"`
	ModelName        string  `json:"model_name"`
	Mhz              float64 `json:"mhz"`
	StealPercent     float64 `json:"steal_percent"`
	Load1            float64 `json:"load1"`
	Load5            float64 `json:"load5"`
	Load15           float64 `json:"load15"`
	Load1PerCore     float64 `json:"load1_per_core"`
	SchedulerDelayMS float64 `json:"scheduler_delay_ms"`
}

type MonitorMemoryResponse struct {
	UsedBytes    uint64                       `json:"used_bytes"`
	TotalBytes   uint64                       `json:"total_bytes"`
	UsagePercent float64                      `json:"usage_percent"`
	Process      MonitorProcessMemoryResponse `json:"process"`
}

type MonitorProcessMemoryResponse struct {
	RSSBytes       uint64 `json:"rss_bytes"`
	VMSBytes       uint64 `json:"vms_bytes"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	SysBytes       uint64 `json:"sys_bytes"`
	GCCount        uint32 `json:"gc_count"`
}

type MonitorDiskResponse struct {
	Path         string  `json:"path"`
	UsedBytes    uint64  `json:"used_bytes"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

type MonitorLatencyResponse struct {
	Enabled bool    `json:"enabled"`
	Target  string  `json:"target"`
	RTTMS   float64 `json:"rtt_ms"`
	Status  string  `json:"status"`
	Error   string  `json:"error"`
}

type SaveMonitorConfigRequest struct {
	SampleIntervalSeconds int                           `json:"sample_interval_seconds"`
	LatencyProbeEnabled   bool                          `json:"latency_probe_enabled"`
	LatencyProbeURL       string                        `json:"latency_probe_url"`
	LatencyProbeTimeoutMS int                           `json:"latency_probe_timeout_ms"`
	AlertBotChannelID     string                        `json:"alert_bot_channel_id"`
	DailyAlertLimit       int                           `json:"daily_alert_limit"`
	Rules                 []SaveMonitorAlertRuleRequest `json:"rules"`
}

type SaveMonitorAlertRuleRequest struct {
	MetricKey        string  `json:"metric_key"`
	Enabled          bool    `json:"enabled"`
	ThresholdValue   float64 `json:"threshold_value"`
	SustainedSamples int     `json:"sustained_samples"`
}

func GetMonitor(c echo.Context) error {
	monitor, err := usecase.GetMonitor(fwcontext.InternalUsecaseContext(c), usecase.MonitorQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toMonitorResponse(monitor))
}

func SaveMonitorConfig(c echo.Context) error {
	var req SaveMonitorConfigRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	monitor, err := usecase.SaveMonitorConfig(fwcontext.InternalUsecaseContext(c), usecase.SaveMonitorConfigCmd{
		SampleIntervalSeconds: req.SampleIntervalSeconds,
		LatencyProbeEnabled:   req.LatencyProbeEnabled,
		LatencyProbeURL:       req.LatencyProbeURL,
		LatencyProbeTimeoutMS: req.LatencyProbeTimeoutMS,
		AlertBotChannelID:     req.AlertBotChannelID,
		DailyAlertLimit:       req.DailyAlertLimit,
		Rules:                 toSaveMonitorAlertRuleCmds(req.Rules),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toMonitorResponse(monitor))
}

func toMonitorResponse(monitor usecase.MonitorCo) MonitorResponse {
	return MonitorResponse{
		Settings: toMonitorSettingsResponse(monitor.Settings),
		Rules:    toMonitorAlertRuleResponses(monitor.Rules),
		Nodes:    toMonitorNodeResponses(monitor.Nodes),
		Bots:     toMonitorAlertBotResponses(monitor.Bots),
	}
}

func toMonitorSettingsResponse(settings usecase.MonitorSettingsCo) MonitorSettingsResponse {
	return MonitorSettingsResponse{
		SampleIntervalSeconds: settings.SampleIntervalSeconds,
		MinSampleInterval:     settings.MinSampleInterval,
		LatencyProbeEnabled:   settings.LatencyProbeEnabled,
		LatencyProbeURL:       settings.LatencyProbeURL,
		LatencyProbeTimeoutMS: settings.LatencyProbeTimeoutMS,
		AlertBotChannelID:     settings.AlertBotChannelID,
		DailyAlertLimit:       settings.DailyAlertLimit,
		UpdatedAt:             settings.UpdatedAt,
	}
}

func toMonitorAlertRuleResponses(rules []usecase.MonitorAlertRuleCo) []MonitorAlertRuleResponse {
	responses := make([]MonitorAlertRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, MonitorAlertRuleResponse{
			MetricKey:        rule.MetricKey,
			Enabled:          rule.Enabled,
			ThresholdValue:   rule.ThresholdValue,
			SustainedSamples: rule.SustainedSamples,
			UpdatedAt:        rule.UpdatedAt,
		})
	}
	return responses
}

func toMonitorAlertBotResponses(bots []usecase.MonitorAlertBotCo) []MonitorAlertBotResponse {
	responses := make([]MonitorAlertBotResponse, 0, len(bots))
	for _, bot := range bots {
		responses = append(responses, MonitorAlertBotResponse{
			ID:           bot.ID,
			ChannelCode:  bot.ChannelCode,
			ProviderCode: bot.ProviderCode,
			AdapterKey:   bot.AdapterKey,
			Enabled:      bot.Enabled,
			IsPrimary:    bot.IsPrimary,
		})
	}
	return responses
}

func toMonitorNodeResponses(nodes []usecase.MonitorNodeCo) []MonitorNodeResponse {
	responses := make([]MonitorNodeResponse, 0, len(nodes))
	for _, node := range nodes {
		responses = append(responses, MonitorNodeResponse{
			NodeID:    node.NodeID,
			Hostname:  node.Hostname,
			SampledAt: node.SampledAt,
			Stale:     node.Stale,
			Latest:    toMonitorSampleResponse(node.Latest),
			History:   toMonitorSampleResponses(node.History),
		})
	}
	return responses
}

func toMonitorSampleResponses(samples []usecase.MonitorSampleCo) []MonitorSampleResponse {
	responses := make([]MonitorSampleResponse, 0, len(samples))
	for _, sample := range samples {
		responses = append(responses, toMonitorSampleResponse(sample))
	}
	return responses
}

func toMonitorSampleResponse(sample usecase.MonitorSampleCo) MonitorSampleResponse {
	return MonitorSampleResponse{
		SampledAt: sample.SampledAt,
		CPU: MonitorCPUResponse{
			UsagePercent:     sample.CPU.UsagePercent,
			LogicalCores:     sample.CPU.LogicalCores,
			PhysicalCores:    sample.CPU.PhysicalCores,
			ModelName:        sample.CPU.ModelName,
			Mhz:              sample.CPU.Mhz,
			StealPercent:     sample.CPU.StealPercent,
			Load1:            sample.CPU.Load1,
			Load5:            sample.CPU.Load5,
			Load15:           sample.CPU.Load15,
			Load1PerCore:     sample.CPU.Load1PerCore,
			SchedulerDelayMS: sample.CPU.SchedulerDelayMS,
		},
		Memory: MonitorMemoryResponse{
			UsedBytes:    sample.Memory.UsedBytes,
			TotalBytes:   sample.Memory.TotalBytes,
			UsagePercent: sample.Memory.UsagePercent,
			Process: MonitorProcessMemoryResponse{
				RSSBytes:       sample.Memory.Process.RSSBytes,
				VMSBytes:       sample.Memory.Process.VMSBytes,
				HeapAllocBytes: sample.Memory.Process.HeapAllocBytes,
				HeapSysBytes:   sample.Memory.Process.HeapSysBytes,
				SysBytes:       sample.Memory.Process.SysBytes,
				GCCount:        sample.Memory.Process.GCCount,
			},
		},
		Disk: MonitorDiskResponse{
			Path:         sample.Disk.Path,
			UsedBytes:    sample.Disk.UsedBytes,
			TotalBytes:   sample.Disk.TotalBytes,
			UsagePercent: sample.Disk.UsagePercent,
		},
		Latency: MonitorLatencyResponse{
			Enabled: sample.Latency.Enabled,
			Target:  sample.Latency.Target,
			RTTMS:   sample.Latency.RTTMS,
			Status:  sample.Latency.Status,
			Error:   sample.Latency.Error,
		},
	}
}

func toSaveMonitorAlertRuleCmds(rules []SaveMonitorAlertRuleRequest) []usecase.SaveMonitorAlertRuleCmd {
	cmds := make([]usecase.SaveMonitorAlertRuleCmd, 0, len(rules))
	for _, rule := range rules {
		cmds = append(cmds, usecase.SaveMonitorAlertRuleCmd{
			MetricKey:        rule.MetricKey,
			Enabled:          rule.Enabled,
			ThresholdValue:   rule.ThresholdValue,
			SustainedSamples: rule.SustainedSamples,
		})
	}
	return cmds
}
