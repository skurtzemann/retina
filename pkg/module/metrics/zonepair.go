// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	v1 "github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/zap"
)

const (
	ZoneForwardCountName = "adv_zone_forward_count_total"
	ZoneForwardCountDesc = "Number of forwarded packets between zones"

	ZoneLabelSrc = "src_zone"
	ZoneLabelDst = "dst_zone"
	ZoneLabelDir = "direction"
)

type ZonePairMetrics struct {
	baseMetricObject
	countMetric metricsinit.GaugeVec
}

func NewZonePairMetrics(l *log.ZapLogger) *ZonePairMetrics {
	return &ZonePairMetrics{
		baseMetricObject: baseMetricObject{
			l: l.Named("zonepair-metrics"),
		},
	}
}

func (z *ZonePairMetrics) Init(metricName string) {
	z.countMetric = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		ZoneForwardCountName,
		ZoneForwardCountDesc,
		ZoneLabelSrc,
		ZoneLabelDst,
		ZoneLabelDir,
	)
	z.l.Info("Zone pair metrics initialized")
}

func (z *ZonePairMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(z.countMetric))
}

func (z *ZonePairMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil {
		return
	}

	if flow.Verdict != v1.Verdict_FORWARDED {
		return
	}

	if flow.IP == nil {
		return
	}

	srcZone := "unknown"
	dstZone := "unknown"

	if cache.GlobalCache != nil {
		srcZone = cache.GlobalCache.GetZoneByPodIP(flow.IP.Source)
		dstZone = cache.GlobalCache.GetZoneByPodIP(flow.IP.Destination)
	}

	direction := flow.TrafficDirection.String()

	z.countMetric.WithLabelValues(srcZone, dstZone, direction).Add(1)
	z.l.Debug("Zone forward count metric updated",
		zap.String("src_zone", srcZone),
		zap.String("dst_zone", dstZone),
		zap.String("direction", direction),
	)
}
