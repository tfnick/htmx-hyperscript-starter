package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/externalnotification"
)

const (
	MonitorMetricCPU     = "cpu"
	MonitorMetricMemory  = "memory"
	MonitorMetricDisk    = "disk"
	MonitorMetricLatency = "latency"

	defaultMonitorSampleIntervalSeconds = 10
	minMonitorSampleIntervalSeconds     = 5
	maxMonitorSampleIntervalSeconds     = 3600
	defaultMonitorLatencyProbeURL       = "https://www.google.com"
	defaultMonitorLatencyTimeoutMS      = 2000
	minMonitorLatencyTimeoutMS          = 500
	maxMonitorLatencyTimeoutMS          = 10000
	defaultMonitorDailyAlertLimit       = 1
	monitorSampleHistoryLimit           = 20
)

type MonitorQry struct{}

type SaveMonitorConfigCmd struct {
	SampleIntervalSeconds int
	LatencyProbeEnabled   bool
	LatencyProbeURL       string
	LatencyProbeTimeoutMS int
	AlertBotChannelID     string
	DailyAlertLimit       int
	Rules                 []SaveMonitorAlertRuleCmd
}

type SaveMonitorAlertRuleCmd struct {
	MetricKey        string
	Enabled          bool
	ThresholdValue   float64
	SustainedSamples int
}

type MonitorCo struct {
	Settings MonitorSettingsCo    `json:"settings"`
	Rules    []MonitorAlertRuleCo `json:"rules"`
	Nodes    []MonitorNodeCo      `json:"nodes"`
	Bots     []MonitorAlertBotCo  `json:"bots"`
}

type MonitorSettingsCo struct {
	SampleIntervalSeconds int    `json:"sample_interval_seconds"`
	MinSampleInterval     int    `json:"min_sample_interval_seconds"`
	LatencyProbeEnabled   bool   `json:"latency_probe_enabled"`
	LatencyProbeURL       string `json:"latency_probe_url"`
	LatencyProbeTimeoutMS int    `json:"latency_probe_timeout_ms"`
	AlertBotChannelID     string `json:"alert_bot_channel_id"`
	DailyAlertLimit       int    `json:"daily_alert_limit"`
	UpdatedAt             string `json:"updated_at"`
}

type MonitorAlertRuleCo struct {
	MetricKey        string  `json:"metric_key"`
	Enabled          bool    `json:"enabled"`
	ThresholdValue   float64 `json:"threshold_value"`
	SustainedSamples int     `json:"sustained_samples"`
	UpdatedAt        string  `json:"updated_at"`
}

type MonitorAlertBotCo struct {
	ID           string `json:"id"`
	ChannelCode  string `json:"channel_code"`
	ProviderCode string `json:"provider_code"`
	AdapterKey   string `json:"adapter_key"`
	Enabled      bool   `json:"enabled"`
	IsPrimary    bool   `json:"is_primary"`
}

type MonitorNodeCo struct {
	NodeID    string            `json:"node_id"`
	Hostname  string            `json:"hostname"`
	SampledAt string            `json:"sampled_at"`
	Stale     bool              `json:"stale"`
	Latest    MonitorSampleCo   `json:"latest"`
	History   []MonitorSampleCo `json:"history"`
}

type MonitorSampleCo struct {
	SampledAt string           `json:"sampled_at"`
	CPU       MonitorCPUCo     `json:"cpu"`
	Memory    MonitorMemoryCo  `json:"memory"`
	Disk      MonitorDiskCo    `json:"disk"`
	Latency   MonitorLatencyCo `json:"latency"`
}

type MonitorCPUCo struct {
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

type MonitorMemoryCo struct {
	UsedBytes    uint64                 `json:"used_bytes"`
	TotalBytes   uint64                 `json:"total_bytes"`
	UsagePercent float64                `json:"usage_percent"`
	Process      MonitorProcessMemoryCo `json:"process"`
}

type MonitorProcessMemoryCo struct {
	RSSBytes       uint64 `json:"rss_bytes"`
	VMSBytes       uint64 `json:"vms_bytes"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	SysBytes       uint64 `json:"sys_bytes"`
	GCCount        uint32 `json:"gc_count"`
}

type MonitorDiskCo struct {
	Path         string  `json:"path"`
	UsedBytes    uint64  `json:"used_bytes"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

type MonitorLatencyCo struct {
	Enabled bool    `json:"enabled"`
	Target  string  `json:"target"`
	RTTMS   float64 `json:"rtt_ms"`
	Status  string  `json:"status"`
	Error   string  `json:"error"`
}

type monitorSample struct {
	SampledAt time.Time
	CPU       MonitorCPUCo
	Memory    MonitorMemoryCo
	Disk      MonitorDiskCo
	Latency   MonitorLatencyCo
}

type monitorCollector interface {
	Collect(context.Context, monitorCollectorConfig) (monitorSample, error)
}

type monitorCollectorConfig struct {
	LatencyProbeEnabled bool
	LatencyProbeURL     string
	LatencyTimeout      time.Duration
}

type monitorSampler struct {
	mu           sync.RWMutex
	nodeID       string
	hostname     string
	collector    monitorCollector
	samples      []monitorSample
	alertCounts  map[string]int
	lastInterval time.Duration
}

var defaultMonitorSampler = newMonitorSampler(systemMonitorCollector{})

func newMonitorSampler(collector monitorCollector) *monitorSampler {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "local"
	}
	nodeID := strings.TrimSpace(os.Getenv("MONITOR_NODE_ID"))
	if nodeID == "" {
		nodeID = hostname
	}
	return &monitorSampler{
		nodeID:      nodeID,
		hostname:    hostname,
		collector:   collector,
		samples:     make([]monitorSample, 0, monitorSampleHistoryLimit),
		alertCounts: map[string]int{},
	}
}

func GetMonitor(ctx fwusecase.Context, _ MonitorQry) (MonitorCo, error) {
	settings, rules, err := loadMonitorConfig(ctx)
	if err != nil {
		return MonitorCo{}, err
	}
	bots, err := listMonitorAlertBots(ctx)
	if err != nil {
		return MonitorCo{}, err
	}
	return MonitorCo{
		Settings: monitorSettingsCoFromModel(settings),
		Rules:    monitorRuleCosFromModels(rules),
		Nodes:    defaultMonitorSampler.nodes(settings),
		Bots:     bots,
	}, nil
}

func SaveMonitorConfig(ctx fwusecase.Context, cmd SaveMonitorConfigCmd) (MonitorCo, error) {
	settingsInput, err := normalizeMonitorSettingsInput(cmd)
	if err != nil {
		return MonitorCo{}, err
	}
	rulesInput, err := normalizeMonitorRuleInputs(cmd.Rules)
	if err != nil {
		return MonitorCo{}, err
	}
	if settingsInput.AlertBotChannelID != "" {
		if _, err := models.GetIntegrationChannelConfigByID(ctx.Std(), settingsInput.AlertBotChannelID); err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return MonitorCo{}, fwusecase.E(fwusecase.CodeValidation, "alert bot is not configured", err)
			}
			return MonitorCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load alert bot", err)
		}
	}

	if err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if _, err := models.SaveMonitorSettings(txCtx.Std(), settingsInput); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to save monitor settings", err)
		}
		if _, err := models.SaveMonitorAlertRules(txCtx.Std(), rulesInput); err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeValidation, "monitor alert rule is invalid", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to save monitor alert rules", err)
		}
		return nil
	}); err != nil {
		return MonitorCo{}, err
	}
	return GetMonitor(ctx, MonitorQry{})
}

func StartMonitorSampler(ctx context.Context) {
	go defaultMonitorSampler.run(ctx)
}

func (s *monitorSampler) run(ctx context.Context) {
	s.sampleOnce(ctx)
	for {
		settings, _, err := loadMonitorConfig(fwusecase.NewContext(ctx, fwusecase.SurfaceSystem))
		interval := time.Duration(defaultMonitorSampleIntervalSeconds) * time.Second
		if err == nil {
			interval = time.Duration(normalizedMonitorSettings(settings).SampleIntervalSeconds) * time.Second
		}
		if interval < time.Duration(minMonitorSampleIntervalSeconds)*time.Second {
			interval = time.Duration(minMonitorSampleIntervalSeconds) * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.sampleOnce(ctx)
		}
	}
}

func (s *monitorSampler) sampleOnce(ctx context.Context) {
	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceSystem)
	settings, rules, err := loadMonitorConfig(ucCtx)
	if err != nil {
		logger := logging.For("monitor")
		logger.Warn().Err(err).Msg("failed to load monitor config")
		return
	}
	settings = normalizedMonitorSettings(settings)
	sample, err := s.collector.Collect(ctx, monitorCollectorConfig{
		LatencyProbeEnabled: settings.LatencyProbeEnabled == 1,
		LatencyProbeURL:     settings.LatencyProbeURL,
		LatencyTimeout:      time.Duration(settings.LatencyProbeTimeoutMS) * time.Millisecond,
	})
	if err != nil {
		logger := logging.For("monitor")
		logger.Warn().Err(err).Msg("failed to collect monitor sample")
		return
	}
	if sample.SampledAt.IsZero() {
		sample.SampledAt = timefmt.NowUTC()
	}
	s.addSample(sample)
	s.evaluateAlerts(ctx, settings, rules)
}

func (s *monitorSampler) addSample(sample monitorSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, sample)
	if len(s.samples) > monitorSampleHistoryLimit {
		s.samples = append([]monitorSample(nil), s.samples[len(s.samples)-monitorSampleHistoryLimit:]...)
	}
}

func (s *monitorSampler) nodes(settings models.MonitorSettings) []MonitorNodeCo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	history := make([]MonitorSampleCo, 0, len(s.samples))
	for _, sample := range s.samples {
		history = append(history, monitorSampleCo(sample))
	}
	latest := MonitorSampleCo{}
	sampledAt := ""
	stale := true
	if len(s.samples) > 0 {
		last := s.samples[len(s.samples)-1]
		latest = monitorSampleCo(last)
		sampledAt = timefmt.RFC3339(last.SampledAt)
		maxAge := time.Duration(normalizedMonitorSettings(settings).SampleIntervalSeconds*3) * time.Second
		stale = time.Since(last.SampledAt) > maxAge
	}
	return []MonitorNodeCo{{
		NodeID:    s.nodeID,
		Hostname:  s.hostname,
		SampledAt: sampledAt,
		Stale:     stale,
		Latest:    latest,
		History:   history,
	}}
}

func (s *monitorSampler) evaluateAlerts(ctx context.Context, settings models.MonitorSettings, rules []models.MonitorAlertRule) {
	settings = normalizedMonitorSettings(settings)
	if settings.DailyAlertLimit <= 0 || strings.TrimSpace(settings.AlertBotChannelID) == "" {
		return
	}
	s.mu.RLock()
	samples := append([]monitorSample(nil), s.samples...)
	s.mu.RUnlock()
	if len(samples) == 0 || !monitorRulesTriggered(samples, rules) {
		return
	}
	dateKey := timefmt.NowUTC().Format("2006-01-02")
	alertKey := s.nodeID + ":" + dateKey
	s.mu.Lock()
	if s.alertCounts[alertKey] >= settings.DailyAlertLimit {
		s.mu.Unlock()
		return
	}
	s.alertCounts[alertKey]++
	s.mu.Unlock()
	if err := sendMonitorAlert(ctx, settings.AlertBotChannelID, s.nodeID, s.hostname, samples[len(samples)-1], rules); err != nil {
		logger := logging.For("monitor")
		logger.Warn().Err(err).Msg("failed to send monitor alert")
	}
}

type systemMonitorCollector struct{}

func (systemMonitorCollector) Collect(ctx context.Context, cfg monitorCollectorConfig) (monitorSample, error) {
	cpuStats := collectMonitorCPUStats(ctx)
	timesBefore, _ := cpu.TimesWithContext(ctx, false)
	startedCPUPercent := time.Now()
	cpuPercent, err := cpu.PercentWithContext(ctx, 100*time.Millisecond, false)
	cpuStats.SchedulerDelayMS = monitorSchedulerDelayMS(startedCPUPercent, 100*time.Millisecond)
	if err != nil {
		return monitorSample{}, err
	}
	if timesAfter, err := cpu.TimesWithContext(ctx, false); err == nil {
		cpuStats.StealPercent = monitorStealPercent(timesBefore, timesAfter)
	}
	memStats, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return monitorSample{}, err
	}
	processMemory := collectMonitorProcessMemory(ctx)
	path := "."
	if runtime.GOOS != "windows" {
		path = "/"
	}
	diskStats, err := disk.UsageWithContext(ctx, path)
	if err != nil {
		return monitorSample{}, err
	}
	latency := probeMonitorLatency(ctx, cfg)
	usage := 0.0
	if len(cpuPercent) > 0 {
		usage = cpuPercent[0]
	}
	cpuStats.UsagePercent = usage
	return monitorSample{
		SampledAt: timefmt.NowUTC(),
		CPU:       cpuStats,
		Memory: MonitorMemoryCo{
			UsedBytes:    memStats.Used,
			TotalBytes:   memStats.Total,
			UsagePercent: memStats.UsedPercent,
			Process:      processMemory,
		},
		Disk: MonitorDiskCo{
			Path:         path,
			UsedBytes:    diskStats.Used,
			TotalBytes:   diskStats.Total,
			UsagePercent: diskStats.UsedPercent,
		},
		Latency: latency,
	}, nil
}

func collectMonitorProcessMemory(ctx context.Context) MonitorProcessMemoryCo {
	var runtimeStats runtime.MemStats
	runtime.ReadMemStats(&runtimeStats)
	result := MonitorProcessMemoryCo{
		HeapAllocBytes: runtimeStats.HeapAlloc,
		HeapSysBytes:   runtimeStats.HeapSys,
		SysBytes:       runtimeStats.Sys,
		GCCount:        runtimeStats.NumGC,
	}
	proc, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		return result
	}
	info, err := proc.MemoryInfoWithContext(ctx)
	if err != nil || info == nil {
		return result
	}
	result.RSSBytes = info.RSS
	result.VMSBytes = info.VMS
	return result
}

func collectMonitorCPUStats(ctx context.Context) MonitorCPUCo {
	stats := MonitorCPUCo{}
	if logical, err := cpu.CountsWithContext(ctx, true); err == nil && logical > 0 {
		stats.LogicalCores = logical
	}
	if physical, err := cpu.CountsWithContext(ctx, false); err == nil && physical > 0 {
		stats.PhysicalCores = physical
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil {
		for _, info := range infos {
			if stats.ModelName == "" {
				stats.ModelName = strings.TrimSpace(info.ModelName)
			}
			if stats.Mhz <= 0 && info.Mhz > 0 {
				stats.Mhz = info.Mhz
			}
			if stats.ModelName != "" && stats.Mhz > 0 {
				break
			}
		}
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		stats.Load1 = avg.Load1
		stats.Load5 = avg.Load5
		stats.Load15 = avg.Load15
		stats.Load1PerCore = monitorLoadPerCore(avg.Load1, stats.LogicalCores)
	}
	return stats
}

func monitorStealPercent(before []cpu.TimesStat, after []cpu.TimesStat) float64 {
	if len(before) == 0 || len(after) == 0 {
		return 0
	}
	beforeTotal := monitorCPUTotalTime(before[0])
	afterTotal := monitorCPUTotalTime(after[0])
	totalDelta := afterTotal - beforeTotal
	stealDelta := after[0].Steal - before[0].Steal
	if totalDelta <= 0 || stealDelta <= 0 {
		return 0
	}
	return stealDelta / totalDelta * 100
}

func monitorCPUTotalTime(value cpu.TimesStat) float64 {
	return value.User + value.System + value.Idle + value.Nice + value.Iowait + value.Irq + value.Softirq + value.Steal + value.Guest + value.GuestNice
}

func monitorLoadPerCore(loadValue float64, logicalCores int) float64 {
	if loadValue <= 0 || logicalCores <= 0 {
		return 0
	}
	return loadValue / float64(logicalCores)
}

func monitorSchedulerDelayMS(started time.Time, expected time.Duration) float64 {
	delay := time.Since(started) - expected
	if delay <= 0 {
		return 0
	}
	return float64(delay.Microseconds()) / 1000
}

func probeMonitorLatency(ctx context.Context, cfg monitorCollectorConfig) MonitorLatencyCo {
	target := strings.TrimSpace(cfg.LatencyProbeURL)
	if !cfg.LatencyProbeEnabled || target == "" {
		return MonitorLatencyCo{Enabled: false, Target: target, Status: "disabled"}
	}
	timeout := cfg.LatencyTimeout
	if timeout <= 0 {
		timeout = time.Duration(defaultMonitorLatencyTimeoutMS) * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, target, nil)
	if err != nil {
		return MonitorLatencyCo{Enabled: true, Target: target, Status: "error", Error: "invalid probe URL"}
	}
	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	rtt := float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		return MonitorLatencyCo{Enabled: true, Target: target, RTTMS: rtt, Status: "error", Error: "probe failed"}
	}
	defer resp.Body.Close()
	status := "ok"
	if resp.StatusCode >= 400 {
		status = "degraded"
	}
	return MonitorLatencyCo{Enabled: true, Target: target, RTTMS: rtt, Status: status}
}

func loadMonitorConfig(ctx fwusecase.Context) (models.MonitorSettings, []models.MonitorAlertRule, error) {
	settings, err := models.GetMonitorSettings(ctx.Std())
	if err != nil {
		return models.MonitorSettings{}, nil, fwusecase.E(fwusecase.CodeInternal, "failed to load monitor settings", err)
	}
	rules, err := models.ListMonitorAlertRules(ctx.Std())
	if err != nil {
		return models.MonitorSettings{}, nil, fwusecase.E(fwusecase.CodeInternal, "failed to load monitor alert rules", err)
	}
	return normalizedMonitorSettings(settings), rules, nil
}

func normalizedMonitorSettings(settings models.MonitorSettings) models.MonitorSettings {
	if settings.ID == "" {
		settings.ID = models.MonitorSettingsIDDefault
	}
	if settings.SampleIntervalSeconds <= 0 {
		settings.SampleIntervalSeconds = defaultMonitorSampleIntervalSeconds
	}
	if settings.SampleIntervalSeconds < minMonitorSampleIntervalSeconds {
		settings.SampleIntervalSeconds = minMonitorSampleIntervalSeconds
	}
	if settings.SampleIntervalSeconds > maxMonitorSampleIntervalSeconds {
		settings.SampleIntervalSeconds = maxMonitorSampleIntervalSeconds
	}
	settings.LatencyProbeURL = strings.TrimSpace(settings.LatencyProbeURL)
	if settings.LatencyProbeURL == "" {
		settings.LatencyProbeURL = defaultMonitorLatencyProbeURL
	}
	if settings.LatencyProbeTimeoutMS <= 0 {
		settings.LatencyProbeTimeoutMS = defaultMonitorLatencyTimeoutMS
	}
	if settings.LatencyProbeTimeoutMS < minMonitorLatencyTimeoutMS {
		settings.LatencyProbeTimeoutMS = minMonitorLatencyTimeoutMS
	}
	if settings.LatencyProbeTimeoutMS > maxMonitorLatencyTimeoutMS {
		settings.LatencyProbeTimeoutMS = maxMonitorLatencyTimeoutMS
	}
	if settings.DailyAlertLimit <= 0 {
		settings.DailyAlertLimit = defaultMonitorDailyAlertLimit
	}
	settings.AlertBotChannelID = strings.TrimSpace(settings.AlertBotChannelID)
	return settings
}

func normalizeMonitorSettingsInput(cmd SaveMonitorConfigCmd) (models.SaveMonitorSettingsCmd, error) {
	settings := models.MonitorSettings{
		SampleIntervalSeconds: cmd.SampleIntervalSeconds,
		LatencyProbeEnabled:   boolToInt(cmd.LatencyProbeEnabled),
		LatencyProbeURL:       strings.TrimSpace(cmd.LatencyProbeURL),
		LatencyProbeTimeoutMS: cmd.LatencyProbeTimeoutMS,
		AlertBotChannelID:     strings.TrimSpace(cmd.AlertBotChannelID),
		DailyAlertLimit:       cmd.DailyAlertLimit,
	}
	if settings.SampleIntervalSeconds < minMonitorSampleIntervalSeconds || settings.SampleIntervalSeconds > maxMonitorSampleIntervalSeconds {
		return models.SaveMonitorSettingsCmd{}, fwusecase.E(
			fwusecase.CodeValidation,
			fmt.Sprintf("sample interval must be between %d and %d seconds", minMonitorSampleIntervalSeconds, maxMonitorSampleIntervalSeconds),
			nil,
		)
	}
	if settings.LatencyProbeEnabled == 1 {
		if _, err := url.ParseRequestURI(settings.LatencyProbeURL); err != nil {
			return models.SaveMonitorSettingsCmd{}, fwusecase.E(fwusecase.CodeValidation, "latency probe URL is invalid", err)
		}
	}
	if settings.LatencyProbeTimeoutMS < minMonitorLatencyTimeoutMS || settings.LatencyProbeTimeoutMS > maxMonitorLatencyTimeoutMS {
		return models.SaveMonitorSettingsCmd{}, fwusecase.E(
			fwusecase.CodeValidation,
			fmt.Sprintf("latency probe timeout must be between %d and %d ms", minMonitorLatencyTimeoutMS, maxMonitorLatencyTimeoutMS),
			nil,
		)
	}
	if settings.DailyAlertLimit <= 0 {
		return models.SaveMonitorSettingsCmd{}, fwusecase.E(fwusecase.CodeValidation, "daily alert limit must be greater than 0", nil)
	}
	settings = normalizedMonitorSettings(settings)
	return models.SaveMonitorSettingsCmd{
		SampleIntervalSeconds: settings.SampleIntervalSeconds,
		LatencyProbeEnabled:   settings.LatencyProbeEnabled == 1,
		LatencyProbeURL:       settings.LatencyProbeURL,
		LatencyProbeTimeoutMS: settings.LatencyProbeTimeoutMS,
		AlertBotChannelID:     settings.AlertBotChannelID,
		DailyAlertLimit:       settings.DailyAlertLimit,
	}, nil
}

func normalizeMonitorRuleInputs(rules []SaveMonitorAlertRuleCmd) ([]models.SaveMonitorAlertRuleCmd, error) {
	byMetric := map[string]SaveMonitorAlertRuleCmd{}
	for _, rule := range rules {
		key := strings.TrimSpace(strings.ToLower(rule.MetricKey))
		if !validMonitorMetricKey(key) {
			return nil, fwusecase.E(fwusecase.CodeValidation, "monitor alert rule metric is invalid", nil)
		}
		rule.MetricKey = key
		byMetric[key] = rule
	}
	result := make([]models.SaveMonitorAlertRuleCmd, 0, 4)
	for _, key := range monitorMetricKeys() {
		rule, ok := byMetric[key]
		if !ok {
			rule = SaveMonitorAlertRuleCmd{MetricKey: key, Enabled: false, ThresholdValue: defaultMonitorThreshold(key), SustainedSamples: 3}
		}
		if rule.ThresholdValue < 0 {
			return nil, fwusecase.E(fwusecase.CodeValidation, "monitor alert threshold must be non-negative", nil)
		}
		if rule.SustainedSamples < 1 || rule.SustainedSamples > monitorSampleHistoryLimit {
			return nil, fwusecase.E(fwusecase.CodeValidation, "monitor alert sustained samples is invalid", nil)
		}
		result = append(result, models.SaveMonitorAlertRuleCmd{
			MetricKey:        key,
			Enabled:          rule.Enabled,
			ThresholdValue:   rule.ThresholdValue,
			SustainedSamples: rule.SustainedSamples,
		})
	}
	return result, nil
}

func monitorMetricKeys() []string {
	return []string{MonitorMetricCPU, MonitorMetricMemory, MonitorMetricDisk, MonitorMetricLatency}
}

func validMonitorMetricKey(key string) bool {
	for _, item := range monitorMetricKeys() {
		if key == item {
			return true
		}
	}
	return false
}

func defaultMonitorThreshold(key string) float64 {
	if key == MonitorMetricLatency {
		return 1000
	}
	return 90
}

func monitorSettingsCoFromModel(settings models.MonitorSettings) MonitorSettingsCo {
	settings = normalizedMonitorSettings(settings)
	return MonitorSettingsCo{
		SampleIntervalSeconds: settings.SampleIntervalSeconds,
		MinSampleInterval:     minMonitorSampleIntervalSeconds,
		LatencyProbeEnabled:   settings.LatencyProbeEnabled == 1,
		LatencyProbeURL:       settings.LatencyProbeURL,
		LatencyProbeTimeoutMS: settings.LatencyProbeTimeoutMS,
		AlertBotChannelID:     settings.AlertBotChannelID,
		DailyAlertLimit:       settings.DailyAlertLimit,
		UpdatedAt:             settings.UpdatedAt,
	}
}

func monitorRuleCosFromModels(rules []models.MonitorAlertRule) []MonitorAlertRuleCo {
	result := make([]MonitorAlertRuleCo, 0, len(rules))
	for _, rule := range rules {
		result = append(result, MonitorAlertRuleCo{
			MetricKey:        rule.MetricKey,
			Enabled:          rule.Enabled == 1,
			ThresholdValue:   rule.ThresholdValue,
			SustainedSamples: rule.SustainedSamples,
			UpdatedAt:        rule.UpdatedAt,
		})
	}
	return result
}

func monitorSampleCo(sample monitorSample) MonitorSampleCo {
	return MonitorSampleCo{
		SampledAt: timefmt.RFC3339(sample.SampledAt),
		CPU:       sample.CPU,
		Memory:    sample.Memory,
		Disk:      sample.Disk,
		Latency:   sample.Latency,
	}
}

func monitorRulesTriggered(samples []monitorSample, rules []models.MonitorAlertRule) bool {
	for _, rule := range rules {
		if rule.Enabled != 1 {
			continue
		}
		count := rule.SustainedSamples
		if count < 1 {
			count = 1
		}
		if len(samples) < count {
			continue
		}
		triggered := true
		for _, sample := range samples[len(samples)-count:] {
			if monitorMetricValue(sample, rule.MetricKey) < rule.ThresholdValue {
				triggered = false
				break
			}
		}
		if triggered {
			return true
		}
	}
	return false
}

func monitorMetricValue(sample monitorSample, metricKey string) float64 {
	switch metricKey {
	case MonitorMetricCPU:
		return sample.CPU.UsagePercent
	case MonitorMetricMemory:
		return sample.Memory.UsagePercent
	case MonitorMetricDisk:
		return sample.Disk.UsagePercent
	case MonitorMetricLatency:
		if !sample.Latency.Enabled || sample.Latency.Status == "error" {
			return 0
		}
		return sample.Latency.RTTMS
	default:
		return 0
	}
}

func listMonitorAlertBots(ctx fwusecase.Context) ([]MonitorAlertBotCo, error) {
	channels, err := models.ListIntegrationChannelConfigs(ctx.Std(), models.IntegrationScenarioExternalNotification)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load alert bots", err)
	}
	result := make([]MonitorAlertBotCo, 0, len(channels))
	for _, channel := range channels {
		result = append(result, MonitorAlertBotCo{
			ID:           channel.ID,
			ChannelCode:  channel.ChannelCode,
			ProviderCode: channel.ProviderCode,
			AdapterKey:   channel.AdapterKey,
			Enabled:      channel.Enabled == 1,
			IsPrimary:    channel.IsPrimary == 1,
		})
	}
	return result, nil
}

func sendMonitorAlert(ctx context.Context, channelID string, nodeID string, hostname string, sample monitorSample, rules []models.MonitorAlertRule) error {
	channel, err := models.GetIntegrationChannelConfigByID(ctx, channelID)
	if err != nil {
		return err
	}
	if channel.Enabled != 1 || channel.Scenario != models.IntegrationScenarioExternalNotification {
		return fmt.Errorf("monitor alert bot channel is not enabled")
	}
	adapter, ok := RegisteredExternalNotificationAdapter(channel.AdapterKey)
	if !ok {
		return fmt.Errorf("external notification adapter not registered: %s", channel.AdapterKey)
	}
	fields := []externalnotification.MessageField{
		{Label: "Node", Value: nodeID},
		{Label: "Host", Value: hostname},
		{Label: "CPU", Value: fmt.Sprintf("%.1f%%", sample.CPU.UsagePercent)},
		{Label: "Memory", Value: fmt.Sprintf("%.1f%%", sample.Memory.UsagePercent)},
		{Label: "Disk", Value: fmt.Sprintf("%.1f%%", sample.Disk.UsagePercent)},
		{Label: "Latency", Value: fmt.Sprintf("%.1f ms %s", sample.Latency.RTTMS, sample.Latency.Status)},
		{Label: "Sampled At", Value: timefmt.RFC3339(sample.SampledAt)},
	}
	for _, rule := range rules {
		if rule.Enabled == 1 && monitorMetricValue(sample, rule.MetricKey) >= rule.ThresholdValue {
			fields = append(fields, externalnotification.MessageField{
				Label: strings.ToUpper(rule.MetricKey) + " Rule",
				Value: fmt.Sprintf(">= %.1f", rule.ThresholdValue),
			})
		}
	}
	_, err = adapter.Send(ctx, externalnotification.ProviderConfig{
		ProviderCode: channel.ProviderCode,
		AdapterKey:   channel.AdapterKey,
		Credential:   channel.CredentialValue,
		ConfigJSON:   channel.ConfigJSON,
	}, externalnotification.SendRequest{
		Title:      "Monitor alert",
		Summary:    "Server monitor rule triggered",
		EventTopic: "monitor.alert",
		Fields:     fields,
	})
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
