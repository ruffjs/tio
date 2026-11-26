<template>
  <div class="broker-view">
    <header class="workspace-toolbar">
      <div class="toolbar-copy">
        <h1>{{ $t('broker.title') }}</h1>
        <p>{{ $t('broker.subtitle') }}</p>
      </div>
      <div class="toolbar-actions">
        <span class="refresh-label">{{ $t('broker.autoRefresh') }}</span>
        <el-select v-model="refreshSeconds" size="small" class="refresh-select">
          <el-option :value="0" :label="$t('broker.refreshOff')" />
          <el-option
            v-for="n in [15, 30, 60]"
            :key="n"
            :value="n"
            :label="$t('broker.seconds', { n })"
          />
        </el-select>
        <el-button :loading="loadingOverview" icon="Refresh" @click="refreshAll()">
          {{ $t('common.refresh') }}
        </el-button>
      </div>
    </header>

    <section class="overview-grid">
      <div class="overview-card">
        <div class="card-label">{{ $t('broker.node') }}</div>
        <div class="card-value">{{ overview?.node || '-' }}</div>
        <div class="card-note">NATS {{ overview?.version || '-' }}</div>
      </div>
      <div class="overview-card">
        <div class="card-label">{{ $t('broker.sampledAt') }}</div>
        <div class="card-value">{{ fmt(overview?.sampledAt) }}</div>
        <div class="card-note">{{ $t('broker.lastHour') }}</div>
      </div>
      <div class="overview-card">
        <div class="card-label">{{ $t('broker.onlineConnections') }}</div>
        <div class="card-value">{{ num(overview?.connections) }}</div>
        <div class="card-note">{{ $t('broker.onlineConnectionsNote') }}</div>
      </div>
      <div class="overview-card">
        <div class="card-label">{{ $t('broker.consumer') }}</div>
        <div class="card-value">{{ consumerText }}</div>
        <div class="card-note">{{ overview?.consumer?.stream || '$MQTT_msgs' }}</div>
      </div>
    </section>

    <section class="charts">
      <div class="chart-card">
        <div class="chart-head">
          <span>{{ $t('broker.msgRate') }}</span>
          <span class="chart-unit">msg/s</span>
        </div>
        <div class="chart-body">
          <div class="axis-y">
            <span>{{ msgAxis.max }}</span>
            <span>{{ msgAxis.mid }}</span>
            <span>0</span>
          </div>
          <div
            class="chart-wrap"
            @mousemove="onChartMove($event, 'msgRate')"
            @mouseleave="hoverState.msgRate = null"
          >
            <svg class="chart" viewBox="0 0 600 100" preserveAspectRatio="none" role="img">
              <line v-for="y in chartGrid" :key="y" x1="0" :y1="y" x2="600" :y2="y" class="grid-line" />
              <polyline
                v-for="(seg, i) in msgChart.segments"
                :key="i"
                :points="seg"
                class="chart-line"
              />
              <circle
                v-for="(pt, i) in msgChart.singlePoints"
                :key="'p' + i"
                :cx="pt[0]"
                :cy="pt[1]"
                r="1.6"
                class="chart-dot"
              />
              <line
                v-if="hoverState.msgRate"
                :x1="hoverState.msgRate.x"
                y1="4"
                :x2="hoverState.msgRate.x"
                y2="96"
                class="crosshair"
              />
              <circle
                v-if="hoverState.msgRate && hoverState.msgRate.y !== null"
                :cx="hoverState.msgRate.x"
                :cy="hoverState.msgRate.y"
                r="2.5"
                class="chart-dot"
              />
            </svg>
            <div class="axis-x"><span>{{ rangeStart }}</span><span>{{ rangeEnd }}</span></div>
            <div
              v-if="hoverState.msgRate"
              class="chart-tip"
              :style="tipStyle(hoverState.msgRate.xPct)"
            >
              <div class="tip-time">{{ hoverState.msgRate.time }}</div>
              <div>{{ hoverState.msgRate.value ?? '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="chart-card">
        <div class="chart-head">
          <span>{{ $t('broker.byteRate') }}</span>
          <span class="chart-unit">{{ byteUnit.label }}</span>
        </div>
        <div class="chart-body">
          <div class="axis-y">
            <span>{{ byteAxis.max }}</span>
            <span>{{ byteAxis.mid }}</span>
            <span>0</span>
          </div>
          <div
            class="chart-wrap"
            @mousemove="onChartMove($event, 'byteRate')"
            @mouseleave="hoverState.byteRate = null"
          >
            <svg class="chart" viewBox="0 0 600 100" preserveAspectRatio="none" role="img">
              <line v-for="y in chartGrid" :key="y" x1="0" :y1="y" x2="600" :y2="y" class="grid-line" />
              <polyline
                v-for="(seg, i) in byteChart.segments"
                :key="i"
                :points="seg"
                class="chart-line byte-line"
              />
              <circle
                v-for="(pt, i) in byteChart.singlePoints"
                :key="'b' + i"
                :cx="pt[0]"
                :cy="pt[1]"
                r="1.6"
                class="chart-dot byte-dot"
              />
              <line
                v-if="hoverState.byteRate"
                :x1="hoverState.byteRate.x"
                y1="4"
                :x2="hoverState.byteRate.x"
                y2="96"
                class="crosshair"
              />
              <circle
                v-if="hoverState.byteRate && hoverState.byteRate.y !== null"
                :cx="hoverState.byteRate.x"
                :cy="hoverState.byteRate.y"
                r="2.5"
                class="chart-dot byte-dot"
              />
            </svg>
            <div class="axis-x"><span>{{ rangeStart }}</span><span>{{ rangeEnd }}</span></div>
            <div
              v-if="hoverState.byteRate"
              class="chart-tip"
              :style="tipStyle(hoverState.byteRate.xPct)"
            >
              <div class="tip-time">{{ hoverState.byteRate.time }}</div>
              <div>{{ hoverState.byteRate.value ?? '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <p class="chart-note">
        {{ $t('broker.chartScope') }} · {{ $t('broker.gapNote') }}
      </p>
    </section>

    <section class="table-shell">
      <div class="table-toolbar">
        <h2>{{ $t('broker.connections') }}</h2>
        <div class="filters">
          <el-input
            v-model="filters.clientId"
            :placeholder="$t('broker.searchClientId')"
            clearable
            class="filter-client"
            @keyup.enter="search"
            @clear="search"
          />
          <el-select v-model="filters.sort" class="filter-sort" @change="search">
            <el-option
              v-for="opt in sortOptions"
              :key="opt.value"
              :label="$t(opt.label)"
              :value="opt.value"
            />
          </el-select>
          <el-button type="primary" icon="Search" @click="search">
            {{ $t('common.search') }}
          </el-button>
        </div>
      </div>

      <el-table
        v-loading="loadingConns"
        :data="connections.items"
        class="clean-table"
        :scrollbar-always-on="false"
        @row-click="openDetail"
      >
        <el-table-column :label="$t('broker.type')" width="80">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="row.type === 'mqtt' ? 'success' : 'info'">
              {{ row.type || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('broker.clientId')" min-width="180">
          <template #default="{ row }">{{ row.clientId || row.name || '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('broker.address')" min-width="170">
          <template #default="{ row }">{{ fmtAddr(row.ip, row.port) }}</template>
        </el-table-column>
        <el-table-column :label="$t('broker.connectionAndLast')" min-width="180">
          <template #default="{ row }">
            <div class="time-stack">
              <el-tooltip
                :content="$t('broker.connectedAtAt', { time: fmtFull(row.start) })"
                placement="top"
              >
                <span class="time-row">
                  <span class="time-label">{{ $t('broker.connectedShort') }}</span>
                  <span class="time-value muted">{{ relativeTime(row.start) }}</span>
                </span>
              </el-tooltip>
              <el-tooltip
                :content="$t('broker.lastActivityAt', { time: fmtFull(row.lastActivity) })"
                placement="top"
              >
                <span class="time-row">
                  <span class="time-label">{{ $t('broker.activityShort') }}</span>
                  <span class="time-value">{{ relativeTime(row.lastActivity) }}</span>
                </span>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
        <el-table-column min-width="100" align="right">
          <template #header>
            <el-tooltip :content="$t('broker.inMsgsTip')" placement="top">
              <span class="header-tip">{{ $t('broker.uplink') }}</span>
            </el-tooltip>
          </template>
          <template #default="{ row }">
            <div class="cell-stack">
              <span>{{ num(row.inMsgs) }} msg</span>
              <span class="cell-sub">{{ fmtBytes(row.inBytes) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column min-width="100" align="right">
          <template #header>
            <el-tooltip :content="$t('broker.outMsgsTip')" placement="top">
              <span class="header-tip">{{ $t('broker.downlink') }}</span>
            </el-tooltip>
          </template>
          <template #default="{ row }">
            <div class="cell-stack">
              <span>{{ num(row.outMsgs) }} msg</span>
              <span class="cell-sub">{{ fmtBytes(row.outBytes) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          prop="subscriptions"
          :label="$t('broker.subscriptions')"
          min-width="90"
          align="right"
        />
      </el-table>

      <div class="pager">
        <el-pagination
          v-model:current-page="filters.page"
          v-model:page-size="filters.pageSize"
          :total="connections.total"
          :page-sizes="[20, 50, 100, 200]"
          layout="total, sizes, prev, pager, next"
          background
          @current-change="onPageChange"
          @size-change="search"
        />
      </div>
    </section>

    <el-drawer v-model="drawer.open" size="540px" :title="$t('broker.detail')">
      <div v-loading="drawer.loading" class="drawer-body">
        <el-alert
          v-if="drawer.closed"
          type="warning"
          :closable="false"
          :title="$t('broker.connectionClosed')"
        />
        <el-alert
          v-else-if="drawer.failed"
          type="error"
          :closable="false"
          :title="$t('broker.loadFailed')"
        />
        <template v-else-if="drawer.detail">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="$t('broker.type')">
              {{ drawer.detail.type || '-' }}
            </el-descriptions-item>
            <el-descriptions-item v-if="drawer.detail.clientId" :label="$t('broker.clientId')">
              {{ drawer.detail.clientId }}
            </el-descriptions-item>
            <el-descriptions-item v-if="drawer.detail.name" :label="$t('broker.name')">
              {{ drawer.detail.name }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.address')">
              {{ fmtAddr(drawer.detail.ip, drawer.detail.port) }}
            </el-descriptions-item>
            <el-descriptions-item label="CID">{{ drawer.detail.cid }}</el-descriptions-item>
            <el-descriptions-item :label="$t('broker.connectedAt')">
              {{ relativeTime(drawer.detail.start) }}
              <span class="cell-sub">({{ fmtFull(drawer.detail.start) }})</span>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.lastActivity')">
              {{ relativeTime(drawer.detail.lastActivity) }}
              <span class="cell-sub">({{ fmtFull(drawer.detail.lastActivity) }})</span>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.inMsgs')">
              {{ num(drawer.detail.inMsgs) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.outMsgs')">
              {{ num(drawer.detail.outMsgs) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.inBytes')">
              {{ num(drawer.detail.inBytes) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.outBytes')">
              {{ num(drawer.detail.outBytes) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.pending')">
              {{ num(drawer.detail.pending) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.idle')">
              {{ drawer.detail.idle || '-' }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.uptime')">
              {{ drawer.detail.uptime || '-' }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.rtt')">
              {{ drawer.detail.rtt || '-' }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('broker.subscriptions')">
              {{ drawer.detail.subscriptionCount }}
            </el-descriptions-item>
          </el-descriptions>
          <p class="empty-hint">{{ $t('broker.metricNote') }}</p>
          <p class="empty-hint">{{ $t('broker.cidNote') }}</p>

          <h4>{{ $t('broker.normalSubscriptions') }} ({{ drawer.detail.subscriptions.length }})</h4>
          <ul class="sub-list">
            <li v-for="s in drawer.detail.subscriptions" :key="s">{{ s }}</li>
          </ul>
          <p v-if="drawer.detail.subscriptions.length === 0" class="empty-hint">
            {{ $t('broker.noSubscriptions') }}
          </p>

          <h4>{{ $t('broker.internalSubscriptions') }} ({{ drawer.detail.internalSubscriptions.length }})</h4>
          <ul class="sub-list internal">
            <li v-for="s in drawer.detail.internalSubscriptions" :key="s">{{ s }}</li>
          </ul>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import dayjs from "dayjs";
import { useI18n } from "vue-i18n";
import {
  getBrokerConnection,
  getBrokerConnections,
  getBrokerOverview,
} from "@/apis";

const { t } = useI18n();

const overview = ref(null);
const loadingOverview = ref(false);
const connections = reactive({ items: [], total: 0 });
const nowTick = ref(Date.now());
const loadingConns = ref(false);
const refreshSeconds = ref(30);
const filters = reactive({
  clientId: "",
  sort: "uptime",
  page: 1,
  pageSize: 20,
});
const sortOptions = [
  { value: "start", label: "broker.sortStart" },
  { value: "subs", label: "broker.sortSubs" },
  { value: "pending", label: "broker.sortPending" },
  { value: "msgs_to", label: "broker.sortOutMsgs" },
  { value: "msgs_from", label: "broker.sortInMsgs" },
  { value: "bytes_to", label: "broker.sortOutBytes" },
  { value: "bytes_from", label: "broker.sortInBytes" },
  { value: "last", label: "broker.sortLast" },
  { value: "idle", label: "broker.sortIdle" },
  { value: "uptime", label: "broker.sortUptime" },
  { value: "rtt", label: "broker.sortRtt" },
];
const drawer = reactive({
  open: false,
  loading: false,
  detail: null,
  closed: false,
  failed: false,
});
let connectionsReqId = 0;
let detailReqId = 0;
let refreshTimer = null;
let tickTimer = null;

const PLOT_TOP = 4;
const PLOT_BOTTOM = 96;
const chartGrid = [PLOT_TOP, (PLOT_TOP + PLOT_BOTTOM) / 2, PLOT_BOTTOM];
const hoverState = reactive({ msgRate: null, byteRate: null });

const fmt = (v) => (v ? dayjs(v).format("MM-DD HH:mm:ss") : "-");
const fmtFull = (v) => (v ? dayjs(v).format("YYYY-MM-DD HH:mm:ss") : "-");
const relativeTime = (v) => {
  if (!v) return "-";
  const seconds = Math.max(0, Math.floor((nowTick.value - dayjs(v).valueOf()) / 1000));
  if (seconds < 60) return t("broker.justNow");
  if (seconds < 3600) return t("broker.minutesAgo", { n: Math.floor(seconds / 60) });
  if (seconds < 86400) return t("broker.hoursAgo", { n: Math.floor(seconds / 3600) });
  return t("broker.daysAgo", { n: Math.floor(seconds / 86400) });
};
const num = (v) => Number(v ?? 0).toLocaleString();
const unitOf = (max, base, step) => {
  if (max >= step * step) return { div: step * step, label: `M${base}` };
  if (max >= step) return { div: step, label: `k${base}` };
  return { div: 1, label: base };
};
const scaled = (v, div) => (v / div).toFixed(v / div >= 100 ? 0 : 1);
const axisNum = (v) => (v >= 10 ? Math.round(v).toLocaleString() : v.toFixed(1));
const fmtBytes = (v) => {
  const n = Number(v ?? 0);
  if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${n} B`;
};
const fmtAddr = (ip, port) => {
  if (!ip) return "-";
  return `${ip.includes(":") ? `[${ip}]` : ip}${port ? `:${port}` : ""}`;
};

const consumerText = computed(() => {
  const c = overview.value?.consumer;
  if (!c) return "-";
  if (c.state === "not_created") return t("broker.notCreated");
  if (c.state === "unknown") return t("broker.unknown");
  const limit = c.limitState === "unlimited" ? t("broker.unlimited") : num(c.limit);
  return `${num(c.usage)} / ${limit}`;
});

const makeChart = (key) => {
  const series = overview.value?.series || [];
  const values = series.map((p) => p[key]).filter((v) => v != null);
  const max = Math.max(1, ...values);
  const step = series.length > 1 ? 600 / (series.length - 1) : 600;
  const segments = [];
  const singlePoints = [];
  let current = [];
  const flush = () => {
    if (current.length === 1) singlePoints.push(current[0]);
    else if (current.length > 1) segments.push(current.map((p) => p.join(",")).join(" "));
    current = [];
  };
  const y = (v) => PLOT_BOTTOM - (v / max) * (PLOT_BOTTOM - PLOT_TOP);
  series.forEach((p, i) => {
    const v = p[key];
    if (v == null) {
      flush();
      return;
    }
    current.push([Number((i * step).toFixed(2)), Number(y(v).toFixed(2))]);
  });
  flush();
  return { segments, singlePoints, max };
};

const msgChart = computed(() => makeChart("msgRate"));
const byteChart = computed(() => makeChart("byteRate"));
const byteUnit = computed(() => unitOf(byteChart.value.max, "B/s", 1024));

const msgAxis = computed(() => ({
  max: axisNum(msgChart.value.max),
  mid: axisNum(msgChart.value.max / 2),
}));
const byteAxis = computed(() => ({
  max: scaled(byteChart.value.max, byteUnit.value.div),
  mid: scaled(byteChart.value.max / 2, byteUnit.value.div),
}));

const seriesStart = computed(() => overview.value?.series?.[0]?.time);
const seriesEnd = computed(() => {
  const series = overview.value?.series || [];
  return series[series.length - 1]?.time;
});
const rangeStart = computed(() => (seriesStart.value ? dayjs(seriesStart.value).format("HH:mm:ss") : "-"));
const rangeEnd = computed(() => (seriesEnd.value ? dayjs(seriesEnd.value).format("HH:mm:ss") : "-"));

const onChartMove = (event, key) => {
  const series = overview.value?.series || [];
  if (series.length === 0) return;
  const rect = event.currentTarget.getBoundingClientRect();
  const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
  const index = Math.round(ratio * (series.length - 1));
  const point = series[index];
  const max = key === "msgRate" ? msgChart.value.max : byteChart.value.max;
  const raw = point[key];
  const x = series.length > 1 ? (index * 600) / (series.length - 1) : 0;
  hoverState[key] = {
    x,
    xPct: (x / 600) * 100,
    time: dayjs(point.time).format("HH:mm:ss"),
    y: raw == null ? null : PLOT_BOTTOM - (raw / max) * (PLOT_BOTTOM - PLOT_TOP),
    value:
      raw == null
        ? null
        : key === "byteRate"
          ? `${scaled(raw, byteUnit.value.div)} ${byteUnit.value.label}`
          : `${Number(raw).toLocaleString(undefined, { maximumFractionDigits: 1 })} msg/s`,
  };
};

const tipStyle = (xPct) => ({
  left: `${xPct}%`,
  transform:
    xPct > 82 ? "translateX(-100%)" : xPct < 18 ? "translateX(0)" : "translateX(-50%)",
});

const loadOverview = async (silent = false) => {
  if (!silent) loadingOverview.value = true;
  try {
    overview.value = (await getBrokerOverview()).data;
  } catch (error) {
    // The request interceptor already reports the failure; keep the previous data.
  } finally {
    if (!silent) loadingOverview.value = false;
  }
};

const loadConnections = async (silent = false) => {
  const reqId = ++connectionsReqId;
  if (!silent) loadingConns.value = true;
  try {
    const res = await getBrokerConnections({
      clientId: filters.clientId || undefined,
      sort: filters.sort,
      page: filters.page,
      pageSize: filters.pageSize,
    });
    if (reqId !== connectionsReqId) return;
    connections.items = res.data.items || [];
    connections.total = res.data.total || 0;
  } catch (error) {
    // Keep the previous page; the interceptor reports the failure.
  } finally {
    if (reqId === connectionsReqId && !silent) loadingConns.value = false;
  }
};

const search = () => {
  filters.page = 1;
  loadConnections();
};

const onPageChange = () => loadConnections();

const openDetail = async (row) => {
  const reqId = ++detailReqId;
  drawer.open = true;
  drawer.loading = true;
  drawer.detail = null;
  drawer.closed = false;
  drawer.failed = false;
  try {
    const res = await getBrokerConnection(row.cid);
    if (reqId !== detailReqId) return;
    drawer.detail = res.data;
  } catch (error) {
    if (reqId !== detailReqId) return;
    drawer.closed = error?.response?.status === 404;
    drawer.failed = !drawer.closed;
  } finally {
    if (reqId === detailReqId) drawer.loading = false;
  }
};

const refreshAll = (silent = false) => {
  loadOverview(silent);
  loadConnections(silent);
};

const stopAutoRefresh = () => {
  if (refreshTimer) clearInterval(refreshTimer);
  refreshTimer = null;
};

const startAutoRefresh = () => {
  stopAutoRefresh();
  if (!refreshSeconds.value) return;
  refreshTimer = setInterval(() => {
    if (document.hidden || loadingOverview.value || loadingConns.value) return;
    refreshAll(true);
  }, refreshSeconds.value * 1000);
};

watch(refreshSeconds, startAutoRefresh);
onMounted(() => {
  refreshAll();
  startAutoRefresh();
  tickTimer = setInterval(() => {
    nowTick.value = Date.now();
  }, 30 * 1000);
});
onBeforeUnmount(() => {
  stopAutoRefresh();
  if (tickTimer) clearInterval(tickTimer);
  tickTimer = null;
});
</script>

<style scoped lang="scss">
.broker-view {
  padding: 20px 24px 32px;
  overflow: auto;
}

.workspace-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;

  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  h1 {
    margin: 0 0 4px;
    font-size: 22px;
  }

  p {
    margin: 0;
    color: var(--el-text-color-secondary);
  }
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  margin-bottom: 18px;
}

.overview-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 14px 16px;
}

.card-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-bottom: 6px;
}

.card-value {
  font-size: 18px;
  font-weight: 600;
  word-break: break-all;
}

.card-note {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-top: 4px;
}

.charts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 18px;
}

@media (max-width: 900px) {
  .charts {
    grid-template-columns: 1fr;
  }
}

.chart-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 12px 16px 8px;
}

.chart-head {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  margin-bottom: 6px;
}

.chart-unit {
  color: var(--el-text-color-secondary);
}

.chart-body {
  display: flex;
  gap: 6px;
}

.axis-y {
  width: 48px;
  height: 140px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  align-items: flex-end;
  padding: 2px 0 8px;
  font-size: 10px;
  color: var(--el-text-color-secondary);
}

.chart-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
}

.chart {
  display: block;
  width: 100%;
  height: 140px;
}

.axis-x {
  display: flex;
  justify-content: space-between;
  margin-top: 2px;
  font-size: 10px;
  color: var(--el-text-color-secondary);
}

.crosshair {
  stroke: var(--el-text-color-secondary);
  stroke-width: 0.6;
  stroke-dasharray: 3 3;
}

.chart-tip {
  position: absolute;
  top: 2px;
  padding: 4px 8px;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  box-shadow: var(--el-box-shadow-light);
  font-size: 12px;
  white-space: nowrap;
  pointer-events: none;
  z-index: 2;
}

.tip-time {
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.cell-stack {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}

.time-stack {
  display: grid;
  gap: 1px;
  justify-items: start;
}

.time-row {
  display: inline-grid;
  grid-template-columns: 28px auto;
  column-gap: 6px;
  align-items: center;
  width: fit-content;
}

.time-label {
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.time-value {
  font-size: 12px;
}

.time-value.muted {
  color: var(--el-text-color-secondary);
}

.cell-sub {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.cell-relative {
  cursor: help;
}

.grid-line {
  stroke: var(--el-border-color-lighter);
  stroke-width: 0.5;
}

.chart-line {
  fill: none;
  stroke: var(--el-color-primary);
  stroke-width: 1.4;
}

.byte-line {
  stroke: var(--el-color-success);
}

.chart-dot {
  fill: var(--el-color-primary);
}

.byte-dot {
  fill: var(--el-color-success);
}

.refresh-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.refresh-select {
  width: 100px;
}

.chart-note {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.table-shell {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 16px;
}

.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;

  h2 {
    margin: 0;
    font-size: 16px;
  }
}

.header-tip {
  border-bottom: 1px dashed var(--el-border-color);
  cursor: help;
}

.filters {
  display: flex;
  gap: 8px;
}

.filter-client {
  width: 220px;
}

.filter-sort {
  width: 240px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.drawer-body h4 {
  margin: 20px 0 8px;
}

.sub-list {
  margin: 0;
  padding-left: 18px;
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
  word-break: break-all;

  li {
    margin-bottom: 4px;
  }

  &.internal {
    color: var(--el-text-color-secondary);
  }
}

.empty-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

:deep(.clean-table .el-table__row) {
  cursor: pointer;
}

:deep(.clean-table .el-table__cell) {
  padding-top: 4px;
  padding-bottom: 4px;
}
</style>
